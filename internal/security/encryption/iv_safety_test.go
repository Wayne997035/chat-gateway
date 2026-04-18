package encryption

import (
	"crypto/rand"
	"io"
	"testing"
)

const testMessage = "test message"

// TestNonceUniqueness 測試 GCM nonce 的唯一性（等同原 CTR IV 唯一性測試）
func TestNonceUniqueness(t *testing.T) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encryptor, err := NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := testMessage
	ciphertexts := make([]string, 100)

	for i := 0; i < 100; i++ {
		ciphertext, err := encryptor.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encryption failed: %v", err)
		}
		ciphertexts[i] = ciphertext
	}

	for i := 0; i < len(ciphertexts); i++ {
		for j := i + 1; j < len(ciphertexts); j++ {
			if ciphertexts[i] == ciphertexts[j] {
				t.Errorf("Found duplicate ciphertext at index %d and %d", i, j)
				t.Error("This means nonce is not unique - SECURITY ISSUE!")
			}
		}
	}

	t.Log("✓ All 100 ciphertexts are unique - nonce generation is secure")
}

// TestNonceExtraction 測試加解密循環的完整性
func TestNonceExtraction(t *testing.T) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	encryptor, err := NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	plaintext := testMessage

	ciphertext, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encryption failed: %v", err)
	}

	decrypted, err := encryptor.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decryption failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("Decryption mismatch: got %q, want %q", decrypted, plaintext)
		t.Error("This means nonce extraction is broken - SECURITY ISSUE!")
	}

	t.Log("✓ Nonce extraction works correctly - encryption/decryption cycle successful")
}

// TestSignalProtocolIVUniqueness 測試 Signal Protocol 的 IV 唯一性
func TestSignalProtocolIVUniqueness(t *testing.T) {
	sp, err := NewSignalProtocol()
	if err != nil {
		t.Fatalf("Failed to create signal protocol: %v", err)
	}

	sessionID := "test_session"
	rootKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, rootKey); err != nil {
		t.Fatalf("Failed to generate root key: %v", err)
	}

	if err := sp.DoubleRatchet(sessionID, rootKey); err != nil {
		t.Fatalf("Failed to initialize session: %v", err)
	}

	plaintext := []byte(testMessage)
	ciphertexts := make([][]byte, 10)

	for i := 0; i < 10; i++ {
		ciphertext, err := sp.EncryptMessage(sessionID, plaintext)
		if err != nil {
			t.Fatalf("Encryption %d failed: %v", i, err)
		}
		ciphertexts[i] = ciphertext
	}

	for i := 0; i < len(ciphertexts); i++ {
		for j := i + 1; j < len(ciphertexts); j++ {
			minLen := len(ciphertexts[i])
			if len(ciphertexts[j]) < minLen {
				minLen = len(ciphertexts[j])
			}
			if minLen > 50 {
				minLen = 50
			}

			same := true
			for k := 0; k < minLen; k++ {
				if ciphertexts[i][k] != ciphertexts[j][k] {
					same = false
					break
				}
			}

			if same {
				t.Errorf("Found similar ciphertext at index %d and %d", i, j)
				t.Error("This means Signal Protocol IV/nonce is not unique - SECURITY ISSUE!")
			}
		}
	}

	t.Log("✓ All 10 Signal Protocol ciphertexts are unique - IV/nonce generation is secure")
}
