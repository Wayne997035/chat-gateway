package encryption

import (
	"fmt"
	"log"

	"chat-gateway/internal/security/keymanager"
)

const plaintextPrefix = "plaintext:"

// MessageEncryption 消息加密服務，使用 AES-256-GCM AEAD 加密模式
type MessageEncryption struct {
	enabled    bool
	keyManager *keymanager.KeyManagerWithPersistence
}

// NewMessageEncryption 創建消息加密服務
func NewMessageEncryption(enabled bool, km *keymanager.KeyManagerWithPersistence) *MessageEncryption {
	if km == nil {
		log.Println("[WARNING] KeyManager is nil. Encryption will be disabled.")
		enabled = false
	}

	return &MessageEncryption{
		enabled:    enabled,
		keyManager: km,
	}
}

// EncryptMessage 使用 AES-256-GCM 加密消息
func (m *MessageEncryption) EncryptMessage(content, roomID string) (string, error) {
	if !m.enabled {
		log.Println("[WARNING] Message encryption is DISABLED. Messages are stored in PLAIN TEXT!")
		return plaintextPrefix + content, nil
	}

	if m.keyManager == nil {
		return "", fmt.Errorf("key manager not initialized")
	}

	key, err := m.keyManager.GetOrCreateRoomKey(roomID)
	if err != nil {
		return "", fmt.Errorf("failed to get room key: %w", err)
	}

	aesGCM, err := NewAESGCMEncryption(key)
	if err != nil {
		return "", fmt.Errorf("failed to create encryptor: %w", err)
	}

	encrypted, err := aesGCM.Encrypt(content)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	return encrypted, nil
}

// DecryptMessage 解密消息，GCM auth tag 驗證竄改
func (m *MessageEncryption) DecryptMessage(encryptedContent, roomID string) (string, error) {
	if !m.enabled {
		if len(encryptedContent) > len(plaintextPrefix) && encryptedContent[:len(plaintextPrefix)] == plaintextPrefix {
			return encryptedContent[len(plaintextPrefix):], nil
		}
		return encryptedContent, nil
	}

	if m.keyManager == nil {
		return "", fmt.Errorf("key manager not initialized")
	}

	// plaintext: 前綴（加密 disabled 時存的明文）
	if len(encryptedContent) >= len(plaintextPrefix) && encryptedContent[:len(plaintextPrefix)] == plaintextPrefix {
		return encryptedContent[len(plaintextPrefix):], nil
	}

	key, err := m.keyManager.GetOrCreateRoomKey(roomID)
	if err != nil {
		return "", fmt.Errorf("failed to get room key: %w", err)
	}

	aesGCM, err := NewAESGCMEncryption(key)
	if err != nil {
		return "", fmt.Errorf("failed to create decryptor: %w", err)
	}

	decrypted, err := aesGCM.Decrypt(encryptedContent)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return decrypted, nil
}

// IsEncrypted 檢查消息是否為 AES-256-GCM 加密格式
func (m *MessageEncryption) IsEncrypted(content string) bool {
	if len(content) < len(aes256GCMPrefix) {
		return false
	}
	return content[:len(aes256GCMPrefix)] == aes256GCMPrefix
}

// GetKeyInfo 獲取密鑰信息（用於調試）
func (m *MessageEncryption) GetKeyInfo(roomID string) (*keymanager.KeyInfo, error) {
	if m.keyManager == nil {
		return nil, fmt.Errorf("key manager not initialized")
	}

	return m.keyManager.GetKeyInfo(roomID)
}
