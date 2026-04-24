// Package main provides database migration utilities for chat-gateway.
//
// Usage:
//
//	MASTER_KEY=xxx CRYPTO_KEK_KEYSET=yyy APP_ENV=development go run ./cmd/migrate rekey
package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	kekpkg "chat-gateway/internal/crypto/kek"
	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/driver"
	"chat-gateway/internal/platform/logger"
	"chat-gateway/internal/security/keymanager"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate <subcommand>\nsubcommands: rekey")
	}

	switch os.Args[1] {
	case "rekey":
		return cmdRekey()
	default:
		return fmt.Errorf("unknown subcommand %q; available: rekey", os.Args[1])
	}
}

// cmdRekey re-encrypts all gcm: format room keys in MongoDB to tink: format.
//
// The operation is idempotent: records already in tink: format are skipped.
//
// Required env vars:
//   - MASTER_KEY           — base64 32-byte legacy key for decrypting gcm: records
//   - CRYPTO_KEK_KEYSET    — base64 Tink JSON keyset for re-encrypting as tink:
//   - APP_ENV / CONFIG_PATH — selects config file (defaults to local)
func cmdRekey() error {
	ctx := context.Background()

	logger.Init("info", false)

	if err := config.Load(); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfg := config.Get()

	// --- Load legacy MASTER_KEY ---
	masterKeyEnv := os.Getenv("MASTER_KEY")
	if masterKeyEnv == "" {
		masterKeyEnv = cfg.Security.Encryption.MasterKey
	}
	if masterKeyEnv == "" {
		return fmt.Errorf("MASTER_KEY env var is required for rekey (needed to decrypt gcm: records)")
	}

	masterKeyBytes, err := base64.StdEncoding.DecodeString(masterKeyEnv)
	if err != nil {
		return fmt.Errorf("decode MASTER_KEY: %w", err)
	}
	if len(masterKeyBytes) != 32 {
		return fmt.Errorf("MASTER_KEY must decode to exactly 32 bytes, got %d", len(masterKeyBytes))
	}

	// --- Load Tink KEK ---
	kekStr := os.Getenv("CRYPTO_KEK_KEYSET")
	if kekStr == "" {
		kekStr = cfg.Security.Encryption.KEKKeyset
	}
	if kekStr == "" {
		return fmt.Errorf("CRYPTO_KEK_KEYSET env var is required for rekey")
	}

	provider := kekpkg.EnvVarProvider{KeysetBase64: kekStr}

	kekAEAD, err := provider.AEAD()
	if err != nil {
		return fmt.Errorf("load KEK: %w", err)
	}

	// --- Connect to MongoDB ---
	if err = driver.ConnectMongo(); err != nil {
		return fmt.Errorf("connect mongo: %w", err)
	}
	defer func() {
		if closeErr := driver.CloseMongo(); closeErr != nil {
			logger.Errorf(ctx, "close mongo: %v", closeErr)
		}
	}()

	db := driver.GetMongoDatabase()
	if db == nil {
		return fmt.Errorf("mongodb not initialized")
	}

	return rekeyCollection(ctx, db, masterKeyBytes, kekAEAD)
}

// rekeyCollection iterates the room_keys collection and re-encrypts gcm: records.
func rekeyCollection(ctx context.Context, db *mongo.Database, masterKey []byte, kekAEAD interface {
	Encrypt([]byte, []byte) ([]byte, error)
}) error {
	coll := db.Collection("room_keys")

	// Find all documents where encrypted_key starts with "gcm:"
	filter := bson.M{"encrypted_key": bson.M{"$regex": "^gcm:"}}
	cursor, err := coll.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("find gcm records: %w", err)
	}
	defer func() {
		if closeErr := cursor.Close(ctx); closeErr != nil {
			fmt.Fprintf(os.Stderr, "close cursor: %v\n", closeErr)
		}
	}()

	var processed, skipped, failed int

	for cursor.Next(ctx) {
		var doc struct {
			ID           interface{} `bson:"_id"`
			EncryptedKey string      `bson:"encrypted_key"`
		}

		if err = cursor.Decode(&doc); err != nil {
			fmt.Fprintf(os.Stderr, "decode document: %v\n", err)
			failed++
			continue
		}

		// Skip if already tink: format (idempotent)
		if strings.HasPrefix(doc.EncryptedKey, "tink:") {
			skipped++
			continue
		}

		if !strings.HasPrefix(doc.EncryptedKey, "gcm:") {
			fmt.Fprintf(os.Stderr, "skipping document with unsupported format: %v\n", doc.ID)
			skipped++
			continue
		}

		// Decrypt with legacy MASTER_KEY
		roomKey, decErr := decryptGCM(masterKey, doc.EncryptedKey[len("gcm:"):])
		if decErr != nil {
			fmt.Fprintf(os.Stderr, "decrypt gcm doc %v: %v\n", doc.ID, decErr)
			failed++
			continue
		}

		// Re-encrypt with Tink KEK
		ct, encErr := kekAEAD.Encrypt(roomKey, nil)
		if encErr != nil {
			fmt.Fprintf(os.Stderr, "tink encrypt doc %v: %v\n", doc.ID, encErr)
			failed++
			continue
		}

		newEncryptedKey := "tink:" + base64.StdEncoding.EncodeToString(ct)

		// Write back to MongoDB
		updateFilter := bson.M{"_id": doc.ID}
		update := bson.M{"$set": bson.M{
			"encrypted_key": newEncryptedKey,
			"updated_at":    time.Now(),
		}}

		_, updateErr := coll.UpdateOne(ctx, updateFilter, update, options.UpdateOne())
		if updateErr != nil {
			fmt.Fprintf(os.Stderr, "update doc %v: %v\n", doc.ID, updateErr)
			failed++
			continue
		}

		processed++
	}

	if err = cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %w", err)
	}

	fmt.Printf("rekey complete — processed: %d, skipped: %d, failed: %d\n", processed, skipped, failed)

	if failed > 0 {
		return fmt.Errorf("%d documents failed to rekey; check stderr for details", failed)
	}

	return nil
}

// decryptGCM decrypts a base64-encoded AES-CTR ciphertext using the legacy masterKey.
func decryptGCM(masterKey []byte, encoded string) ([]byte, error) {
	return keymanager.DecryptGCMLegacy(masterKey, encoded)
}
