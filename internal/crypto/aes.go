package crypto

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/tink-crypto/tink-go/v2/aead"
	"github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset"
	"github.com/tink-crypto/tink-go/v2/keyset"
)

type aeadPrimitive interface {
	Encrypt(pt, aad []byte) ([]byte, error)
	Decrypt(ct, aad []byte) ([]byte, error)
}

var aeadCache sync.Map

func getAEAD(keysetBase64 string) (aeadPrimitive, error) {
	keysetBase64 = strings.TrimSpace(keysetBase64)
	if keysetBase64 == "" {
		return nil, fmt.Errorf("empty keyset")
	}

	if cached, ok := aeadCache.Load(keysetBase64); ok {
		return cached.(aeadPrimitive), nil
	}

	keysetJSON, err := base64.StdEncoding.DecodeString(keysetBase64)
	if err != nil {
		return nil, fmt.Errorf("decode keyset: %w", err)
	}

	reader := keyset.NewJSONReader(bytes.NewReader(keysetJSON))
	// insecurecleartextkeyset: keyset values are encrypted by the caller (ENC wrapper)
	// before being stored in config — cleartext here means cleartext in memory only.
	handle, err := insecurecleartextkeyset.Read(reader)
	if err != nil {
		return nil, fmt.Errorf("read keyset: %w", err)
	}

	primitive, err := aead.New(handle)
	if err != nil {
		return nil, fmt.Errorf("build aead primitive: %w", err)
	}

	aeadCache.Store(keysetBase64, primitive)
	return primitive, nil
}

// ValidateKeySet 驗證 keyset 是否可被載入為 Tink AEAD.
func ValidateKeySet(keysetBase64 string) error {
	_, err := getAEAD(keysetBase64)
	return err
}

// Encrypt 使用 Tink AEAD 加密，回傳 base64 ciphertext.
func Encrypt(plaintext, keysetBase64 string) (string, error) {
	primitive, err := getAEAD(keysetBase64)
	if err != nil {
		return "", err
	}

	ciphertext, err := primitive.Encrypt([]byte(plaintext), nil)
	if err != nil {
		return "", fmt.Errorf("encrypt: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 使用 Tink AEAD 解密 base64 ciphertext.
func Decrypt(encrypted, keysetBase64 string) (string, error) {
	primitive, err := getAEAD(keysetBase64)
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("decode ciphertext: %w", err)
	}

	plaintext, err := primitive.Decrypt(ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return string(plaintext), nil
}

// GenerateKeySet 產生包含 n 把 AES256-GCM key 的 Tink JSON keyset（base64 編碼）.
func GenerateKeySet(n int) (string, error) {
	if n < 1 {
		return "", fmt.Errorf("key count must be >= 1")
	}

	handle, err := keyset.NewHandle(aead.AES256GCMKeyTemplate())
	if err != nil {
		return "", fmt.Errorf("create keyset: %w", err)
	}

	manager := keyset.NewManagerFromHandle(handle)
	for i := 1; i < n; i++ {
		keyID, err := manager.Add(aead.AES256GCMKeyTemplate())
		if err != nil {
			return "", fmt.Errorf("add key to keyset: %w", err)
		}
		if err := manager.SetPrimary(keyID); err != nil {
			return "", fmt.Errorf("set primary key: %w", err)
		}
	}
	handle, err = manager.Handle()
	if err != nil {
		return "", fmt.Errorf("finalize keyset: %w", err)
	}

	var buf bytes.Buffer
	writer := keyset.NewJSONWriter(&buf)
	if err := insecurecleartextkeyset.Write(handle, writer); err != nil {
		return "", fmt.Errorf("write keyset: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
