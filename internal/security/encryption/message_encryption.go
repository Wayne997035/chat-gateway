package encryption

import (
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"chat-gateway/internal/security/keymanager"
)

const plaintextPrefix = "plaintext:"

// MessageEncryption 消息加密服務，使用 AES-256-GCM AEAD 加密模式
type MessageEncryption struct {
	enabled    bool
	keyManager *keymanager.KeyManagerWithPersistence
	warnOnce   sync.Once
}

// NewMessageEncryption 創建消息加密服務
func NewMessageEncryption(enabled bool, km *keymanager.KeyManagerWithPersistence) *MessageEncryption {
	if km == nil {
		slog.Warn("[WARNING] KeyManager is nil. Encryption will be disabled.")
		enabled = false
	}

	return &MessageEncryption{
		enabled:    enabled,
		keyManager: km,
	}
}

// EncryptMessage 使用 AES-256-GCM 加密消息，並在密文前加上 v{version}: 前綴
func (m *MessageEncryption) EncryptMessage(content, roomID string) (string, error) {
	if !m.enabled {
		m.warnOnce.Do(func() {
			slog.Warn("[WARNING] Message encryption is DISABLED. Messages are stored in PLAIN TEXT!")
		})
		return plaintextPrefix + content, nil
	}

	if m.keyManager == nil {
		return "", fmt.Errorf("key manager not initialized")
	}

	// GetActiveKeyWithVersion 原子性取得 key bytes 與版本號，避免兩次呼叫之間發生 rotation
	key, version, err := m.keyManager.GetActiveKeyWithVersion(roomID)
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

	return fmt.Sprintf("v%d:%s", version, encrypted), nil
}

// DecryptMessage 解密消息，支援 v{version}: 前綴路由至對應版本密鑰.
// 不帶版本前綴的密文視為 v1（向下相容）.
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

	// 解析版本前綴 v{N}:，不帶前綴時視為 v1
	version := 1
	ciphertext := encryptedContent
	if len(encryptedContent) > 2 && encryptedContent[0] == 'v' {
		colonIdx := strings.Index(encryptedContent[1:], ":")
		if colonIdx >= 0 {
			if n, parseErr := strconv.Atoi(encryptedContent[1 : 1+colonIdx]); parseErr == nil {
				version = n
				ciphertext = encryptedContent[1+colonIdx+1:]
			}
		}
	}

	// 版本號無效（0 或負數）視為 legacy v1，整個字串作為密文
	if version <= 0 {
		version = 1
		ciphertext = encryptedContent
	}

	key, err := m.keyManager.GetKeyByVersion(roomID, version)
	if err != nil {
		return "", fmt.Errorf("failed to get key for version %d: %w", version, err)
	}

	aesGCM, err := NewAESGCMEncryption(key)
	if err != nil {
		return "", fmt.Errorf("failed to create decryptor: %w", err)
	}

	decrypted, err := aesGCM.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return decrypted, nil
}

// IsEncrypted 檢查消息是否為 AES-256-GCM 加密格式（含或不含版本前綴）
func (m *MessageEncryption) IsEncrypted(content string) bool {
	// 帶版本前綴: v{N}:aes256gcm:...
	if len(content) > 2 && content[0] == 'v' {
		colonIdx := strings.Index(content[1:], ":")
		if colonIdx >= 0 {
			rest := content[1+colonIdx+1:]
			if len(rest) >= len(aes256GCMPrefix) && rest[:len(aes256GCMPrefix)] == aes256GCMPrefix {
				return true
			}
		}
	}
	// 不帶版本前綴的舊格式
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
