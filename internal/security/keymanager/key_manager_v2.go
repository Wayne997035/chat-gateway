package keymanager

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/tink-crypto/tink-go/v2/tink"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
)

const (
	tinkEncryptedPrefix = "tink:"
	gcmEncryptedPrefix  = "gcm:"
)

// KeyManagerWithPersistence 帶持久化的密鑰管理器.
type KeyManagerWithPersistence struct {
	mu              sync.RWMutex
	keys            map[string]*Key   // roomID -> 當前密鑰（緩存）
	oldKeys         map[string][]*Key // roomID -> 歷史密鑰（緩存）
	kek             tink.AEAD         // KEK AEAD（Tink，用於加密/解密 room key）
	legacyMasterKey []byte            // 舊格式 gcm: 解密用（migration 完成後可為 nil）
	store           *KeyStore         // 持久化存儲
	rotationPolicy  RotationPolicy
	stopChan        chan struct{}
	running         bool
	logger          *zap.Logger
}

// NewKeyManagerWithPersistence 創建帶持久化的密鑰管理器.
// kek 是 Tink AEAD，用於新 tink: 格式的加密/解密。
// legacyMasterKey 是 32-byte raw key，用於讀取舊 gcm: 格式（migration 前）；migration 完成後傳 nil。
// logger 是結構化日誌記錄器；傳 nil 時使用 zap.NewNop()。
func NewKeyManagerWithPersistence(
	kek tink.AEAD, legacyMasterKey []byte, db *mongo.Database, logger *zap.Logger,
) (*KeyManagerWithPersistence, error) {
	if kek == nil {
		return nil, fmt.Errorf("kek AEAD must not be nil")
	}
	if legacyMasterKey != nil && len(legacyMasterKey) != 32 {
		return nil, fmt.Errorf("legacy master key must be 32 bytes (256 bits)")
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	// 防禦性複製 legacyMasterKey（安全增強：防止外部修改）
	var legacyCopy []byte
	if legacyMasterKey != nil {
		legacyCopy = make([]byte, len(legacyMasterKey))
		copy(legacyCopy, legacyMasterKey)
	}

	km := &KeyManagerWithPersistence{
		keys:            make(map[string]*Key),
		oldKeys:         make(map[string][]*Key),
		kek:             kek,
		legacyMasterKey: legacyCopy,
		store:           NewKeyStore(db),
		rotationPolicy: RotationPolicy{
			Enabled:          false,
			RotationInterval: 24 * time.Hour,
			MaxKeyAge:        30 * 24 * time.Hour,
			KeepOldKeys:      5,
		},
		logger: logger,
	}

	// 啟動時清理過期密鑰
	go func() {
		defer func() {
			if r := recover(); r != nil {
				km.logger.Error("key cleanup panicked", zap.Any("panic", r))
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		count, err := km.store.DeleteExpiredKeys(ctx)
		if err != nil {
			km.logger.Error("key cleanup failed", zap.Error(err))
		} else if count > 0 {
			km.logger.Info("expired keys cleaned up", zap.Int64("count", count))
		}
	}()

	return km, nil
}

// GetOrCreateRoomKey 獲取或創建聊天室密鑰（帶 DB 持久化）.
// 使用 Double-Check Locking 防止並發創建.
func (km *KeyManagerWithPersistence) GetOrCreateRoomKey(roomID string) ([]byte, error) {
	if roomID == "" {
		return nil, fmt.Errorf("roomID cannot be empty")
	}

	// 第一次檢查：使用讀鎖（快速路徑）
	km.mu.RLock()
	key, exists := km.keys[roomID]
	km.mu.RUnlock()

	if exists && key.Status == KeyStatusActive {
		return key.Value, nil
	}

	// 獲取寫鎖以進行創建或加載（慢速路徑）
	km.mu.Lock()
	defer km.mu.Unlock()

	// 第二次檢查：其他協程可能已經創建了密鑰
	if key, exists := km.keys[roomID]; exists && key.Status == KeyStatusActive {
		return key.Value, nil
	}

	// 從數據庫加載（在鎖內執行，確保只有一個協程執行）
	ctx := context.Background()
	keyDoc, err := km.store.GetActiveKey(ctx, roomID)
	if err != nil {
		return nil, fmt.Errorf("key loading error")
	}

	if keyDoc != nil {
		// 解密密鑰
		roomKey, err := km.decryptRoomKey(keyDoc.EncryptedKey)
		if err != nil {
			return nil, fmt.Errorf("key decryption error")
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
		km.keys[roomID] = key

		return roomKey, nil
	}

	// 密鑰不存在，創建新密鑰（已持有寫鎖，安全）
	return km.createRoomKeyUnsafe(roomID)
}

// createRoomKeyUnsafe 創建新的聊天室密鑰（不加鎖版本）.
// 調用者必須已經持有 km.mu 寫鎖.
func (km *KeyManagerWithPersistence) createRoomKeyUnsafe(roomID string) ([]byte, error) {
	// 再次檢查（防止並發創建）
	if key, exists := km.keys[roomID]; exists && key.Status == KeyStatusActive {
		return key.Value, nil
	}

	// 生成 256-bit 隨機密鑰
	keyValue := make([]byte, 32)
	if _, err := rand.Read(keyValue); err != nil {
		return nil, fmt.Errorf("key generation error")
	}

	// 使用完後清零（安全增強）
	defer func() {
		for i := range keyValue {
			keyValue[i] = 0
		}
	}()

	// 為緩存創建獨立的副本（避免被 defer 清零）
	keyValueForCache := make([]byte, 32)
	copy(keyValueForCache, keyValue)

	now := time.Now()
	key := &Key{
		ID:        roomID,
		Value:     keyValueForCache, // 使用副本，不會被清零
		CreatedAt: now,
		RotatedAt: now,
		Version:   1,
		Status:    KeyStatusActive,
	}

	// 用 KEK 加密 Room Key
	encryptedKey, err := km.encryptRoomKey(keyValue)
	if err != nil {
		return nil, fmt.Errorf("key encryption error")
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
		return nil, fmt.Errorf("key persistence error")
	}

	// 複製密鑰值以返回（安全增強：避免返回內部引用）
	keyValueCopy := make([]byte, len(keyValue))
	copy(keyValueCopy, keyValue)

	// 加載到緩存
	km.keys[roomID] = key

	return keyValueCopy, nil
}

// rotateKey 輪換密鑰（保存到 DB）.
func (km *KeyManagerWithPersistence) rotateKey(roomID string) error {
	km.mu.Lock()
	defer km.mu.Unlock()

	oldKey, exists := km.keys[roomID]
	if !exists {
		return fmt.Errorf("key not found for room %s", roomID)
	}

	// 生成新密鑰
	newKeyValue := make([]byte, 32)
	if _, err := rand.Read(newKeyValue); err != nil {
		return fmt.Errorf("key generation error")
	}

	// 使用完後清零（安全增強）
	defer func() {
		for i := range newKeyValue {
			newKeyValue[i] = 0
		}
	}()

	now := time.Now()
	newVersion := oldKey.Version + 1

	// 為緩存創建獨立的副本（避免被 defer 清零）
	keyValueForCache := make([]byte, len(newKeyValue))
	copy(keyValueForCache, newKeyValue)

	// 創建新密鑰
	newKey := &Key{
		ID:        roomID,
		Value:     keyValueForCache, // 使用副本，不會被清零
		CreatedAt: oldKey.CreatedAt,
		RotatedAt: now,
		Version:   newVersion,
		Status:    KeyStatusActive,
	}

	// 用 KEK 加密新的 Room Key
	encryptedKey, err := km.encryptRoomKey(newKeyValue)
	if err != nil {
		return fmt.Errorf("key encryption error")
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
		return fmt.Errorf("key persistence error")
	}

	// 清零舊密鑰值（安全增強）
	for i := range oldKey.Value {
		oldKey.Value[i] = 0
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

	return nil
}

// encryptRoomKey 用 Tink KEK 加密 Room Key，輸出格式：tink:<base64(ciphertext)>.
func (km *KeyManagerWithPersistence) encryptRoomKey(roomKey []byte) (string, error) {
	ct, err := km.kek.Encrypt(roomKey, nil)
	if err != nil {
		return "", fmt.Errorf("encrypt room key: %w", err)
	}

	return tinkEncryptedPrefix + base64.StdEncoding.EncodeToString(ct), nil
}

// decryptRoomKey 解密 Room Key，支援 tink: 新格式和 gcm: 舊格式（向後相容）.
func (km *KeyManagerWithPersistence) decryptRoomKey(encryptedKey string) ([]byte, error) {
	switch {
	case strings.HasPrefix(encryptedKey, tinkEncryptedPrefix):
		encoded := encryptedKey[len(tinkEncryptedPrefix):]
		ct, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode tink ciphertext: %w", err)
		}

		return km.kek.Decrypt(ct, nil)

	case strings.HasPrefix(encryptedKey, gcmEncryptedPrefix):
		// 向後相容：gcm: 格式（migration 前還存在）
		return km.decryptRoomKeyGCM(encryptedKey[len(gcmEncryptedPrefix):])

	default:
		return nil, fmt.Errorf("unsupported key format")
	}
}

// decryptRoomKeyGCM 使用舊 MASTER_KEY（legacyMasterKey）解密 gcm: 格式的 Room Key.
// 在 migration 完成後，此路徑不再需要。
func (km *KeyManagerWithPersistence) decryptRoomKeyGCM(encoded string) ([]byte, error) {
	if km.legacyMasterKey == nil {
		return nil, fmt.Errorf("legacy master key not configured; cannot decrypt gcm: format")
	}

	// Base64 解碼
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decryption error")
	}

	// 使用完後清零 ciphertext（安全增強）
	defer func() {
		for i := range ciphertext {
			ciphertext[i] = 0
		}
	}()

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("invalid encrypted data")
	}

	block, err := aes.NewCipher(km.legacyMasterKey)
	if err != nil {
		return nil, fmt.Errorf("decryption error")
	}

	// 提取 IV
	iv := ciphertext[:aes.BlockSize]
	encryptedData := ciphertext[aes.BlockSize:]

	// 使用 CTR 模式解密
	// #nosec G407 -- IV is extracted from encrypted data, not hardcoded
	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(encryptedData))
	stream.XORKeyStream(plaintext, encryptedData)

	return plaintext, nil
}

// LoadAllKeys 從數據庫加載所有密鑰（啟動時使用）.
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

// cleanupOldKeys 清理過舊的密鑰.
func (km *KeyManagerWithPersistence) cleanupOldKeys(roomID string) {
	oldKeyList := km.oldKeys[roomID]
	if len(oldKeyList) <= km.rotationPolicy.KeepOldKeys {
		return
	}

	// 只保留最新的 N 個密鑰
	km.oldKeys[roomID] = oldKeyList[len(oldKeyList)-km.rotationPolicy.KeepOldKeys:]
}

// shouldRotateKey 判斷是否需要輪換密鑰.
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

// GetActiveKeyWithVersion 返回指定聊天室的當前密鑰值及版本號.
// 若密鑰不存在則自動創建（與 GetOrCreateRoomKey 行為相同）。
func (km *KeyManagerWithPersistence) GetActiveKeyWithVersion(roomID string) (keyBytes []byte, version int, err error) {
	key, err := km.GetOrCreateRoomKey(roomID)
	if err != nil {
		return nil, 0, err
	}

	km.mu.RLock()
	cachedKey, exists := km.keys[roomID]
	km.mu.RUnlock()

	version = 1
	if exists {
		version = cachedKey.Version
	}

	return key, version, nil
}

// GetKeyByVersion 根據版本號返回指定聊天室的密鑰值.
// 先查緩存（活躍鍵 + 歷史鍵），找不到時從 DB 載入。
func (km *KeyManagerWithPersistence) GetKeyByVersion(roomID string, version int) ([]byte, error) {
	km.mu.RLock()
	// 先查活躍鍵
	if active, ok := km.keys[roomID]; ok && active.Version == version {
		val := make([]byte, len(active.Value))
		copy(val, active.Value)
		km.mu.RUnlock()
		return val, nil
	}
	// 再查歷史鍵緩存
	for _, old := range km.oldKeys[roomID] {
		if old.Version == version {
			val := make([]byte, len(old.Value))
			copy(val, old.Value)
			km.mu.RUnlock()
			return val, nil
		}
	}
	km.mu.RUnlock()

	// 緩存未命中，從 DB 載入
	ctx := context.Background()
	keyDoc, err := km.store.GetKeyByVersion(ctx, roomID, version)
	if err != nil {
		return nil, fmt.Errorf("key version %d not found for room %s: %w", version, roomID, err)
	}

	roomKey, err := km.decryptRoomKey(keyDoc.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt key version %d: %w", version, err)
	}

	return roomKey, nil
}

// GetKeyInfo 獲取密鑰信息（不返回密鑰值）.
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

// SetRotationPolicy 設置密鑰輪換策略.
func (km *KeyManagerWithPersistence) SetRotationPolicy(policy RotationPolicy) {
	km.mu.Lock()
	defer km.mu.Unlock()
	km.rotationPolicy = policy
}

// GetRotationPolicy 返回當前密鑰輪換策略（用於驗證與測試）.
func (km *KeyManagerWithPersistence) GetRotationPolicy() RotationPolicy {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.rotationPolicy
}

// StartAutoRotation 啟動自動密鑰輪換.
// SetRotationPolicy must be called before StartAutoRotation.
// Interval changes after start are ignored until process restart.
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

// StopAutoRotation 停止自動密鑰輪換.
func (km *KeyManagerWithPersistence) StopAutoRotation() {
	km.mu.Lock()
	defer km.mu.Unlock()

	if !km.running {
		return
	}

	close(km.stopChan)
	km.running = false
}

// autoRotationLoop 自動輪換循環.
func (km *KeyManagerWithPersistence) autoRotationLoop() {
	km.mu.RLock()
	interval := km.rotationPolicy.RotationInterval
	km.mu.RUnlock()

	if interval <= 0 {
		interval = 1 * time.Hour
	}

	ticker := time.NewTicker(interval)
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

// checkAndRotateKeys 檢查並輪換需要輪換的密鑰.
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
		if err := km.rotateKey(roomID); err != nil {
			km.logger.Error("key rotation failed", zap.String("room_id", roomID), zap.Error(err))
		} else {
			km.logger.Info("key rotated", zap.String("room_id", roomID))
		}
	}
}

