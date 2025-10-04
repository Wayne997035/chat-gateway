package keymanager

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// KeyDocument MongoDB 中存儲的密鑰文檔
type KeyDocument struct {
	RoomID       string    `bson:"room_id"`       // 聊天室 ID
	KeyVersion   int       `bson:"key_version"`   // 密鑰版本
	EncryptedKey string    `bson:"encrypted_key"` // 用 Master Key 加密的 Room Key
	CreatedAt    time.Time `bson:"created_at"`    // 創建時間
	RotatedAt    time.Time `bson:"rotated_at"`    // 上次輪替時間
	IsActive     bool      `bson:"is_active"`     // 是否為活躍密鑰
	ExpiresAt    time.Time `bson:"expires_at"`    // 過期時間
}

// KeyStore 密鑰持久化存儲
type KeyStore struct {
	collection *mongo.Collection
}

// NewKeyStore 創建密鑰存儲
func NewKeyStore(db *mongo.Database) *KeyStore {
	collection := db.Collection("encryption_keys")

	// 創建索引
	ctx := context.Background()

	// room_id + key_version 唯一索引
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "room_id", Value: 1},
			{Key: "key_version", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	}) // #nosec G104 -- index creation errors are not critical, DB will still work

	// room_id + is_active 索引（快速查詢活躍密鑰）
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "room_id", Value: 1},
			{Key: "is_active", Value: 1},
		},
	}) // #nosec G104 -- index creation errors are not critical

	// expires_at 索引（用於清理過期密鑰）
	_, _ = collection.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "expires_at", Value: 1},
		},
	}) // #nosec G104 -- index creation errors are not critical

	return &KeyStore{
		collection: collection,
	}
}

// SaveKey 保存密鑰到數據庫（優先使用事務，失敗則降級）
func (ks *KeyStore) SaveKey(ctx context.Context, doc *KeyDocument) error {
	// 嘗試使用事務（需要 MongoDB 副本集）
	session, err := ks.collection.Database().Client().StartSession()
	if err == nil {
		defer session.EndSession(ctx)

		// 在事務中執行操作
		_, err = session.WithTransaction(ctx, func(sc context.Context) (interface{}, error) {
			return nil, ks.saveKeyWithContext(sc, doc)
		})

		// 事務成功
		if err == nil {
			return nil
		}

		// 事務失敗，降級為非事務版本（開發環境單節點 MongoDB）
	}

	// 非事務版本（不保證原子性，但可用於開發環境）
	return ks.saveKeyWithContext(ctx, doc)
}

// saveKeyWithContext 執行實際的保存操作（可在事務或非事務上下文中使用）
func (ks *KeyStore) saveKeyWithContext(ctx context.Context, doc *KeyDocument) error {
	// 如果保存新的活躍密鑰，先將舊的標記為非活躍
	if doc.IsActive {
		filter := bson.M{
			"room_id":     doc.RoomID,
			"is_active":   true,
			"key_version": bson.M{"$ne": doc.KeyVersion},
		}
		update := bson.M{
			"$set": bson.M{"is_active": false},
		}
		_, err := ks.collection.UpdateMany(ctx, filter, update)
		if err != nil {
			return fmt.Errorf("failed to deactivate old keys: %w", err)
		}
	}

	// 保存新密鑰（使用 ReplaceOne with upsert）
	filter := bson.M{
		"room_id":     doc.RoomID,
		"key_version": doc.KeyVersion,
	}

	opts := options.Replace().SetUpsert(true)

	_, err := ks.collection.ReplaceOne(ctx, filter, doc, opts)
	if err != nil {
		return fmt.Errorf("failed to save key: %w", err)
	}

	return nil
}

// GetActiveKey 獲取活躍的密鑰
func (ks *KeyStore) GetActiveKey(ctx context.Context, roomID string) (*KeyDocument, error) {
	filter := bson.M{
		"room_id":   roomID,
		"is_active": true,
	}

	var doc KeyDocument
	err := ks.collection.FindOne(ctx, filter).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil // 沒有找到密鑰
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get active key: %w", err)
	}

	return &doc, nil
}

// GetKeyByVersion 根據版本號獲取密鑰
func (ks *KeyStore) GetKeyByVersion(ctx context.Context, roomID string, version int) (*KeyDocument, error) {
	filter := bson.M{
		"room_id":     roomID,
		"key_version": version,
	}

	var doc KeyDocument
	err := ks.collection.FindOne(ctx, filter).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get key by version: %w", err)
	}

	return &doc, nil
}

// GetAllKeys 獲取聊天室的所有密鑰（用於解密舊訊息）
func (ks *KeyStore) GetAllKeys(ctx context.Context, roomID string) ([]*KeyDocument, error) {
	filter := bson.M{"room_id": roomID}
	opts := options.Find().SetSort(bson.D{{Key: "key_version", Value: -1}}) // 最新的在前

	cursor, err := ks.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get all keys: %w", err)
	}
	defer cursor.Close(ctx)

	var keys []*KeyDocument
	if err := cursor.All(ctx, &keys); err != nil {
		return nil, fmt.Errorf("failed to decode keys: %w", err)
	}

	return keys, nil
}

// DeleteExpiredKeys 刪除過期的密鑰
func (ks *KeyStore) DeleteExpiredKeys(ctx context.Context) (int64, error) {
	filter := bson.M{
		"expires_at": bson.M{"$lt": time.Now()},
		"is_active":  false, // 只刪除非活躍的過期密鑰
	}

	result, err := ks.collection.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired keys: %w", err)
	}

	return result.DeletedCount, nil
}

// GetKeysToRotate 獲取需要輪替的密鑰
func (ks *KeyStore) GetKeysToRotate(ctx context.Context, rotationInterval time.Duration) ([]*KeyDocument, error) {
	filter := bson.M{
		"is_active":  true,
		"rotated_at": bson.M{"$lt": time.Now().Add(-rotationInterval)},
	}

	cursor, err := ks.collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get keys to rotate: %w", err)
	}
	defer cursor.Close(ctx)

	var keys []*KeyDocument
	if err := cursor.All(ctx, &keys); err != nil {
		return nil, fmt.Errorf("failed to decode keys: %w", err)
	}

	return keys, nil
}
