# 🛠️ 安全修復實施路線圖

## 概述
本文檔提供詳細的、可執行的安全修復步驟。

---

## 📋 修復優先級矩陣

| 優先級 | 時間框架 | 數量 | 阻擋上線 |
|-------|----------|------|----------|
| P0 - 致命 | 立即（今天） | 3 項 | ✅ 是 |
| P1 - 嚴重 | 本週內 | 6 項 | ✅ 是 |
| P2 - 高危 | 本月內 | 6 項 | ⚠️ 建議 |
| P3 - 中危 | 季度內 | 3 項 | ❌ 否 |

---

## 🚨 P0 - 立即修復（阻擋上線）

### P0-1: 移除配置文件中的明文密碼

**時間**: 1 小時  
**難度**: 簡單  
**影響**: 低

**步驟**:

1. 修改 `configs/local.yaml`：
```yaml
database:
  mongo:
    url: "mongodb://localhost:27017/chatroom"
    username: ""  # ✅ 移除明文
    password: ""  # ✅ 移除明文
```

2. 修改 `internal/platform/driver/mongo.go`：
```go
func NewMongoDB(cfg *config.Config) (*mongo.Database, error) {
    // ✅ 從環境變量讀取
    mongoURL := os.Getenv("MONGO_URL")
    if mongoURL == "" {
        mongoURL = cfg.Database.Mongo.URL
    }
    
    mongoUsername := os.Getenv("MONGO_USERNAME")
    mongoPassword := os.Getenv("MONGO_PASSWORD")
    
    credential := options.Credential{
        Username: mongoUsername,
        Password: mongoPassword,
    }
    
    clientOptions := options.Client().
        ApplyURI(mongoURL).
        SetAuth(credential)
    // ... rest of code
}
```

3. 創建 `.env` 文件（不提交到 Git）：
```bash
MONGO_URL=mongodb://localhost:27017/chatroom
MONGO_USERNAME=your_username
MONGO_PASSWORD=your_secure_password
JWT_SECRET=your-secure-jwt-secret-min-32-chars
```

4. 更新 `.gitignore`：
```
.env
.env.local
.env.production
configs/*local*.yaml
```

**驗證**:
```bash
# 檢查配置文件沒有明文密碼
grep -r "password.*localPassword" configs/
# 應該沒有結果

# 檢查 .env 不在 Git 中
git status .env
# 應該被忽略
```

---

### P0-2: 禁用假加密或添加明確警告

**時間**: 30 分鐘  
**難度**: 簡單  
**影響**: 中

**選項 A：完全禁用加密（推薦用於開發）**

修改 `configs/local.yaml`：
```yaml
security:
  encryption:
    enabled: false  # ✅ 明確禁用
    algorithm: "NONE"  # ✅ 明確標註
```

修改 `internal/security/encryption/message_encryption.go`：
```go
func (m *MessageEncryption) EncryptMessage(content string, roomID string) (string, error) {
    if !m.enabled {
        // ✅ 添加明確日誌警告
        log.Println("⚠️ WARNING: Message encryption is DISABLED. Messages are stored in PLAIN TEXT!")
        return content, nil
    }
    
    // ❌ 移除假的"加密"
    // return m.simpleEncrypt(content)
    return "", fmt.Errorf("encryption not properly implemented")
}
```

**選項 B：添加"未加密"前綴**

```go
func (m *MessageEncryption) EncryptMessage(content string, roomID string) (string, error) {
    if !m.enabled {
        // ✅ 添加明確前綴
        return "PLAINTEXT:" + content, nil
    }
    
    return "", fmt.Errorf("encryption not properly implemented")
}
```

---

### P0-3: 修復最後訊息明文洩露

**時間**: 1 小時  
**難度**: 中  
**影響**: 中

修改 `internal/grpc/server.go:486-496`：

```go
// ❌ 舊代碼
lastMessagePreview := req.Content
if len(lastMessagePreview) > 50 {
    lastMessagePreview = lastMessagePreview[:50] + "..."
}
```