// ForceRotateKey 強制輪換指定聊天室的密鑰.
func (km *KeyManagerWithPersistence) ForceRotateKey(roomID string) error {
	km.mu.RLock()
	_, exists := km.keys[roomID]
	km.mu.RUnlock()

	if !exists {
		return fmt.Errorf("key not found for room %s", roomID)
	}

	return km.rotateKey(roomID)
}

// Stats 獲取統計信息.
func (km *KeyManagerWithPersistence) Stats() KeyManagerStats {
	km.mu.RLock()
	defer km.mu.RUnlock()

	stats := KeyManagerStats{
		TotalKeys:    len(km.keys),
		ArchivedKeys: 0,
		ActiveKeys:   0,
		RevokedKeys:  0,
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

// DecryptGCMLegacy decrypts a base64-encoded AES-CTR ciphertext produced by the
// legacy MASTER_KEY scheme (gcm: prefix format, without the prefix).
// This is exported solely for use by the rekey migration command.
func DecryptGCMLegacy(masterKey []byte, encoded string) ([]byte, error) {
	if len(masterKey) != 32 {
		return nil, fmt.Errorf("master key must be 32 bytes")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}

	defer func() {
		for i := range ciphertext {
			ciphertext[i] = 0
		}
	}()

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	iv := ciphertext[:aes.BlockSize]
	encryptedData := ciphertext[aes.BlockSize:]

	// #nosec G407 -- IV is extracted from encrypted data, not hardcoded
	stream := cipher.NewCTR(block, iv)
	plaintext := make([]byte, len(encryptedData))
	stream.XORKeyStream(plaintext, encryptedData)

	return plaintext, nil
}

// EncryptRoomKeyWithLegacy encrypts a room key using the legacy CTR method
// for testing/migration purposes only.
// This is exported solely for use by the rekey migration command.
func EncryptRoomKeyWithLegacy(masterKey, roomKey []byte) (string, error) {
	if len(masterKey) != 32 {
		return "", fmt.Errorf("master key must be 32 bytes")
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	ciphertext := make([]byte, aes.BlockSize+len(roomKey))
	iv := ciphertext[:aes.BlockSize]

	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", fmt.Errorf("failed to generate IV: %w", err)
	}

	// #nosec G407 -- IV is dynamically generated from crypto/rand above, not hardcoded
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], roomKey)

	return gcmEncryptedPrefix + base64.StdEncoding.EncodeToString(ciphertext), nil
}
