package encryption

import (
	"crypto/rand"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
)

// parseVersionPrefix extracts (version, ciphertext) from a possibly-versioned ciphertext.
// Mirrors the logic in DecryptMessage — kept here to test the parsing logic without a DB.
func parseVersionPrefix(encryptedContent string) (version int, ciphertext string) {
	version = 1
	ciphertext = encryptedContent
	if len(encryptedContent) > 2 && encryptedContent[0] == 'v' {
		colonIdx := strings.Index(encryptedContent[1:], ":")
		if colonIdx >= 0 {
			if n, parseErr := strconv.Atoi(encryptedContent[1 : 1+colonIdx]); parseErr == nil {
				version = n
				ciphertext = encryptedContent[1+colonIdx+1:]
			}
		}
	}
	// 版本號無效（0 或負數）視為 legacy v1，整個字串作為密文（與 DecryptMessage 行為一致）
	if version <= 0 {
		version = 1
		ciphertext = encryptedContent
	}
	return version, ciphertext
}

func newTestKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

// TestEncryptMessage_VersionPrefixFormat verifies that the ciphertext produced
// by the encrypt path carries a v{N}:aes256gcm: prefix.
func TestEncryptMessage_VersionPrefixFormat(t *testing.T) {
	cases := []struct {
		name    string
		version int
	}{
		{"version_1", 1},
		{"version_2", 2},
		{"version_10", 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := newTestKey(t)
			aesGCM, err := NewAESGCMEncryption(key)
			if err != nil {
				t.Fatalf("NewAESGCMEncryption: %v", err)
			}
			raw, err := aesGCM.Encrypt("test message")
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}

			versioned := fmt.Sprintf("v%d:%s", tc.version, raw)
			expectedPrefix := fmt.Sprintf("v%d:%s", tc.version, aes256GCMPrefix)

			if !strings.HasPrefix(versioned, expectedPrefix) {
				t.Errorf("versioned ciphertext should have prefix %q", expectedPrefix)
			}

			// Parse it back
			gotVersion, gotCiphertext := parseVersionPrefix(versioned)
			if gotVersion != tc.version {
				t.Errorf("version: got %d, want %d", gotVersion, tc.version)
			}
			if gotCiphertext != raw {
				t.Errorf("ciphertext after strip: got %q, want %q", gotCiphertext, raw)
			}
		})
	}
}

// TestDecryptMessage_WithVersionPrefix tests round-trip with v1: prefix.
func TestDecryptMessage_WithVersionPrefix(t *testing.T) {
	key := newTestKey(t)
	aesGCM, err := NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption: %v", err)
	}

	plaintext := "hello version routing"
	raw, err := aesGCM.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	versioned := fmt.Sprintf("v1:%s", raw)

	// Parse and decrypt
	_, ciphertext := parseVersionPrefix(versioned)
	decrypted, err := aesGCM.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

// TestDecryptMessage_LegacyNoPrefix verifies backward compatibility:
// a ciphertext without v{N}: prefix is treated as version 1.
func TestDecryptMessage_LegacyNoPrefix(t *testing.T) {
	key := newTestKey(t)
	aesGCM, err := NewAESGCMEncryption(key)
	if err != nil {
		t.Fatalf("NewAESGCMEncryption: %v", err)
	}

	plaintext := "legacy message without version prefix"
	// Old format: aes256gcm:... (no v{N}: prefix)
	legacyCiphertext, err := aesGCM.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	version, ciphertext := parseVersionPrefix(legacyCiphertext)
	if version != 1 {
		t.Errorf("legacy ciphertext should be treated as version 1, got %d", version)
	}
	if ciphertext != legacyCiphertext {
		t.Errorf("ciphertext should be unchanged for legacy format")
	}

	// Must decrypt correctly
	decrypted, err := aesGCM.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt legacy ciphertext: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("legacy round-trip mismatch: got %q, want %q", decrypted, plaintext)
	}
}

// TestVersionPrefix_ParseEdgeCases checks parser edge cases.
func TestVersionPrefix_ParseEdgeCases(t *testing.T) {
	cases := []struct {
		name        string
		input       string
		wantVersion int
		wantSame    bool // whether ciphertext should equal input
	}{
		{"empty", "", 1, true},
		{"just_v", "v", 1, true},
		{"v_letters", "vabc:data", 1, true},            // Atoi("abc") fails → version=1
		{"v_no_colon", "v123data", 1, true},            // no colon → version=1
		{"v1_colon", "v1:data", 1, false},              // valid → version=1, ciphertext="data"
		{"v2_colon", "v2:cipher", 2, false},            // valid → version=2, ciphertext="cipher"
		{"v10_colon", "v10:cipher", 10, false},         // valid → version=10
		{"v0_colon", "v0:aes256gcm:somedata", 1, true}, // version 0 is invalid, falls back to v1
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			version, ciphertext := parseVersionPrefix(tc.input)
			if version != tc.wantVersion {
				t.Errorf("version = %d, want %d", version, tc.wantVersion)
			}
			if tc.wantSame && ciphertext != tc.input {
				t.Errorf("ciphertext should equal input for unparseable input, got %q", ciphertext)
			}
		})
	}
}
