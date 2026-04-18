package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
	"strings"
	"testing"

	"chat-gateway/internal/security/encryption"
)

// ---- helpers ----

func newTestKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}
	return key
}

// encryptCTRForTest 用於測試中產生 CTR 格式密文（模擬舊格式資料）.
func encryptCTRForTest(plaintext string, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	data := make([]byte, aes.BlockSize+len(plaintext))
	iv := data[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// #nosec G407 -- IV is dynamically generated from crypto/rand, not hardcoded
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(data[aes.BlockSize:], []byte(plaintext))

	return ctrPrefix + base64.StdEncoding.EncodeToString(data), nil
}

// ---- decryptCTR tests ----

func TestDecryptCTR_HappyPath(t *testing.T) {
	key := newTestKey(t)
	original := "Hello, World! 你好世界"

	ciphertext, err := encryptCTRForTest(original, key)
	if err != nil {
		t.Fatalf("encryptCTRForTest: %v", err)
	}

	got, err := decryptCTR(ciphertext, key)
	if err != nil {
		t.Fatalf("decryptCTR: %v", err)
	}
	if got != original {
		t.Errorf("roundtrip mismatch: got %q, want %q", got, original)
	}
}

func TestDecryptCTR_MissingPrefix(t *testing.T) {
	key := newTestKey(t)
	_, err := decryptCTR("somebase64data==", key)
	if err == nil {
		t.Error("expected error for missing CTR prefix, got nil")
	}
}

func TestDecryptCTR_InvalidBase64(t *testing.T) {
	key := newTestKey(t)
	_, err := decryptCTR(ctrPrefix+"!!!not-base64!!!", key)
	if err == nil {
		t.Error("expected error for invalid base64, got nil")
	}
}

func TestDecryptCTR_TooShort(t *testing.T) {
	key := newTestKey(t)
	// Valid base64 but less than BlockSize bytes.
	tiny := base64.StdEncoding.EncodeToString([]byte("tooshort"))
	_, err := decryptCTR(ctrPrefix+tiny, key)
	if err == nil {
		t.Error("expected error for ciphertext too short, got nil")
	}
}

func TestDecryptCTR_WrongKey(t *testing.T) {
	key1 := newTestKey(t)
	key2 := newTestKey(t)

	ciphertext, err := encryptCTRForTest("secret", key1)
	if err != nil {
		t.Fatalf("encryptCTRForTest: %v", err)
	}

	// CTR decryption with wrong key won't error (stream cipher), but result will be garbage.
	plaintext, err := decryptCTR(ciphertext, key2)
	if err != nil {
		t.Fatalf("decryptCTR with wrong key unexpectedly errored: %v", err)
	}
	if plaintext == "secret" {
		t.Error("expected wrong key to produce different plaintext")
	}
}

// ---- plaintext: prefix migration logic tests ----

func TestMigration_PlaintextPrefix_ReencryptsToGCM(t *testing.T) {
	key := newTestKey(t)

	original := "hello from plaintext prefix"
	content := plaintextPrefix + original

	// Simulate the migration logic: strip prefix, re-encrypt with GCM.
	plaintext := content[len(plaintextPrefix):]

	gcmEnc, err := encryption.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption: %v", err)
	}

	newContent, err := gcmEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if !strings.HasPrefix(newContent, gcmPrefix) {
		t.Errorf("expected GCM prefix %q, got %q", gcmPrefix, newContent[:min(len(newContent), 20)])
	}

	// Verify round-trip.
	decrypted, err := gcmEnc.Decrypt(newContent)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != original {
		t.Errorf("round-trip mismatch: got %q, want %q", decrypted, original)
	}
}

// ---- already-GCM format is skipped ----

func TestMigration_AlreadyGCM_IsSkipped(t *testing.T) {
	key := newTestKey(t)

	gcmEnc, err := encryption.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption: %v", err)
	}

	gcmContent, err := gcmEnc.Encrypt("already encrypted message")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Reproduce the skip logic from migrateBatch.
	isGCM := len(gcmContent) >= len(gcmPrefix) && gcmContent[:len(gcmPrefix)] == gcmPrefix
	if !isGCM {
		t.Errorf("expected content to be detected as GCM format, got %q", gcmContent[:min(len(gcmContent), 20)])
	}
}

// ---- CTR ciphertext → GCM migration ----

func TestMigration_CTRToGCM_RoundTrip(t *testing.T) {
	key := newTestKey(t)
	original := "message encrypted with CTR"

	// 1. Encrypt with CTR (old format).
	ctrCiphertext, err := encryptCTRForTest(original, key)
	if err != nil {
		t.Fatalf("encryptCTRForTest: %v", err)
	}

	// 2. Decrypt CTR.
	plaintext, err := decryptCTR(ctrCiphertext, key)
	if err != nil {
		t.Fatalf("decryptCTR: %v", err)
	}

	// 3. Re-encrypt with GCM.
	gcmEnc, err := encryption.NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption: %v", err)
	}

	gcmCiphertext, err := gcmEnc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// 4. Verify final GCM ciphertext decrypts to original.
	decrypted, err := gcmEnc.Decrypt(gcmCiphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != original {
		t.Errorf("CTR→GCM round-trip mismatch: got %q, want %q", decrypted, original)
	}
}

// ---- loadMasterKey tests ----

func TestLoadMasterKey_Missing(t *testing.T) {
	t.Setenv("MASTER_KEY", "")
	_, err := loadMasterKey()
	if err == nil {
		t.Error("expected error when MASTER_KEY is empty, got nil")
	}
}

func TestLoadMasterKey_InvalidBase64(t *testing.T) {
	t.Setenv("MASTER_KEY", "!!!not-base64!!!")
	_, err := loadMasterKey()
	if err == nil {
		t.Error("expected error for invalid base64 MASTER_KEY, got nil")
	}
}

func TestLoadMasterKey_WrongLength(t *testing.T) {
	// 16 bytes base64 encoded (not 32).
	shortKey := base64.StdEncoding.EncodeToString(make([]byte, 16))
	t.Setenv("MASTER_KEY", shortKey)
	_, err := loadMasterKey()
	if err == nil {
		t.Error("expected error for MASTER_KEY with wrong length, got nil")
	}
}

func TestLoadMasterKey_Valid(t *testing.T) {
	validKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, validKey); err != nil {
		t.Fatalf("generate key: %v", err)
	}
	t.Setenv("MASTER_KEY", base64.StdEncoding.EncodeToString(validKey))

	got, err := loadMasterKey()
	if err != nil {
		t.Fatalf("loadMasterKey: %v", err)
	}
	if len(got) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(got))
	}
}
