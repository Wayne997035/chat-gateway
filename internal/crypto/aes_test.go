package crypto

import (
	"strings"
	"testing"
)

func TestGenerateKeySet_ValidN(t *testing.T) {
	cases := []struct {
		name string
		n    int
	}{
		{"single key", 1},
		{"two keys", 2},
		{"three keys", 3},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ks, err := GenerateKeySet(tc.n)
			if err != nil {
				t.Fatalf("GenerateKeySet(%d): %v", tc.n, err)
			}
			if ks == "" {
				t.Fatal("expected non-empty keyset")
			}
			// Must be valid base64 — ValidateKeySet will decode it
			if err := ValidateKeySet(ks); err != nil {
				t.Fatalf("ValidateKeySet after generate: %v", err)
			}
		})
	}
}

func TestGenerateKeySet_InvalidN(t *testing.T) {
	cases := []struct {
		name string
		n    int
	}{
		{"zero", 0},
		{"negative", -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := GenerateKeySet(tc.n)
			if err == nil {
				t.Errorf("GenerateKeySet(%d): expected error, got nil", tc.n)
			}
		})
	}
}

func TestValidateKeySet_Invalid(t *testing.T) {
	cases := []struct {
		name   string
		keyset string
	}{
		{"empty string", ""},
		{"whitespace only", "   "},
		{"not base64", "!!!notbase64!!!"},
		{"valid base64 but not a keyset", "aGVsbG8gd29ybGQ="},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateKeySet(tc.keyset)
			if err == nil {
				t.Errorf("ValidateKeySet(%q): expected error, got nil", tc.keyset)
			}
		})
	}
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	ks, err := GenerateKeySet(1)
	if err != nil {
		t.Fatalf("GenerateKeySet: %v", err)
	}

	cases := []struct {
		name      string
		plaintext string
	}{
		{"ascii", "Hello, World!"},
		{"chinese", "你好世界"},
		{"special_chars", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
		{"long_text", strings.Repeat("x", 10_000)},
		{"unicode_emoji", "Chat encryption test 🔐"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ct, err := Encrypt(tc.plaintext, ks)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			if ct == "" {
				t.Fatal("expected non-empty ciphertext")
			}

			pt, err := Decrypt(ct, ks)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if pt != tc.plaintext {
				t.Errorf("roundtrip mismatch: got %q, want %q", pt, tc.plaintext)
			}
		})
	}
}

func TestEncrypt_NonceUniqueness(t *testing.T) {
	ks, err := GenerateKeySet(1)
	if err != nil {
		t.Fatalf("GenerateKeySet: %v", err)
	}

	plaintext := "same plaintext every iteration"
	seen := make(map[string]struct{}, 50)

	for i := 0; i < 50; i++ {
		ct, err := Encrypt(plaintext, ks)
		if err != nil {
			t.Fatalf("Encrypt[%d]: %v", i, err)
		}
		if _, dup := seen[ct]; dup {
			t.Fatal("duplicate ciphertext — nonce not unique")
		}
		seen[ct] = struct{}{}
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	ks1, _ := GenerateKeySet(1)
	ks2, _ := GenerateKeySet(1)

	ct, err := Encrypt("secret", ks1)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = Decrypt(ct, ks2)
	if err == nil {
		t.Error("expected decryption to fail with a different key, got nil error")
	}
}

func TestDecrypt_TamperedCiphertext(t *testing.T) {
	ks, _ := GenerateKeySet(1)
	ct, err := Encrypt("tamper me", ks)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	// Corrupt one byte in the base64 payload
	payload := []byte(ct)
	if len(payload) > 5 {
		payload[5] ^= 0xFF
	}

	_, err = Decrypt(string(payload), ks)
	if err == nil {
		t.Error("expected auth tag verification to fail on tampered ciphertext")
	}
}

func TestDecrypt_InvalidBase64Ciphertext(t *testing.T) {
	ks, _ := GenerateKeySet(1)

	_, err := Decrypt("!!!not-base64!!!", ks)
	if err == nil {
		t.Error("expected error for non-base64 ciphertext, got nil")
	}
}

func TestEncrypt_InvalidKeySet(t *testing.T) {
	_, err := Encrypt("hello", "not-a-keyset")
	if err == nil {
		t.Error("expected error for invalid keyset, got nil")
	}
}

func TestDecrypt_InvalidKeySet(t *testing.T) {
	_, err := Decrypt("aGVsbG8=", "not-a-keyset")
	if err == nil {
		t.Error("expected error for invalid keyset, got nil")
	}
}
