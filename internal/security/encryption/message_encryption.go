package encryption

import (
	"fmt"
	"log"
	
	"chat-gateway/internal/security/keymanager"
)

// MessageEncryption 消息加密服務
// 使用 AES-256-CTR 加密模式 + 密鑰管理器
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

// EncryptMessage 加密消息
// 使用 AES-256-CTR 加密模式
func (m *MessageEncryption) EncryptMessage(content string, roomID string) (string, error) {
	if !m.enabled {
		log.Println("[WARNING] Message encryption is DISABLED. Messages are stored in PLAIN TEXT!")
		return "plaintext:" + content, nil
	}
	
	if m.keyManager == nil {
		return "", fmt.Errorf("key manager not initialized")
	}
	
	// 獲取或創建聊天室密鑰
	key, err := m.keyManager.GetOrCreateRoomKey(roomID)
	if err != nil {
		return "", fmt.Errorf("failed to get room key: %w", err)
	}
	
	// 創建 AES-256-CTR 加密器
	aesCTR, err := NewAESCTREncryption(key)
	if err != nil {
		return "", fmt.Errorf("failed to create encryptor: %w", err)
	}
	
	// 加密訊息
	encrypted, err := aesCTR.Encrypt(content)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}
	
	return encrypted, nil
}

// DecryptMessage 解密消息
func (m *MessageEncryption) DecryptMessage(encryptedContent string, roomID string) (string, error) {
	if !m.enabled {
		// 檢查是否有 plaintext 前綴
		if len(encryptedContent) > 10 && encryptedContent[:10] == "plaintext:" {
			return encryptedContent[10:], nil
		}
		return encryptedContent, nil
	}
	
	if m.keyManager == nil {
		return "", fmt.Errorf("key manager not initialized")
	}
	
	// 檢查是否是舊格式（plaintext 或 encrypted）
	if len(encryptedContent) > 10 {
		prefix := encryptedContent[:10]
		if prefix == "plaintext:" {
			return encryptedContent[10:], nil
		}
		if prefix == "encrypted:" {
			// 舊的假加密格式，嘗試解碼
			log.Printf("[WARNING] Found old fake encryption format for room %s", roomID)
			return "", fmt.Errorf("old encryption format not supported, message cannot be decrypted")
		}
	}
	
	// 獲取聊天室密鑰
	key, err := m.keyManager.GetOrCreateRoomKey(roomID)
	if err != nil {
		return "", fmt.Errorf("failed to get room key: %w", err)
	}
	
	// 創建 AES-256-CTR 解密器
	aesCTR, err := NewAESCTREncryption(key)
	if err != nil {
		return "", fmt.Errorf("failed to create decryptor: %w", err)
	}
	
	// 解密訊息
	decrypted, err := aesCTR.Decrypt(encryptedContent)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}
	
	return decrypted, nil
}

// IsEncrypted 檢查消息是否已加密
func (m *MessageEncryption) IsEncrypted(content string) bool {
	if len(content) < 10 {
		return false
	}
	
	prefix := content[:10]
	// 支持 AES-256-CTR 格式
	if prefix == "aes256ctr:" {
		return true
	}
	
	// 舊格式
	if prefix == "encrypted:" || prefix == "plaintext:" {
		return false
	}
	
	return false
}

// GetKeyInfo 獲取密鑰信息（用於調試）
func (m *MessageEncryption) GetKeyInfo(roomID string) (*keymanager.KeyInfo, error) {
	if m.keyManager == nil {
		return nil, fmt.Errorf("key manager not initialized")
	}
	
	return m.keyManager.GetKeyInfo(roomID)
}
