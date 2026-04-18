package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/driver"
	"chat-gateway/internal/security/encryption"
	"chat-gateway/internal/security/keymanager"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	ctrPrefix       = "aes256ctr:"
	plaintextPrefix = "plaintext:"
	gcmPrefix       = "aes256gcm:"
)

// migrateResult 統計遷移結果.
type migrateResult struct {
	total     int
	processed int
	failed    int
	skipped   int
}

// migratableMessage 從 MongoDB 查詢出的最小欄位結構.
type migratableMessage struct {
	ID      bson.ObjectID `bson:"_id"`
	Content string        `bson:"content"`
	RoomID  string        `bson:"room_id"`
}

// decryptCTR 解密 AES-256-CTR 格式密文.
// #nosec G407 -- IV is extracted from stored ciphertext, not hardcoded
func decryptCTR(encryptedText string, key []byte) (string, error) {
	prefix := ctrPrefix
	if len(encryptedText) < len(prefix) || encryptedText[:len(prefix)] != prefix {
		return "", fmt.Errorf("invalid CTR ciphertext format")
	}

	data, err := base64.StdEncoding.DecodeString(encryptedText[len(prefix):])
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher failed: %w", err)
	}

	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]
	plaintext := make([]byte, len(ciphertext))

	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

// loadMasterKey 從環境變量 MASTER_KEY 讀取 base64 編碼的 32 bytes 密鑰.
// Migration 工具必須提供明確的 MASTER_KEY，不允許使用隨機密鑰.
func loadMasterKey() ([]byte, error) {
	masterKeyEnv := os.Getenv("MASTER_KEY")
	if masterKeyEnv == "" {
		return nil, fmt.Errorf("MASTER_KEY environment variable is required for migration")
	}

	masterKey, err := base64.StdEncoding.DecodeString(masterKeyEnv)
	if err != nil {
		return nil, fmt.Errorf("MASTER_KEY base64 decode failed: %w", err)
	}

	if len(masterKey) != 32 {
		return nil, fmt.Errorf("MASTER_KEY must be 32 bytes (256 bits), got %d bytes", len(masterKey))
	}

	return masterKey, nil
}

// migrateBatch 處理單一批次的遷移.
func migrateBatch(
	ctx context.Context,
	collection *mongo.Collection,
	messages []migratableMessage,
	km *keymanager.KeyManagerWithPersistence,
	dryRun bool,
	result *migrateResult,
) {
	for _, msg := range messages {
		content := msg.Content

		// 已經是 GCM 格式，跳過.
		if len(content) >= len(gcmPrefix) && content[:len(gcmPrefix)] == gcmPrefix {
			result.skipped++
			continue
		}

		// 取得 room key.
		roomKey, err := km.GetOrCreateRoomKey(msg.RoomID)
		if err != nil {
			slog.Error("[Migration] failed to get room key",
				"id", msg.ID.Hex(),
				"room_id", msg.RoomID,
				"error", err,
			)
			result.failed++
			continue
		}

		// 解密.
		var plaintext string

		switch {
		case len(content) >= len(ctrPrefix) && content[:len(ctrPrefix)] == ctrPrefix:
			plaintext, err = decryptCTR(content, roomKey)
			if err != nil {
				slog.Error("[Migration] CTR decrypt failed",
					"id", msg.ID.Hex(),
					"error", err,
				)
				result.failed++
				continue
			}

		case len(content) >= len(plaintextPrefix) && content[:len(plaintextPrefix)] == plaintextPrefix:
			plaintext = content[len(plaintextPrefix):]

		default:
			slog.Warn("[Migration] unrecognized content format, skipping",
				"id", msg.ID.Hex(),
				"content_len", len(content),
			)
			result.skipped++
			continue
		}

		// GCM 重新加密.
		gcmEnc, err := encryption.NewAESGCMEncryption(roomKey)
		if err != nil {
			slog.Error("[Migration] failed to create GCM encryptor",
				"id", msg.ID.Hex(),
				"error", err,
			)
			result.failed++
			continue
		}

		newContent, err := gcmEnc.Encrypt(plaintext)
		if err != nil {
			slog.Error("[Migration] GCM encrypt failed",
				"id", msg.ID.Hex(),
				"error", err,
			)
			result.failed++
			continue
		}

		if dryRun {
			slog.Info("[Migration] [DRY-RUN] would update",
				"id", msg.ID.Hex(),
				"room_id", msg.RoomID,
			)
			result.processed++
			continue
		}

		// 更新 DB.
		filter := bson.M{"_id": msg.ID}
		update := bson.M{
			"$set": bson.M{
				"content":    newContent,
				"updated_at": time.Now().UTC(),
			},
		}

		if _, err = collection.UpdateOne(ctx, filter, update); err != nil {
			slog.Error("[Migration] DB update failed",
				"id", msg.ID.Hex(),
				"error", err,
			)
			result.failed++
			continue
		}

		result.processed++
	}
}