```go
// ✅ 選項 1：加密預覽
lastMessagePreview := "*** 加密訊息 ***"

// ✅ 選項 2：不顯示內容
lastMessagePreview := fmt.Sprintf("[%s 發送了訊息]", req.SenderId)

// ✅ 選項 3：顯示類型
lastMessagePreview := fmt.Sprintf("[%s]", req.Type)  // [text], [image], [file]
```

完整修復：
```go
// 更新聊天室的最後訊息
var lastMessagePreview string
switch req.Type {
case "text":
    lastMessagePreview = "[文字訊息]"
case "image":
    lastMessagePreview = "[圖片]"
case "file":
    lastMessagePreview = "[文件]"
default:
    lastMessagePreview = "[訊息]"
}

err = s.repos.ChatRoom.Update(ctx, req.RoomId, map[string]interface{}{
    "last_message":      lastMessagePreview,
    "last_message_time": message.CreatedAt,
    "last_message_at":   message.CreatedAt,
    "updated_at":        message.CreatedAt,
})
```

---

## 🔥 P1 - 本週內修復（高優先級）

### P1-1: 實現真正的 AES-256-GCM 加密

**時間**: 4-6 小時  
**難度**: 中  
**影響**: 高

創建 `internal/security/encryption/aes_gcm.go`：

```go
package encryption

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "io"
)

// AESGCMEncryption AES-256-GCM 加密實現
type AESGCMEncryption struct {
    key []byte  // 256-bit key
}

// NewAESGCMEncryption 創建 AES-GCM 加密實例
func NewAESGCMEncryption(key []byte) (*AESGCMEncryption, error) {
    if len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes (256 bits)")
    }
    
    return &AESGCMEncryption{
        key: key,
    }, nil
}

// Encrypt 加密數據
func (e *AESGCMEncryption) Encrypt(plaintext string) (string, error) {
    // 創建 AES cipher
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %w", err)
    }
    
    // 創建 GCM
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("failed to create GCM: %w", err)
    }
    
    // 生成隨機 nonce (12 bytes for GCM)
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", fmt.Errorf("failed to generate nonce: %w", err)
    }
    
    // 加密（nonce 會被附加到密文前面）
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    
    // Base64 編碼以便存儲
    encoded := base64.StdEncoding.EncodeToString(ciphertext)
    
    return "aes256gcm:" + encoded, nil
}

// Decrypt 解密數據
func (e *AESGCMEncryption) Decrypt(ciphertext string) (string, error) {
    // 檢查前綴
    if len(ciphertext) < 10 || ciphertext[:10] != "aes256gcm:" {
        return "", fmt.Errorf("invalid ciphertext format")
    }
    
    // 移除前綴並 Base64 解碼
    encoded := ciphertext[10:]
    data, err := base64.StdEncoding.DecodeString(encoded)
    if err != nil {
        return "", fmt.Errorf("failed to decode ciphertext: %w", err)
    }
    
    // 創建 AES cipher
    block, err := aes.NewCipher(e.key)
    if err != nil {
        return "", fmt.Errorf("failed to create cipher: %w", err)
    }
    
    // 創建 GCM
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("failed to create GCM: %w", err)
    }
    
    // 檢查數據長度
    nonceSize := gcm.NonceSize()
    if len(data) < nonceSize {
        return "", fmt.Errorf("ciphertext too short")
    }
    
    // 提取 nonce 和密文
    nonce, ciphertext := data[:nonceSize], data[nonceSize:]
    
    // 解密
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", fmt.Errorf("decryption failed: %w", err)
    }
    
    return string(plaintext), nil
}
```

