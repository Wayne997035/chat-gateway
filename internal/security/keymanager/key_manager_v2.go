package keymanager

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"
	
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// KeyManagerWithPersistence 帶持久化的密鑰管理器
type KeyManagerWithPersistence struct {
	mu              sync.RWMutex
	keys            map[string]*Key       // roomID -> 當前密鑰（緩存）
	oldKeys         map[string][]*Key     // roomID -> 歷史密鑰（緩存）
	masterKey       []byte                 // 主密鑰（用於加密存儲的密鑰）
	store           *KeyStore              // 持久化存儲
	rotationPolicy  RotationPolicy
	stopChan        chan struct{}
	running         bool
}

// NewKeyManagerWithPersistence 創建帶持久化的密鑰管理器
func NewKeyManagerWithPersistence(masterKey []byte, db *mongo.Database) (*KeyManagerWithPersistence, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (256 bits)")
	}
	
	km := &KeyManagerWithPersistence{
		keys:      make(map[string]*Key),
		oldKeys:   make(map[string][]*Key),
		masterKey: masterKey,
		store:     NewKeyStore(db),
		rotationPolicy: RotationPolicy{
			Enabled:          false,
			RotationInterval: 24 * time.Hour,
			MaxKeyAge:        30 * 24 * time.Hour,
			KeepOldKeys:      5,
		},
	}
	
	// 啟動時清理過期密鑰
	go func() {
		count, err := km.store.DeleteExpiredKeys(context.Background())
		if err == nil && count > 0 {
			fmt.Printf("Cleaned up %d expired keys\n", count)
		}
	}()
	
	return km, nil
}

// GetOrCreateRoomKey 獲取或創建聊天室密鑰（帶 DB 持久化）
func (km *KeyManagerWithPersistence) GetOrCreateRoomKey(roomID string) ([]byte, error) {
	if roomID == "" {
		return nil, fmt.Errorf("roomID cannot be empty")
	}
	
	// 1. 先檢查內存緩存
	km.mu.RLock()
	key, exists := km.keys[roomID]
	km.mu.RUnlock()
	
	if exists && key.Status == KeyStatusActive {
		return key.Value, nil
	}
	
	// 2. 從數據庫加載
	ctx := context.Background()
	keyDoc, err := km.store.GetActiveKey(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("failed to load key from DB: %w", err)
	}
	
	if keyDoc != nil {
		// 解密密鑰
		roomKey, err := km.decryptRoomKey(keyDoc.EncryptedKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt room key: %w", err)
		}
		
		// 加載到緩存
		key := &Key{
			ID:        roomID,
			Value:     roomKey,
			CreatedAt: keyDoc.CreatedAt,
			RotatedAt: keyDoc.RotatedAt,
			Version:   keyDoc.KeyVersion,
			Status:    KeyStatusActive,
		}
		
		km.mu.Lock()
		km.keys[roomID] = key
		km.mu.Unlock()
		
		return roomKey, nil
	}
	
	// 3. 密鑰不存在，創建新密鑰
	return km.createRoomKey(roomID)
}

// createRoomKey 創建新的聊天室密鑰（保存到 DB）
func (km *KeyManagerWithPersistence) createRoomKey(roomID string) ([]byte, error) {
	km.mu.Lock()
	defer km.mu.Unlock()
	
	// 再次檢查（防止並發創建）
	if key, exists := km.keys[roomID]; exists && key.Status == KeyStatusActive {
		return key.Value, nil
	}
	
	// 生成 256-bit 隨機密鑰
	keyValue := make([]byte, 32)
	if _, err := rand.Read(keyValue); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	
	now := time.Now()
	key := &Key{
		ID:        roomID,
		Value:     keyValue,
		CreatedAt: now,
		RotatedAt: now,
		Version:   1,
		Status:    KeyStatusActive,
	}
	
	// 用 Master Key 加密 Room Key
	encryptedKey, err := km.encryptRoomKey(keyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt room key: %w", err)
	}
	
	// 保存到數據庫
	keyDoc := &KeyDocument{
		RoomID:       roomID,
		KeyVersion:   1,
		EncryptedKey: encryptedKey,
		CreatedAt:    now,
		RotatedAt:    now,
		IsActive:     true,
		ExpiresAt:    now.Add(km.rotationPolicy.MaxKeyAge),
	}
	
	ctx := context.Background()
	if err := km.store.SaveKey(ctx, keyDoc); err != nil {
		return nil, fmt.Errorf("failed to save key to DB: %w", err)
	}
	
	// 加載到緩存
	km.keys[roomID] = key
	
	return keyValue, nil
}