// runMigration 執行遷移主邏輯.
func runMigration(
	ctx context.Context,
	db *mongo.Database,
	km *keymanager.KeyManagerWithPersistence,
	batchSize int,
	dryRun bool,
) error {
	collection := db.Collection("messages")

	// 查詢需要遷移的訊息.
	filter := bson.M{
		"$or": bson.A{
			bson.M{"content": bson.M{"$regex": "^aes256ctr:"}},
			bson.M{"content": bson.M{"$regex": "^plaintext:"}},
		},
	}

	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return fmt.Errorf("count documents failed: %w", err)
	}

	slog.Info("[Migration] Found messages to migrate", "total", total)

	if total == 0 {
		slog.Info("[Migration] Complete: nothing to migrate")
		return nil
	}

	result := &migrateResult{total: int(total)}

	// Clamp batchSize to int32 range; flag parsing ensures positive value, max int32 is 2^31-1.
	if batchSize > int(^int32(0)) {
		batchSize = int(^int32(0))
	}

	opts := options.Find().
		SetBatchSize(int32(batchSize)). // #nosec G115 -- range guarded above
		SetProjection(bson.M{
			"_id":     1,
			"content": 1,
			"room_id": 1,
		})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return fmt.Errorf("find documents failed: %w", err)
	}
	defer cursor.Close(ctx)

	batch := make([]migratableMessage, 0, batchSize)

	for cursor.Next(ctx) {
		var msg migratableMessage
		if err := cursor.Decode(&msg); err != nil {
			slog.Error("[Migration] decode failed", "error", err)
			result.failed++
			continue
		}

		batch = append(batch, msg)

		if len(batch) >= batchSize {
			migrateBatch(ctx, collection, batch, km, dryRun, result)
			slog.Info("[Migration] Progress",
				"processed", result.processed+result.failed+result.skipped,
				"total", result.total,
				"failed", result.failed,
			)
			batch = batch[:0]
		}
	}

	// 處理最後一批.
	if len(batch) > 0 {
		migrateBatch(ctx, collection, batch, km, dryRun, result)
	}

	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %w", err)
	}

	slog.Info("[Migration] Complete",
		"migrated", result.processed,
		"failed", result.failed,
		"skipped", result.skipped,
	)

	if result.failed > 0 {
		return fmt.Errorf("migration completed with %d failures", result.failed)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("[Migration] fatal error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	envFlag := flag.String("env", "local", "config environment (corresponds to configs/{env}.yaml)")
	dryRun := flag.Bool("dry-run", false, "dry-run mode: print plan without writing to DB")
	batchSize := flag.Int("batch-size", 100, "number of messages to process per batch")
	timeoutMinutes := flag.Int("timeout", 30, "migration timeout in minutes (0 = no timeout)")
	confirm := flag.Bool("confirm", false, "confirm execution; required when not using --dry-run")
	flag.Parse()

	if *batchSize <= 0 {
		return fmt.Errorf("--batch-size must be greater than 0, got %d", *batchSize)
	}

	slog.Info("[Migration] Starting...",
		"env", *envFlag,
		"dry_run", *dryRun,
		"batch_size", *batchSize,
		"timeout_minutes", *timeoutMinutes,
	)

	// 設定 APP_ENV 使 config.Load() 讀取正確的配置文件.
	if err := os.Setenv("APP_ENV", *envFlag); err != nil {
		return fmt.Errorf("set APP_ENV failed: %w", err)
	}
	config.SetEnv(*envFlag)

	// 載入配置.
	if err := config.Load(); err != nil {
		return fmt.Errorf("config load failed: %w", err)
	}

	// 載入 MASTER_KEY（migration 必須有正確的 key，不允許隨機 key）.
	masterKey, err := loadMasterKey()
	if err != nil {
		return err
	}
	defer func() {
		for i := range masterKey {
			masterKey[i] = 0
		}
	}()

	// 連接 MongoDB.
	if err := driver.ConnectMongo(); err != nil {
		return fmt.Errorf("MongoDB connection failed: %w", err)
	}

	db := driver.GetMongoDatabase()
	if db == nil {
		return fmt.Errorf("MongoDB database is nil after connection")
	}

	// 建立有 timeout 的 context.
	var ctx context.Context
	var cancel context.CancelFunc
	if *timeoutMinutes > 0 {
		ctx, cancel = context.WithTimeout(context.Background(), time.Duration(*timeoutMinutes)*time.Minute)
	} else {
		ctx, cancel = context.WithCancel(context.Background())
	}
	defer cancel()

	// 初始化 KeyManager.
	km, err := keymanager.NewKeyManagerWithPersistence(masterKey, db)
	if err != nil {
		return fmt.Errorf("key manager init failed: %w", err)
	}

	// 若非 dry-run，需要 --confirm=true 才執行.
	if !*dryRun && !*confirm {
		collection := db.Collection("messages")
		filter := bson.M{
			"$or": bson.A{
				bson.M{"content": bson.M{"$regex": "^aes256ctr:"}},
				bson.M{"content": bson.M{"$regex": "^plaintext:"}},
			},
		}
		total, countErr := collection.CountDocuments(ctx, filter)
		if countErr != nil {
			return fmt.Errorf("count documents failed: %w", countErr)
		}
		slog.Info(fmt.Sprintf("Found %d messages to migrate. Re-run with --confirm=true to execute.", total))
		return nil
	}

	return runMigration(ctx, db, km, *batchSize, *dryRun)
}