**測試代碼**:
```go
package encryption

import "testing"

func TestAESGCMEncryption(t *testing.T) {
    // 生成測試密鑰
    key := make([]byte, 32)
    for i := range key {
        key[i] = byte(i)
    }
    
    enc, err := NewAESGCMEncryption(key)
    if err != nil {
        t.Fatal(err)
    }
    
    // 測試加密
    plaintext := "Hello, World! 你好世界！🔐"
    ciphertext, err := enc.Encrypt(plaintext)
    if err != nil {
        t.Fatal(err)
    }
    
    // 驗證格式
    if ciphertext[:10] != "aes256gcm:" {
        t.Error("Invalid ciphertext format")
    }
    
    // 測試解密
    decrypted, err := enc.Decrypt(ciphertext)
    if err != nil {
        t.Fatal(err)
    }
    
    if decrypted != plaintext {
        t.Errorf("Decryption failed: got %s, want %s", decrypted, plaintext)
    }
    
    // 測試不同密鑰無法解密
    wrongKey := make([]byte, 32)
    enc2, _ := NewAESGCMEncryption(wrongKey)
    _, err = enc2.Decrypt(ciphertext)
    if err == nil {
        t.Error("Should not decrypt with wrong key")
    }
}
```

---

### P1-2: 實現基本密鑰管理

**時間**: 6-8 小時  
**難度**: 高  
**影響**: 高

創建 `internal/security/keymanager/key_manager.go`：

```go
package keymanager

import (
    "crypto/rand"
    "encoding/base64"
    "fmt"
    "sync"
)

// KeyManager 密鑰管理器
type KeyManager struct {
    mu    sync.RWMutex
    keys  map[string]*Key  // roomID -> Key
    vault VaultClient      // Vault 客戶端（可選）
}

// Key 加密密鑰
type Key struct {
    ID        string
    Value     []byte
    CreatedAt int64
    RotatedAt int64
    Version   int
}

// NewKeyManager 創建密鑰管理器
func NewKeyManager() *KeyManager {
    return &KeyManager{
        keys: make(map[string]*Key),
    }
}

// GetRoomKey 獲取聊天室密鑰
func (km *KeyManager) GetRoomKey(roomID string) ([]byte, error) {
    km.mu.RLock()
    key, exists := km.keys[roomID]
    km.mu.RUnlock()
    
    if exists {
        return key.Value, nil
    }
    
    // 密鑰不存在，生成新密鑰
    return km.GenerateRoomKey(roomID)
}

// GenerateRoomKey 生成聊天室密鑰
func (km *KeyManager) GenerateRoomKey(roomID string) ([]byte, error) {
    km.mu.Lock()
    defer km.mu.Unlock()
    
    // 生成 256-bit 密鑰
    keyValue := make([]byte, 32)
    if _, err := rand.Read(keyValue); err != nil {
        return nil, fmt.Errorf("failed to generate key: %w", err)
    }
    
    key := &Key{
        ID:        roomID,
        Value:     keyValue,
        CreatedAt: time.Now().Unix(),
        Version:   1,
    }
    
    km.keys[roomID] = key
    
    // TODO: 持久化到 Vault 或加密數據庫
    // if km.vault != nil {
    //     km.vault.StoreKey(roomID, keyValue)
    // }
    
    return keyValue, nil
}

// RotateKey 輪換密鑰
func (km *KeyManager) RotateKey(roomID string) error {
    km.mu.Lock()
    defer km.mu.Unlock()
    
    oldKey, exists := km.keys[roomID]
    if !exists {
        return fmt.Errorf("key not found")
    }
    
    // 生成新密鑰
    newKeyValue := make([]byte, 32)
    if _, err := rand.Read(newKeyValue); err != nil {
        return fmt.Errorf("failed to generate new key: %w", err)
    }
    
    newKey := &Key{
        ID:        roomID,
        Value:     newKeyValue,
        CreatedAt: oldKey.CreatedAt,
        RotatedAt: time.Now().Unix(),
        Version:   oldKey.Version + 1,
    }
    
    km.keys[roomID] = newKey
    
    // TODO: 保留舊密鑰一段時間以解密舊訊息
    // km.oldKeys[roomID] = append(km.oldKeys[roomID], oldKey)
    
    return nil
}

// ExportKey 導出密鑰（用於備份）
func (km *KeyManager) ExportKey(roomID string) (string, error) {
    km.mu.RLock()
    defer km.mu.RUnlock()
    
    key, exists := km.keys[roomID]
    if !exists {
        return "", fmt.Errorf("key not found")
    }
    
    // Base64 編碼
    encoded := base64.StdEncoding.EncodeToString(key.Value)
    return encoded, nil
}

// ImportKey 導入密鑰（從備份恢復）
func (km *KeyManager) ImportKey(roomID, encodedKey string) error {
    // Base64 解碼
    keyValue, err := base64.StdEncoding.DecodeString(encodedKey)
    if err != nil {
        return fmt.Errorf("invalid key format: %w", err)
    }
    
    if len(keyValue) != 32 {
        return fmt.Errorf("invalid key length: must be 32 bytes")
    }
    
    km.mu.Lock()
    defer km.mu.Unlock()
    
    key := &Key{
        ID:        roomID,
        Value:     keyValue,
        CreatedAt: time.Now().Unix(),
        Version:   1,
    }
    
    km.keys[roomID] = key
    return nil
}
```