// rotateKey 輪換密鑰（保存到 DB）
func (km *KeyManagerWithPersistence) rotateKey(roomID string) ([]byte, error) {
	km.mu.Lock()
	defer km.mu.Unlock()
	
	oldKey, exists := km.keys[roomID]
	if !exists {
		return nil, fmt.Errorf("key not found for room %s", roomID)
	}
	
	// 生成新密鑰
	newKeyValue := make([]byte, 32)
	if _, err := rand.Read(newKeyValue); err != nil {
		return nil, fmt.Errorf("failed to generate new key: %w", err)
	}
	
	now := time.Now()
	newVersion := oldKey.Version + 1
	
	// 創建新密鑰
	newKey := &Key{
		ID:        roomID,
		Value:     newKeyValue,
		CreatedAt: oldKey.CreatedAt,
		RotatedAt: now,
		Version:   newVersion,
		Status:    KeyStatusActive,
	}
	
	// 用 Master Key 加密新的 Room Key
	encryptedKey, err := km.encryptRoomKey(newKeyValue)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt new room key: %w", err)
	}
	
	// 保存新密鑰到數據庫
	newKeyDoc := &KeyDocument{
		RoomID:       roomID,
		KeyVersion:   newVersion,
		EncryptedKey: encryptedKey,
		CreatedAt:    oldKey.CreatedAt,
		RotatedAt:    now,
		IsActive:     true,
		ExpiresAt:    now.Add(km.rotationPolicy.MaxKeyAge),
	}
	
	ctx := context.Background()
	if err := km.store.SaveKey(ctx, newKeyDoc); err != nil {
		return nil, fmt.Errorf("failed to save new key to DB: %w", err)
	}
	
	// 歸檔舊密鑰
	oldKey.Status = KeyStatusArchived
	if km.oldKeys[roomID] == nil {
		km.oldKeys[roomID] = make([]*Key, 0)
	}
	km.oldKeys[roomID] = append(km.oldKeys[roomID], oldKey)
	
	// 清理過舊的密鑰
	km.cleanupOldKeys(roomID)
	
	// 更新當前密鑰
	km.keys[roomID] = newKey
	
	return newKeyValue, nil
}

// encryptRoomKey 用 Master Key 加密 Room Key
func (km *KeyManagerWithPersistence) encryptRoomKey(roomKey []byte) (string, error) {
	block, err := aes.NewCipher(km.masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// 生成隨機 IV
	ciphertext := make([]byte, aes.BlockSize+len(roomKey))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}
	
	// 使用 CTR 模式加密
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], roomKey)
	
	// Base64 編碼
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptRoomKey 用 Master Key 解密 Room Key
func (km *KeyManagerWithPersistence) decryptRoomKey(encryptedKey string) ([]byte, error) {
	// Base64 解碼
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	
	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	
	block, err := aes.NewCipher(km.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	
	// 提取 IV
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	
	// 使用 CTR 模式解密
	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)
	
	return plaintext, nil
}

// LoadAllKeys 從數據庫加載所有密鑰（啟動時使用）
func (km *KeyManagerWithPersistence) LoadAllKeys(ctx context.Context, roomID string) error {
	// 獲取所有密鑰版本
	keyDocs, err := km.store.GetAllKeys(ctx, roomID)
	if err != nil {
		return fmt.Errorf("failed to load keys from DB: %w", err)
	}
	
	km.mu.Lock()
	defer km.mu.Unlock()
	
	for _, keyDoc := range keyDocs {
		// 解密密鑰
		roomKey, err := km.decryptRoomKey(keyDoc.EncryptedKey)
		if err != nil {
			return fmt.Errorf("failed to decrypt room key (version %d): %w", keyDoc.KeyVersion, err)
		}
		
		key := &Key{
			ID:        keyDoc.RoomID,
			Value:     roomKey,
			CreatedAt: keyDoc.CreatedAt,
			RotatedAt: keyDoc.RotatedAt,
			Version:   keyDoc.KeyVersion,
			Status:    KeyStatusActive,
		}
		
		if keyDoc.IsActive {
			// 活躍密鑰
			km.keys[roomID] = key
		} else {
			// 歷史密鑰
			key.Status = KeyStatusArchived
			if km.oldKeys[roomID] == nil {
				km.oldKeys[roomID] = make([]*Key, 0)
			}
			km.oldKeys[roomID] = append(km.oldKeys[roomID], key)
		}
	}
	
	return nil
}

