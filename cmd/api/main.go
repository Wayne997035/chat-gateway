package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tink-crypto/tink-go/v2/tink"

	"chat-gateway/internal/crypto/kek"
	"chat-gateway/internal/grpc"
	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/driver"
	"chat-gateway/internal/platform/logger"
	"chat-gateway/internal/platform/server"
	"chat-gateway/internal/security/keymanager"
	"chat-gateway/internal/storage/database"
)

func main() {
	if err := mainNoExit(); err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %v\n", err)
		os.Exit(1)
	}
}

// loadKEKKeyset 載入 Tink KEK keyset 以及（可選的）舊 MASTER_KEY.
// 從 config 讀取 kek_keyset（base64 Tink JSON），建立 tink.AEAD。
// 若同時設定了 master_key，也一併回傳 legacyKey 供向後相容解密 gcm: 格式。
func loadKEKKeyset(ctx context.Context, cfg *config.Config) (tink.AEAD, []byte, error) {
	keysetStr := cfg.Security.Encryption.KEKKeyset
	if keysetStr == "" {
		logger.Error(ctx, "CRYPTO_KEK_KEYSET 未設定", nil)
		return nil, nil, fmt.Errorf("CRYPTO_KEK_KEYSET not set; set the env var with a base64 Tink AES256-GCM keyset")
	}

	provider := kek.EnvVarProvider{KeysetBase64: keysetStr}

	aeadPrimitive, err := provider.AEAD()
	if err != nil {
		logger.Error(ctx, "載入 KEK keyset 失敗", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return nil, nil, fmt.Errorf("load KEK: %w", err)
	}

	logger.Info(ctx, "[SUCCESS] 成功載入 Tink KEK keyset", nil)

	// 嘗試讀取舊的 MASTER_KEY（供 gcm: 向後相容解密）
	var legacyKey []byte
	if mk := cfg.Security.Encryption.MasterKey; mk != "" {
		decoded, decErr := base64.StdEncoding.DecodeString(mk)
		if decErr == nil && len(decoded) == 32 {
			legacyKey = decoded
			logger.Info(ctx, "[INFO] 載入 legacy MASTER_KEY 供 gcm: 格式向後相容", nil)
		} else {
			logger.Info(ctx, "[WARNING] MASTER_KEY 格式不正確，略過 gcm: fallback", nil)
		}
	}

	return aeadPrimitive, legacyKey, nil
}

// mainNoExit 分離主要邏輯以避免 exitAfterDefer 問題，確保 defer 函數正常執行.
func mainNoExit() error {
	// Bootstrap logger with sane defaults before config is loaded.
	logger.Init("info", true)

	ctx := context.Background()

	// 載入配置.
	if err := config.Load(); err != nil {
		return err
	}

	// Re-initialize logger with values from config.
	cfg := config.Get()
	isDev := cfg.App.Debug
	logger.Init(cfg.Log.Level, isDev)

	// 連接資料庫.
	if err := driver.ConnectMongo(); err != nil {
		return err
	}
	defer func() {
		if err := driver.CloseMongo(); err != nil {
			logger.Errorf(ctx, "關閉 MongoDB 連接失敗: %v", err)
		}
	}()

	// 設置 MongoDB 連接到 database 包
	database.SetMongoDB(driver.GetMongoDatabase())

	// 初始化 Repository.
	repos := database.NewRepositories(config.Get())

	// 獲取安全配置
	encryptionEnabled := cfg.Security.Encryption.Enabled
	auditEnabled := cfg.Security.Audit.Enabled

	// 初始化密鑰管理器（帶持久化）
	var keyManager *keymanager.KeyManagerWithPersistence
	if encryptionEnabled {
		// 載入 Tink KEK keyset
		kekAEAD, legacyKey, err := loadKEKKeyset(ctx, cfg)
		if err != nil {
			logger.Error(ctx, "無法載入 KEK keyset", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			return fmt.Errorf("encryption initialization failed")
		}

		// 創建帶持久化的密鑰管理器
		mongoDB := driver.GetMongoDatabase()
		if mongoDB == nil {
			logger.Error(ctx, "MongoDB 未初始化", nil)
			return fmt.Errorf("database initialization failed")
		}

		keyManager, err = keymanager.NewKeyManagerWithPersistence(kekAEAD, legacyKey, mongoDB)
		if err != nil {
			logger.Error(ctx, "密鑰管理器創建失敗", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			return fmt.Errorf("encryption initialization failed")
		}

		// 啟用自動密鑰輪換（可選）
		if os.Getenv("KEY_ROTATION_ENABLED") == "true" {
			keyManager.StartAutoRotation()
			logger.Info(ctx, "[KeyManager] 自動密鑰輪換已啟用")
		}
	}

	// 啟動 gRPC 服務器
	grpcServer, err := grpc.NewServer(repos, encryptionEnabled, auditEnabled, keyManager, cfg.Security.TLS)
	if err != nil {
		logger.Error(ctx, "gRPC 服務器創建失敗", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return fmt.Errorf("server initialization failed")
	}
	go func() {
		if err := grpcServer.Start("8081"); err != nil {
			logger.Errorf(ctx, "gRPC 服務器啟動失敗: %v", err)
		}
	}()

	// 啟動 HTTP 服務器（API 橋樑）
	go func() {
		if err := server.Start(repos); err != nil {
			logger.Errorf(ctx, "HTTP 服務器啟動失敗: %v", err)
		}
	}()

	// 等待一下讓服務器啟動
	time.Sleep(2 * time.Second)
	logger.Info(ctx, "[System] 服務器啟動完成")

	// 等待中斷信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "正在關閉服務器...", logger.WithAction("shutdown"))
	grpcServer.Stop()

	return nil
}
