package config

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"chat-gateway/internal/crypto"
)

// generateTestKeyset 產生一組 Tink AES-256-GCM keyset 供測試使用.
func generateTestKeyset(t *testing.T) string {
	t.Helper()
	ks, err := crypto.GenerateKeySet(1)
	if err != nil {
		t.Fatalf("GenerateKeySet failed: %v", err)
	}
	return ks
}

// encryptForTest 將明文加密為 ENC(ciphertext) 格式供測試使用.
func encryptForTest(t *testing.T, plaintext, keyset string) string {
	t.Helper()
	ct, err := crypto.Encrypt(plaintext, keyset)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	return fmt.Sprintf("ENC(%s)", ct)
}

func TestWalkAndDecrypt_ValidENC(t *testing.T) {
	keyset := generateTestKeyset(t)
	enc := encryptForTest(t, "secret-value", keyset)

	type container struct {
		Field string
	}
	c := container{Field: enc}

	v := reflect.ValueOf(&c).Elem()
	if err := walkAndDecrypt(v, keyset); err != nil {
		t.Fatalf("walkAndDecrypt returned error: %v", err)
	}
	if c.Field != "secret-value" {
		t.Errorf("expected decrypted value %q, got %q", "secret-value", c.Field)
	}
}

func TestWalkAndDecrypt_PlainString_Unchanged(t *testing.T) {
	keyset := generateTestKeyset(t)

	type container struct {
		Field string
	}
	c := container{Field: "plain-value"}

	v := reflect.ValueOf(&c).Elem()
	if err := walkAndDecrypt(v, keyset); err != nil {
		t.Fatalf("walkAndDecrypt returned error: %v", err)
	}
	if c.Field != "plain-value" {
		t.Errorf("expected unchanged value %q, got %q", "plain-value", c.Field)
	}
}

func TestWalkAndDecrypt_InvalidBase64_ReturnsError(t *testing.T) {
	keyset := generateTestKeyset(t)

	type container struct {
		Field string
	}
	// The ENC content is not valid base64 ciphertext for this keyset
	c := container{Field: "ENC(!!!not-valid-base64!!!)"}

	v := reflect.ValueOf(&c).Elem()
	err := walkAndDecrypt(v, keyset)
	if err == nil {
		t.Fatal("expected error for invalid base64 ENC content, got nil")
	}
}

func TestWalkAndDecrypt_TamperedCiphertext_ReturnsError(t *testing.T) {
	keyset := generateTestKeyset(t)
	// Generate valid base64 but not a valid ciphertext for this keyset
	// Use a different keyset to encrypt so it cannot be decrypted by the first keyset
	otherKeyset := generateTestKeyset(t)
	enc := encryptForTest(t, "secret", otherKeyset)
	// enc is ENC(<valid base64 ciphertext for otherKeyset>)
	// strip ENC() wrapper to get the raw ciphertext
	inner := strings.TrimPrefix(enc, "ENC(")
	inner = strings.TrimSuffix(inner, ")")

	type container struct {
		Field string
	}
	c := container{Field: fmt.Sprintf("ENC(%s)", inner)}

	v := reflect.ValueOf(&c).Elem()
	err := walkAndDecrypt(v, keyset)
	if err == nil {
		t.Fatal("expected error for tampered ciphertext, got nil")
	}
}

func TestWalkAndDecrypt_EmptyENCContent_ReturnsError(t *testing.T) {
	keyset := generateTestKeyset(t)

	type container struct {
		Field string
	}
	c := container{Field: "ENC()"}

	v := reflect.ValueOf(&c).Elem()
	err := walkAndDecrypt(v, keyset)
	if err == nil {
		t.Fatal("expected error for empty ENC() content, got nil")
	}
}

func TestWalkAndDecrypt_NestedStruct(t *testing.T) {
	keyset := generateTestKeyset(t)
	enc := encryptForTest(t, "nested-secret", keyset)

	type inner struct {
		Token string
	}
	type outer struct {
		Sub inner
	}

	c := outer{Sub: inner{Token: enc}}
	v := reflect.ValueOf(&c).Elem()
	if err := walkAndDecrypt(v, keyset); err != nil {
		t.Fatalf("walkAndDecrypt returned error: %v", err)
	}
	if c.Sub.Token != "nested-secret" {
		t.Errorf("expected decrypted value %q, got %q", "nested-secret", c.Sub.Token)
	}
}

func TestDecryptConfig_EmptyKeySet_NoOp(t *testing.T) {
	cfg := &Config{
		Security: SecurityConfig{
			KeySet:     "",
			AdminToken: "ENC(some-value)",
		},
	}
	// Should return nil without touching anything
	if err := decryptConfig(cfg); err != nil {
		t.Fatalf("decryptConfig with empty KeySet returned error: %v", err)
	}
	// Field should remain unchanged
	if cfg.Security.AdminToken != "ENC(some-value)" {
		t.Errorf("expected field unchanged, got %q", cfg.Security.AdminToken)
	}
}

func TestDecryptConfig_SkipsSecurityField(t *testing.T) {
	keyset := generateTestKeyset(t)

	// Place an ENC() value in a non-Security field and a plain value in Security.KeySet
	// walkAndDecrypt should skip the Security struct entirely
	enc := encryptForTest(t, "db-password", keyset)

	cfg := &Config{}
	cfg.Database.Mongo.Password = enc
	cfg.Security.KeySet = keyset
	// Security.AdminToken is plain; it should remain plain since Security is skipped
	cfg.Security.AdminToken = "plain-admin-token"

	if err := decryptConfig(cfg); err != nil {
		t.Fatalf("decryptConfig returned error: %v", err)
	}
	if cfg.Database.Mongo.Password != "db-password" {
		t.Errorf("expected decrypted mongo password, got %q", cfg.Database.Mongo.Password)
	}
	// Security.AdminToken must remain unchanged (Security subtree is skipped)
	if cfg.Security.AdminToken != "plain-admin-token" {
		t.Errorf("expected Security.AdminToken unchanged, got %q", cfg.Security.AdminToken)
	}
}

func TestDecryptConfig_MultipleEncFields(t *testing.T) {
	keyset := generateTestKeyset(t)

	encURL := encryptForTest(t, "mongodb://user:pass@host/db", keyset)
	encDB := encryptForTest(t, "mydb", keyset)

	cfg := &Config{}
	cfg.Database.Mongo.URL = encURL
	cfg.Database.Mongo.Database = encDB
	cfg.Security.KeySet = keyset

	if err := decryptConfig(cfg); err != nil {
		t.Fatalf("decryptConfig returned error: %v", err)
	}
	if cfg.Database.Mongo.URL != "mongodb://user:pass@host/db" {
		t.Errorf("unexpected URL: %q", cfg.Database.Mongo.URL)
	}
	if cfg.Database.Mongo.Database != "mydb" {
		t.Errorf("unexpected database: %q", cfg.Database.Mongo.Database)
	}
}