// cleanupOldKeys 清理過舊的密鑰
func (km *KeyManagerWithPersistence) cleanupOldKeys(roomID string) {
	oldKeyList := km.oldKeys[roomID]
	if len(oldKeyList) <= km.rotationPolicy.KeepOldKeys {
		return
	}
	
	// 只保留最新的 N 個密鑰
	km.oldKeys[roomID] = oldKeyList[len(oldKeyList)-km.rotationPolicy.KeepOldKeys:]
}

// shouldRotateKey 判斷是否需要輪換密鑰
func (km *KeyManagerWithPersistence) shouldRotateKey(key *Key) bool {
	if !km.rotationPolicy.Enabled {
		return false
	}
	
	now := time.Now()
	
	// 檢查密鑰年齡
	if km.rotationPolicy.MaxKeyAge > 0 {
		if now.Sub(key.CreatedAt) > km.rotationPolicy.MaxKeyAge {
			return true
		}
	}
	
	// 檢查輪換間隔
	if km.rotationPolicy.RotationInterval > 0 {
		if now.Sub(key.RotatedAt) > km.rotationPolicy.RotationInterval {
			return true
		}
	}
	
	return false
}

// GetKeyInfo 獲取密鑰信息（不返回密鑰值）
func (km *KeyManagerWithPersistence) GetKeyInfo(roomID string) (*KeyInfo, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	
	key, exists := km.keys[roomID]
	if !exists {
		return nil, fmt.Errorf("key not found for room %s", roomID)
	}
	
	return &KeyInfo{
		RoomID:    key.ID,
		Version:   key.Version,
		CreatedAt: key.CreatedAt,
		RotatedAt: key.RotatedAt,
		Status:    key.Status,
		Age:       time.Since(key.CreatedAt),
	}, nil
}

// SetRotationPolicy 設置密鑰輪換策略
func (km *KeyManagerWithPersistence) SetRotationPolicy(policy RotationPolicy) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.rotationPolicy = policy
}

// StartAutoRotation 啟動自動密鑰輪換
func (km *KeyManagerWithPersistence) StartAutoRotation() {
	km.mu.Lock()
	defer km.mu.Unlock()
	
	if km.running {
		return
	}
	
	km.stopChan = make(chan struct{})
	km.running = true
	
	go km.autoRotationLoop()
}

// StopAutoRotation 停止自動密鑰輪換
func (km *KeyManagerWithPersistence) StopAutoRotation() {
	km.mu.Lock()
	defer km.mu.Unlock()
	
	if !km.running {
		return
	}
	
	close(km.stopChan)
	km.running = false
}

// autoRotationLoop 自動輪換循環
func (km *KeyManagerWithPersistence) autoRotationLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			km.checkAndRotateKeys()
		case <-km.stopChan:
			return
		}
	}
}

// checkAndRotateKeys 檢查並輪換需要輪換的密鑰
func (km *KeyManagerWithPersistence) checkAndRotateKeys() {
	km.mu.RLock()
	keysToRotate := make([]string, 0)
	
	for roomID, key := range km.keys {
		if km.shouldRotateKey(key) {
			keysToRotate = append(keysToRotate, roomID)
		}
	}
	km.mu.RUnlock()
	
	// 輪換需要輪換的密鑰
	for _, roomID := range keysToRotate {
		if _, err := km.rotateKey(roomID); err != nil {
			fmt.Printf("Failed to rotate key for room %s: %v\n", roomID, err)
		} else {
			fmt.Printf("Successfully rotated key for room %s\n", roomID)
		}
	}
}

// ForceRotateKey 強制輪換指定聊天室的密鑰
func (km *KeyManagerWithPersistence) ForceRotateKey(roomID string) error {
	km.mu.RLock()
	_, exists := km.keys[roomID]
	km.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("key not found for room %s", roomID)
	}
	
	_, err := km.rotateKey(roomID)
	return err
}

// Stats 獲取統計信息
func (km *KeyManagerWithPersistence) Stats() KeyManagerStats {
	km.mu.RLock()
	defer km.mu.RUnlock()
	
	stats := KeyManagerStats{
		TotalKeys:     len(km.keys),
		ArchivedKeys:  0,
		ActiveKeys:    0,
		RevokedKeys:   0,
	}
	
	for _, key := range km.keys {
		switch key.Status {
		case KeyStatusActive:
			stats.ActiveKeys++
		case KeyStatusRevoked:
			stats.RevokedKeys++
		}
	}
	
	for _, keyList := range km.oldKeys {
		stats.ArchivedKeys += len(keyList)
	}
	
	return stats
}
