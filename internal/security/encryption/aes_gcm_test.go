package encryption

import (
	"crypto/rand"
	"io"
	"strings"
	"sync"
	"testing"
)

func newTestGCMKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

func TestAESGCMEncryption_RoundTrip(t *testing.T) {
	cases := []struct {
		name      string
		plaintext string
	}{
		{"ascii", "Hello, World!"},
		{"chinese", "你好世界，這是加密測試"},
		{"special_chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"empty_spaces", "   spaces   \t\n"},
		{"long_text", strings.Repeat("a", 10_000)},
		{"unicode_mixed", "Chat 訊息 🦉 encryption test"},
	}

	key := newTestGCMKey(t)
	enc, err := NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tc.plaintext)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if !strings.HasPrefix(ciphertext, aes256GCMPrefix) {
				t.Errorf("ciphertext missing prefix: %q", ciphertext[:20])
			}

			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if decrypted != tc.plaintext {
				t.Errorf("roundtrip mismatch: got %q, want %q", decrypted, tc.plaintext)
			}
		})
	}
}

func TestAESGCMEncryption_EmptyInput(t *testing.T) {
	key := newTestGCMKey(t)
	enc, _ := NewAESGCMEncryption(key)

	_, err := enc.Encrypt("")
	if err == nil {
		t.Error("expected error for empty plaintext, got nil")
	}
}

func TestAESGCMEncryption_WrongKey(t *testing.T) {
	key1 := newTestGCMKey(t)
	key2 := newTestGCMKey(t)

	enc1, _ := NewAESGCMEncryption(key1)
	enc2, _ := NewAESGCMEncryption(key2)

	ciphertext, err := enc1.Encrypt("secret message")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = enc2.Decrypt(ciphertext)
	if err == nil {
		t.Error("expected decryption to fail with wrong key, got nil error")
	}
}

func TestAESGCMEncryption_TamperedCiphertext(t *testing.T) {
	key := newTestGCMKey(t)
	enc, _ := NewAESGCMEncryption(key)

	ciphertext, err := enc.Encrypt("tamper me")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// flip a byte in the base64 payload
	prefix := aes256GCMPrefix
	payload := []byte(ciphertext[len(prefix):])
	if len(payload) > 5 {
		payload[5] ^= 0xFF
	}
	tampered := prefix + string(payload)

	_, err = enc.Decrypt(tampered)
	if err == nil {
		t.Error("expected auth tag verification to fail on tampered ciphertext")
	}
}

func TestAESGCMEncryption_InvalidKey(t *testing.T) {
	cases := []int{0, 16, 31, 33, 64}
	for _, size := range cases {
		key := make([]byte, size)
		_, err := NewAESGCMEncryption(key)
		if err == nil {
			t.Errorf("expected error for key size %d, got nil", size)
		}
	}
}

func TestAESGCMEncryption_IsEncrypted(t *testing.T) {
	key := newTestGCMKey(t)
	enc, _ := NewAESGCMEncryption(key)

	ciphertext, _ := enc.Encrypt("test")

	if !enc.IsEncrypted(ciphertext) {
		t.Error("IsEncrypted should return true for GCM ciphertext")
	}
	if enc.IsEncrypted("plaintext:hello") {
		t.Error("IsEncrypted should return false for plaintext prefix")
	}
	if enc.IsEncrypted("") {
		t.Error("IsEncrypted should return false for empty string")
	}
	if enc.IsEncrypted("aes256ctr:something") {
		t.Error("IsEncrypted should return false for CTR prefix")
	}
}

func TestAESGCMEncryption_NonceUniqueness(t *testing.T) {
	key := newTestGCMKey(t)
	enc, _ := NewAESGCMEncryption(key)

	plaintext := "same message every time"
	ciphertexts := make([]string, 100)

	for i := range ciphertexts {
		c, err := enc.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("Encrypt[%d]: %v", i, err)
		}
		ciphertexts[i] = c
	}

	for i := 0; i < len(ciphertexts); i++ {
		for j := i + 1; j < len(ciphertexts); j++ {
			if ciphertexts[i] == ciphertexts[j] {
				t.Errorf("duplicate ciphertext at index %d and %d — nonce not unique", i, j)
			}
		}
	}
}

func TestAESGCMEncryption_Concurrent(t *testing.T) {
	key := newTestGCMKey(t)
	enc, _ := NewAESGCMEncryption(key)

	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ct, err := enc.Encrypt("concurrent test message")
			if err != nil {
				errs <- err
				return
			}
			if _, err := enc.Decrypt(ct); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}
