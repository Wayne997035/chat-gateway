package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// AESCTREncryption AES-256-CTR 加密實現
// CTR 模式特點：
// - 將塊密碼轉換為流密碼
// - 可並行加密/解密
// - 不需要填充
// - 適合大數據加密
type AESCTREncryption struct {
	key []byte // 256-bit (32 bytes) key
}

// NewAESCTREncryption 創建 AES-256-CTR 加密實例
func NewAESCTREncryption(key []byte) (*AESCTREncryption, error) {
	// 驗證密鑰長度必須是 32 bytes (256 bits)
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes (256 bits), got %d bytes", len(key))
	}
	
	// 防禦性複製密鑰（安全增強）
	keyCopy := make([]byte, len(key))
	copy(keyCopy, key)
	
	return &AESCTREncryption{
		key: keyCopy,
	}, nil
}

// Encrypt 加密數據
// 格式: "aes256ctr:" + base64(IV + ciphertext)
func (e *AESCTREncryption) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", fmt.Errorf("plaintext cannot be empty")
	}
	
	// 創建 AES cipher block
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// 將明文轉為字節
	plaintextBytes := []byte(plaintext)
	
	// 使用完後清零明文字節（安全增強）
	defer func() {
		for i := range plaintextBytes {
			plaintextBytes[i] = 0
		}
	}()
	
	// 創建密文緩衝區（與明文同樣大小）
	ciphertext := make([]byte, len(plaintextBytes))
	
	// 生成隨機 IV (Initialization Vector)
	// CTR 模式 IV 長度等於 block size (16 bytes for AES)
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}
	
	// 創建 CTR 模式加密器
	stream := cipher.NewCTR(block, iv)
	
	// 加密數據
	stream.XORKeyStream(ciphertext, plaintextBytes)
	
	// 將 IV 和密文組合（IV 在前）
	result := make([]byte, aes.BlockSize+len(ciphertext))
	copy(result[:aes.BlockSize], iv)
	copy(result[aes.BlockSize:], ciphertext)
	
	// 使用完後清零臨時緩衝區（安全增強）
	defer func() {
		for i := range result {
			result[i] = 0
		}
		for i := range ciphertext {
			ciphertext[i] = 0
		}
	}()
	
	// Base64 編碼以便存儲和傳輸
	encoded := base64.StdEncoding.EncodeToString(result)
	
	return "aes256ctr:" + encoded, nil
}

// Decrypt 解密數據
func (e *AESCTREncryption) Decrypt(encryptedText string) (string, error) {
	if encryptedText == "" {
		return "", fmt.Errorf("encrypted text cannot be empty")
	}
	
	// 檢查格式前綴
	prefix := "aes256ctr:"
	if len(encryptedText) < len(prefix) || encryptedText[:len(prefix)] != prefix {
		return "", fmt.Errorf("invalid ciphertext format: missing 'aes256ctr:' prefix")
	}
	
	// 移除前綴並 Base64 解碼
	encoded := encryptedText[len(prefix):]
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}
	
	// 使用完後清零（安全增強）
	defer func() {
		for i := range data {
			data[i] = 0
		}
	}()
	
	// 檢查數據長度（至少要有 IV）
	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short: must be at least %d bytes", aes.BlockSize)
	}
	
	// 創建 AES cipher block
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// 提取 IV 和密文
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]
	
	// 創建明文緩衝區
	plaintext := make([]byte, len(ciphertext))
	
	// 創建 CTR 模式解密器
	stream := cipher.NewCTR(block, iv)
	
	// 解密數據
	stream.XORKeyStream(plaintext, ciphertext)
	
	return string(plaintext), nil
}

// EncryptBytes 加密字節數據（用於文件等）
func (e *AESCTREncryption) EncryptBytes(plaintext []byte) ([]byte, error) {
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("plaintext cannot be empty")
	}
	
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	ciphertext := make([]byte, len(plaintext))
	
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}
	
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext, plaintext)
	
	result := make([]byte, aes.BlockSize+len(ciphertext))
	copy(result[:aes.BlockSize], iv)
	copy(result[aes.BlockSize:], ciphertext)
	
	return result, nil
}

// DecryptBytes 解密字節數據
func (e *AESCTREncryption) DecryptBytes(encryptedData []byte) ([]byte, error) {
	if len(encryptedData) < aes.BlockSize {
		return nil, fmt.Errorf("encrypted data too short")
	}
	
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	iv := encryptedData[:aes.BlockSize]
	ciphertext := encryptedData[aes.BlockSize:]
	
	plaintext := make([]byte, len(ciphertext))
	
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)
	
	return plaintext, nil
}

// IsEncrypted 檢查文本是否已加密
func (e *AESCTREncryption) IsEncrypted(text string) bool {
	return len(text) >= 10 && text[:10] == "aes256ctr:"
}