---

### P1-3: 整合 AES-GCM 和密鑰管理

**時間**: 2-3 小時  
**難度**: 中  
**影響**: 高

修改 `internal/security/encryption/message_encryption.go`：

```go
package encryption

import (
    "fmt"
    "chat-gateway/internal/security/keymanager"
)

// MessageEncryption 消息加密服務
type MessageEncryption struct {
    enabled    bool
    keyManager *keymanager.KeyManager
}

// NewMessageEncryption 創建消息加密服務
func NewMessageEncryption(enabled bool, km *keymanager.KeyManager) *MessageEncryption {
    return &MessageEncryption{
        enabled:    enabled,
        keyManager: km,
    }
}

// EncryptMessage 加密消息
func (m *MessageEncryption) EncryptMessage(content string, roomID string) (string, error) {
    if !m.enabled {
        return "plaintext:" + content, nil
    }
    
    // 獲取聊天室密鑰
    key, err := m.keyManager.GetRoomKey(roomID)
    if err != nil {
        return "", fmt.Errorf("failed to get room key: %w", err)
    }
    
    // 創建 AES-GCM 加密器
    aesGCM, err := NewAESGCMEncryption(key)
    if err != nil {
        return "", fmt.Errorf("failed to create encryptor: %w", err)
    }
    
    // 加密
    encrypted, err := aesGCM.Encrypt(content)
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
    
    // 獲取聊天室密鑰
    key, err := m.keyManager.GetRoomKey(roomID)
    if err != nil {
        return "", fmt.Errorf("failed to get room key: %w", err)
    }
    
    // 創建 AES-GCM 解密器
    aesGCM, err := NewAESGCMEncryption(key)
    if err != nil {
        return "", fmt.Errorf("failed to create decryptor: %w", err)
    }
    
    // 解密
    decrypted, err := aesGCM.Decrypt(encryptedContent)
    if err != nil {
        return "", fmt.Errorf("decryption failed: %w", err)
    }
    
    return decrypted, nil
}
```

---

## 📝 實施檢查清單

### P0 檢查清單（今天）
- [ ] 移除配置文件中的明文密碼
- [ ] 創建 `.env` 文件（不提交到 Git）
- [ ] 更新 `.gitignore`
- [ ] 禁用假加密或添加警告
- [ ] 修復最後訊息明文洩露
- [ ] 驗證所有修改

### P1 檢查清單（本週）
- [ ] 實現 AES-256-GCM 加密
- [ ] 編寫加密單元測試
- [ ] 實現密鑰管理器
- [ ] 整合加密和密鑰管理
- [ ] 測試端到端加密流程
- [ ] 更新文檔

---

## 🎯 成功標準

### P0 完成標準：
1. ✅ Git 歷史中沒有明文密碼
2. ✅ 配置文件使用環境變量
3. ✅ 加密被明確禁用或警告
4. ✅ 最後訊息不洩露內容

### P1 完成標準：
1. ✅ 使用真正的 AES-256-GCM 加密
2. ✅ 通過加密單元測試
3. ✅ 密鑰安全存儲
4. ✅ 可以加密和解密訊息

---

**本文檔應該每天更新進度！**

