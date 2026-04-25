package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

const aes256GCMPrefix = "aes256gcm:"

// AESGCMEncryption AES-256-GCM AEAD 加密實現
// GCM 特點：
//   - 同時提供機密性與完整性驗證（AEAD）
//   - 12-byte nonce（NIST 推薦），16-byte auth tag
//   - 任何竄改都會在解密時被偵測
type AESGCMEncryption struct {
	key []byte // 256-bit (32 bytes) key
}

// NewAESGCMEncryption 創建 AES-256-GCM 加密實例
func NewAESGCMEncryption(key []byte) (*AESGCMEncryption, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (256 bits), got %d bytes", len(key))
	}

	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)

	return &AESGCMEncryption{key: keyCopy}, nil
}

// Encrypt 加密數據
// 格式: "aes256gcm:" + base64(nonce[12] | ciphertext | authTag[16])
func (e *AESGCMEncryption) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", fmt.Errorf("plaintext cannot be empty")
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize()) // 12 bytes
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	plaintextBytes := []byte(plaintext)
	defer func() {
		for i := range plaintextBytes {
			plaintextBytes[i] = 0
		}
	}()

	// Seal appends: nonce + ciphertext + authTag
	sealed := gcm.Seal(nonce, nonce, plaintextBytes, nil)

	return aes256GCMPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt 解密數據，GCM auth tag 驗證失敗會回傳錯誤
func (e *AESGCMEncryption) Decrypt(encryptedText string) (string, error) {
	if encryptedText == "" {
		return "", fmt.Errorf("encrypted text cannot be empty")
	}

	if len(encryptedText) < len(aes256GCMPrefix) || encryptedText[:len(aes256GCMPrefix)] != aes256GCMPrefix {
		return "", fmt.Errorf("invalid ciphertext format: missing '%s' prefix", aes256GCMPrefix)
	}

	data, err := base64.StdEncoding.DecodeString(encryptedText[len(aes256GCMPrefix):])
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	defer func() {
		for i := range data {
			data[i] = 0
		}
	}()

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+gcm.Overhead() {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed (auth tag mismatch or corrupted data): %w", err)
	}
	defer func() {
		for i := range plaintext {
			plaintext[i] = 0
		}
	}()

	return string(plaintext), nil
}

// IsEncrypted 檢查文本是否為 GCM 加密格式
func (e *AESGCMEncryption) IsEncrypted(text string) bool {
	return len(text) >= len(aes256GCMPrefix) && text[:len(aes256GCMPrefix)] == aes256GCMPrefix
}
