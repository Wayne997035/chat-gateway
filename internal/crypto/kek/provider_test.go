package kek_test

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"

	"chat-gateway/internal/crypto/kek"
)

// validKeysetBase64 generates a fresh AES256-GCM keyset for testing.
func validKeysetBase64(t *testing.T) string {
	t.Helper()

	handle, err := keyset.NewHandle(aead.AES256GCMKeyTemplate())
	if err != nil {
		t.Fatalf("generate keyset: %v", err)
	}

	var buf bytes.Buffer
	if err = insecurecleartextkeyset.Write(handle, keyset.NewJSONWriter(&buf)); err != nil {
		t.Fatalf("write keyset: %v", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestEnvVarProvider_AEAD(t *testing.T) {
	t.Parallel()

	validKeyset := validKeysetBase64(t)

	tests := []struct {
		name        string
		keysetB64   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "valid keyset returns AEAD",
			keysetB64: validKeyset,
			wantErr:   false,
		},
		{
			name:        "empty keyset returns error",
			keysetB64:   "",
			wantErr:     true,
			errContains: "KEK keyset is empty",
		},
		{
			name:        "invalid base64 returns error",
			keysetB64:   "not-valid-base64!!!",
			wantErr:     true,
			errContains: "decode KEK keyset",
		},
		{
			name:        "valid base64 but invalid keyset JSON returns error",
			keysetB64:   base64.StdEncoding.EncodeToString([]byte(`{"not":"a keyset"}`)),
			wantErr:     true,
			errContains: "load KEK keyset",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := kek.EnvVarProvider{KeysetBase64: tc.keysetB64}
			got, err := p.AEAD()

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContains != "" && !containsStr(err.Error(), tc.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.errContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatal("expected non-nil AEAD")
			}

			// Verify the AEAD actually works: encrypt then decrypt.
			plaintext := []byte("hello tink kek provider")
			ct, err := got.Encrypt(plaintext, nil)
			if err != nil {
				t.Fatalf("encrypt: %v", err)
			}
			pt, err := got.Decrypt(ct, nil)
			if err != nil {
				t.Fatalf("decrypt: %v", err)
			}
			if !bytes.Equal(pt, plaintext) {
				t.Errorf("decrypt mismatch: got %q, want %q", pt, plaintext)
			}
		})
	}
}

func containsStr(s, sub string) bool {
	return sub == "" || bytes.Contains([]byte(s), []byte(sub))
}
