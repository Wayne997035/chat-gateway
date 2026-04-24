package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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

// loadMasterKey 載入主密鑰
// 優先順序：MASTER_KEY env var → config security.encryption.master_key → 臨時隨機密鑰（開發環境）
func loadMasterKey() ([]byte, error) {
	ctx := context.Background()

	raw := os.Getenv("MASTER_KEY")
	source := "MASTER_KEY environment variable"

	if raw == "" {
		raw = config.Get().Security.Encryption.MasterKey
		source = "config security.encryption.master_key"
	}

	if raw != "" {
		masterKey, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			logger.Error(ctx, "Master Key 格式錯誤", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			return nil, fmt.Errorf("invalid master key configuration")
		}

		if len(masterKey) != 32 {
			logger.Error(ctx, "Master Key 長度錯誤", logger.WithDetails(map[string]interface{}{"expected": 32, "got": len(masterKey)}))
			return nil, fmt.Errorf("invalid master key configuration")
		}

		masked := fmt.Sprintf("%x****", masterKey[:2])
		logger.Info(ctx, "[SUCCESS] 成功載入主密鑰", logger.WithDetails(map[string]interface{}{
			"masked": masked,
			"length": len(masterKey),
			"source": source,
		}))
		return masterKey, nil
	}

	// 開發環境：生成臨時隨機密鑰（重啟後舊訊息將無法解密）
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		logger.Error(ctx, "無法生成隨機密鑰", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
		return nil, fmt.Errorf("master key initialization failed")
	}

	masked := fmt.Sprintf("%x****", masterKey[:2])
	logger.Info(ctx, "[WARNING] 開發模式：使用臨時主密鑰（重啟後舊訊息將無法解密）", logger.WithDetails(map[string]interface{}{
		"masked": masked,
		"source": "randomly generated",
	}))
	logger.Info(ctx, "[WARNING] 提示：生產環境請設置 MASTER_KEY 環境變量")
	logger.Info(ctx, "生成方式：export MASTER_KEY=$(openssl rand -base64 32)")

	return masterKey, nil
}

// applyKeyRotationPolicy 從 config 讀取密鑰輪換策略並套用到密鑰管理器.
func applyKeyRotationPolicy(ctx context.Context, km *keymanager.KeyManagerWithPersistence, cfg *config.Config) {
	kr := cfg.Security.KeyRotation
	if !kr.Enabled {
		return
	}

	rotationIntervalHours := kr.RotationIntervalHours
	if rotationIntervalHours <= 0 {
		rotationIntervalHours = 24
	}
	maxKeyAgeDays := kr.MaxKeyAgeDays
	if maxKeyAgeDays <= 0 {
		maxKeyAgeDays = 30
	}
	keepOldKeys := kr.KeepOldKeys
	if keepOldKeys <= 0 {
		keepOldKeys = 5
	}

	km.SetRotationPolicy(keymanager.RotationPolicy{
		Enabled:          true,
		RotationInterval: time.Duration(rotationIntervalHours) * time.Hour,
		MaxKeyAge:        time.Duration(maxKeyAgeDays) * 24 * time.Hour,
		KeepOldKeys:      keepOldKeys,
	})
	km.StartAutoRotation()
	logger.Info(ctx, "[KeyManager] 自動密鑰輪換已啟用（由配置驅動）", logger.WithDetails(map[string]interface{}{
		"rotation_interval_hours": rotationIntervalHours,
		"max_key_age_days":        maxKeyAgeDays,
		"keep_old_keys":           keepOldKeys,
	}))
}

// mainNoExit 分離主要邏輯以避免 exitAfterDefer 問題，確保 defer 函數正常執行.
func mainNoExit() error {
	// 初始化日誌.
	if err := logger.InitLogger(); err != nil {
		return err
	}
	defer logger.CloseLogger()

	ctx := context.Background()

	// 載入配置.
	if err := config.Load(); err != nil {
		return err
	}
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
	cfg := config.Get()
	encryptionEnabled := cfg.Security.Encryption.Enabled
	auditEnabled := cfg.Security.Audit.Enabled

	// 初始化密鑰管理器（帶持久化）
	var keyManager *keymanager.KeyManagerWithPersistence
	if encryptionEnabled {
		// 載入主密鑰 (Master Key)
		masterKey, err := loadMasterKey()
		if err != nil {
			logger.Error(ctx, "無法載入主密鑰", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			return fmt.Errorf("encryption initialization failed")
		}

		// 創建帶持久化的密鑰管理器
		mongoDB := driver.GetMongoDatabase()
		if mongoDB == nil {
			logger.Error(ctx, "MongoDB 未初始化", nil)
			return fmt.Errorf("database initialization failed")
		}

		keyManager, err = keymanager.NewKeyManagerWithPersistence(masterKey, mongoDB)
		if err != nil {
			logger.Error(ctx, "密鑰管理器創建失敗", logger.WithDetails(map[string]interface{}{"error": err.Error()}))
			return fmt.Errorf("encryption initialization failed")
		}

		// 從配置讀取密鑰輪換策略
		applyKeyRotationPolicy(ctx, keyManager, cfg)

		// 確保 shutdown 時停止自動輪換 goroutine
		if cfg.Security.KeyRotation.Enabled {
			defer keyManager.StopAutoRotation()
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

	// 建立密鑰輪換 HTTP handler（encryption 未啟用或 ADMIN_TOKEN 未設定時為 nil，router 會略過）
	keyRotationHandler := buildKeyRotationHandler(keyManager, cfg.Security.AdminToken, encryptionEnabled)

	// 啟動 HTTP 服務器（API 橋樑）
	go func() {
		if err := server.Start(repos, keyRotationHandler); err != nil {
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

// buildKeyRotationHandler 建立密鑰輪換 handler；條件未滿足時回傳 nil（router 略過）.
func buildKeyRotationHandler(
	km *keymanager.KeyManagerWithPersistence, adminToken string, encryptionEnabled bool,
) *keymanager.KeyRotationHandler {
	if !encryptionEnabled || km == nil || adminToken == "" {
		return nil
	}
	return keymanager.NewKeyRotationHandler(km, adminToken)
}
