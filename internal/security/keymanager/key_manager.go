package keymanager

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync"
	"time"
)

// KeyManager 密鑰管理器
// 負責生成、存儲、輪換和管理加密密鑰
type KeyManager struct {
	mu              sync.RWMutex
	keys            map[string]*Key       // roomID -> 當前密鑰
	oldKeys         map[string][]*Key     // roomID -> 歷史密鑰（用於解密舊訊息）
	masterKey       []byte                 // 主密鑰（用於加密存儲的密鑰）
	rotationPolicy  RotationPolicy
	stopChan        chan struct{}          // 用於停止後台輪換任務
	running         bool                   // 是否正在運行
}

// Key 加密密鑰
type Key struct {
	ID        string    // 密鑰 ID (通常是 roomID)
	Value     []byte    // 256-bit 密鑰值
	CreatedAt time.Time // 創建時間
	RotatedAt time.Time // 最後輪換時間
	Version   int       // 密鑰版本
	Status    KeyStatus // 密鑰狀態
}

// KeyStatus 密鑰狀態
type KeyStatus string

const (
	KeyStatusActive   KeyStatus = "active"   // 活躍（用於加密）
	KeyStatusArchived KeyStatus = "archived" // 歸檔（只用於解密）
	KeyStatusRevoked  KeyStatus = "revoked"  // 撤銷（不可用）
)

// RotationPolicy 密鑰輪換策略
type RotationPolicy struct {
	Enabled          bool          // 是否啟用自動輪換
	RotationInterval time.Duration // 輪換間隔
	MaxKeyAge        time.Duration // 密鑰最大使用時間
	KeepOldKeys      int           // 保留的歷史密鑰數量
}

// NewKeyManager 創建密鑰管理器
func NewKeyManager(masterKey []byte) (*KeyManager, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes (256 bits)")
	}
	
	km := &KeyManager{
		keys:       make(map[string]*Key),
		oldKeys:    make(map[string][]*Key),
		masterKey:  masterKey,
		rotationPolicy: RotationPolicy{
			Enabled:          false, // 默認關閉自動輪換
			RotationInterval: 24 * time.Hour,
			MaxKeyAge:        30 * 24 * time.Hour,
			KeepOldKeys:      5,
		},
	}
	
	return km, nil
}

// GetOrCreateRoomKey 獲取或創建聊天室密鑰
func (km *KeyManager) GetOrCreateRoomKey(roomID string) ([]byte, error) {
	if roomID == "" {
		return nil, fmt.Errorf("roomID cannot be empty")
	}
	
	// 先嘗試讀取
	km.mu.RLock()
	key, exists := km.keys[roomID]
	km.mu.RUnlock()
	
	if exists && key.Status == KeyStatusActive {
		// 檢查是否需要輪換
		if km.shouldRotateKey(key) {
			return km.rotateKey(roomID)
		}
		return key.Value, nil
	}
	
	// 密鑰不存在，創建新密鑰
	return km.createRoomKey(roomID)
}

// createRoomKey 創建新的聊天室密鑰
func (km *KeyManager) createRoomKey(roomID string) ([]byte, error) {
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
	
	key := &Key{
		ID:        roomID,
		Value:     keyValue,
		CreatedAt: time.Now(),
		RotatedAt: time.Now(),
		Version:   1,
		Status:    KeyStatusActive,
	}
	
	km.keys[roomID] = key
	
	// TODO: 持久化到 Vault 或加密數據庫
	// if km.vault != nil {
	//     if err := km.vault.StoreKey(roomID, keyValue); err != nil {
	//         return nil, fmt.Errorf("failed to store key in vault: %w", err)
	//     }
	// }
	
	return keyValue, nil
}

// rotateKey 輪換密鑰
func (km *KeyManager) rotateKey(roomID string) ([]byte, error) {
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
	
	// 創建新密鑰
	newKey := &Key{
		ID:        roomID,
		Value:     newKeyValue,
		CreatedAt: oldKey.CreatedAt,
		RotatedAt: time.Now(),
		Version:   oldKey.Version + 1,
		Status:    KeyStatusActive,
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

// shouldRotateKey 判斷是否需要輪換密鑰
func (km *KeyManager) shouldRotateKey(key *Key) bool {
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

// cleanupOldKeys 清理過舊的密鑰
func (km *KeyManager) cleanupOldKeys(roomID string) {
	oldKeyList := km.oldKeys[roomID]
	if len(oldKeyList) <= km.rotationPolicy.KeepOldKeys {
		return
	}
	
	// 只保留最新的 N 個密鑰
	km.oldKeys[roomID] = oldKeyList[len(oldKeyList)-km.rotationPolicy.KeepOldKeys:]
}

// GetKeyForDecryption 獲取解密用密鑰（包括歷史密鑰）
func (km *KeyManager) GetKeyForDecryption(roomID string, version int) ([]byte, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	
	// 先檢查當前密鑰
	if key, exists := km.keys[roomID]; exists {
		if version == 0 || key.Version == version {
			return key.Value, nil
		}
	}
	
	// 檢查歷史密鑰
	if oldKeyList, exists := km.oldKeys[roomID]; exists {
		for _, key := range oldKeyList {
			if key.Version == version {
				if key.Status == KeyStatusRevoked {
					return nil, fmt.Errorf("key version %d has been revoked", version)
				}
				return key.Value, nil
			}
		}
	}
	
	return nil, fmt.Errorf("key version %d not found for room %s", version, roomID)
}

// RevokeKey 撤銷密鑰（緊急情況使用）
func (km *KeyManager) RevokeKey(roomID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()
	
	key, exists := km.keys[roomID]
	if !exists {
		return fmt.Errorf("key not found for room %s", roomID)
	}
	
	key.Status = KeyStatusRevoked
	
	// TODO: 通知 Vault
	// if km.vault != nil {
	//     km.vault.RevokeKey(roomID)
	// }
	
	return nil
}

// ExportKey 導出密鑰（用於備份）
func (km *KeyManager) ExportKey(roomID string) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	
	key, exists := km.keys[roomID]
	if !exists {
		return "", fmt.Errorf("key not found for room %s", roomID)
	}
	
	if key.Status == KeyStatusRevoked {
		return "", fmt.Errorf("cannot export revoked key")
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
		return fmt.Errorf("invalid key length: must be 32 bytes, got %d", len(keyValue))
	}
	
	km.mu.Lock()
	defer km.mu.Unlock()
	
	key := &Key{
		ID:        roomID,
		Value:     keyValue,
		CreatedAt: time.Now(),
		RotatedAt: time.Now(),
		Version:   1,
		Status:    KeyStatusActive,
	}
	
	km.keys[roomID] = key
	return nil
}

// SetRotationPolicy 設置密鑰輪換策略
func (km *KeyManager) SetRotationPolicy(policy RotationPolicy) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.rotationPolicy = policy
}

// GetKeyInfo 獲取密鑰信息（不返回密鑰值）
func (km *KeyManager) GetKeyInfo(roomID string) (*KeyInfo, error) {
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

// KeyInfo 密鑰信息（不包含密鑰值）
type KeyInfo struct {
	RoomID    string
	Version   int
	CreatedAt time.Time
	RotatedAt time.Time
	Status    KeyStatus
	Age       time.Duration
}

// Stats 獲取統計信息
func (km *KeyManager) Stats() KeyManagerStats {
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

// KeyManagerStats 密鑰管理器統計信息
type KeyManagerStats struct {
	TotalKeys    int
	ActiveKeys   int
	ArchivedKeys int
	RevokedKeys  int
}

// StartAutoRotation 啟動自動密鑰輪換
func (km *KeyManager) StartAutoRotation() {
	km.mu.Lock()
	defer km.mu.Unlock()
	
	if km.running {
		return // 已經在運行
	}
	
	km.stopChan = make(chan struct{})
	km.running = true
	
	go km.autoRotationLoop()
}

// StopAutoRotation 停止自動密鑰輪換
func (km *KeyManager) StopAutoRotation() {
	km.mu.Lock()
	defer km.mu.Unlock()
	
	if !km.running {
		return
	}
	
	close(km.stopChan)
	km.running = false
}

// autoRotationLoop 自動輪換循環
func (km *KeyManager) autoRotationLoop() {
	ticker := time.NewTicker(1 * time.Hour) // 每小時檢查一次
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
func (km *KeyManager) checkAndRotateKeys() {
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
			// 記錄錯誤（實際應該使用 logger）
			fmt.Printf("Failed to rotate key for room %s: %v\n", roomID, err)
		} else {
			fmt.Printf("Successfully rotated key for room %s\n", roomID)
		}
	}
}

// ForceRotateKey 強制輪換指定聊天室的密鑰（手動觸發）
func (km *KeyManager) ForceRotateKey(roomID string) error {
	km.mu.RLock()
	_, exists := km.keys[roomID]
	km.mu.RUnlock()
	
	if !exists {
		return fmt.Errorf("key not found for room %s", roomID)
	}
	
	_, err := km.rotateKey(roomID)
	return err
}

// RotateAllKeys 輪換所有活躍密鑰（批量操作）
func (km *KeyManager) RotateAllKeys() []error {
	km.mu.RLock()
	roomIDs := make([]string, 0, len(km.keys))
	for roomID := range km.keys {
		roomIDs = append(roomIDs, roomID)
	}
	km.mu.RUnlock()
	
	errors := make([]error, 0)
	for _, roomID := range roomIDs {
		if _, err := km.rotateKey(roomID); err != nil {
			errors = append(errors, fmt.Errorf("room %s: %w", roomID, err))
		}
	}
	
	return errors
}

