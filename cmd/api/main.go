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
// 從環境變量 MASTER_KEY 讀取 base64 編碼的 32 bytes 密鑰
// 如果未設置，生成臨時隨機密鑰（開發環境）
func loadMasterKey() ([]byte, error) {
	masterKeyEnv := os.Getenv("MASTER_KEY")
	
	if masterKeyEnv != "" {
		// 從環境變量讀取（base64 解碼）
		masterKey, err := base64.StdEncoding.DecodeString(masterKeyEnv)
		if err != nil {
			return nil, fmt.Errorf("invalid MASTER_KEY format (must be base64): %w", err)
		}
		
		// 驗證長度必須是 32 bytes
		if len(masterKey) != 32 {
			return nil, fmt.Errorf("MASTER_KEY must be 32 bytes, got %d bytes", len(masterKey))
		}
		
		fmt.Println("從環境變量載入主密鑰（base64 解碼）")
		return masterKey, nil
	}
	
	// 開發環境：生成臨時隨機密鑰
	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		return nil, fmt.Errorf("failed to generate random master key: %w", err)
	}
	
	fmt.Println("開發模式：使用臨時主密鑰（重啟後舊訊息將無法解密）")
	fmt.Printf("提示：生產環境請設置 MASTER_KEY 環境變量\n")
	fmt.Printf("生成方式：export MASTER_KEY=$(openssl rand -base64 32)\n")
	
	return masterKey, nil
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
	logger.Info(ctx, "配置載入成功")

	// 連接資料庫.
	if err := driver.ConnectMongo(); err != nil {
		return err
	}
	defer func() {
		if err := driver.CloseMongo(); err != nil {
			logger.Errorf(ctx, "關閉 MongoDB 連接失敗: %v", err)
		}
	}()
	logger.Info(ctx, "MongoDB 連接成功")

	// 設置 MongoDB 連接到 database 包
	database.SetMongoDB(driver.GetMongoDatabase())

	// 初始化 Repository.
	repos := database.NewRepositories(config.Get())

	// 獲取安全配置
	cfg := config.Get()
	encryptionEnabled := cfg.Security.Encryption.Enabled
	auditEnabled := cfg.Security.Audit.Enabled

	logger.Infof(ctx, "安全配置 - 加密: %v, 審計: %v", encryptionEnabled, auditEnabled)

	// 初始化密鑰管理器（帶持久化）
	var keyManager *keymanager.KeyManagerWithPersistence
	if encryptionEnabled {
		// 載入主密鑰 (Master Key)
		masterKey, err := loadMasterKey()
		if err != nil {
			return fmt.Errorf("failed to load master key: %w", err)
		}
		
		// 創建帶持久化的密鑰管理器
		mongoDb := driver.GetMongoDatabase()
		if mongoDb == nil {
			return fmt.Errorf("MongoDB database not initialized")
		}
		
		keyManager, err = keymanager.NewKeyManagerWithPersistence(masterKey, mongoDb)
		if err != nil {
			return fmt.Errorf("failed to create key manager: %w", err)
		}
		logger.Info(ctx, "密鑰管理器初始化成功（已啟用持久化）")
		
		// 啟用自動密鑰輪換（可選）
		// 在生產環境中建議啟用
		if os.Getenv("KEY_ROTATION_ENABLED") == "true" {
			keyManager.StartAutoRotation()
			logger.Info(ctx, "自動密鑰輪換已啟用")
		}
	} else {
		logger.Info(ctx, "加密已禁用，訊息將以明文存儲")
	}

	// 啟動 gRPC 服務器
	grpcServer, err := grpc.NewServer(repos, encryptionEnabled, auditEnabled, keyManager, cfg.Security.TLS)
	if err != nil {
		return fmt.Errorf("failed to create gRPC server: %w", err)
	}
	go func() {
		logger.Info(ctx, "正在啟動 gRPC 聊天室服務...", logger.WithAction("start_grpc"))
		if err := grpcServer.Start("8081"); err != nil {
			logger.Errorf(ctx, "gRPC 服務器啟動失敗: %v", err)
		}
	}()

	// 啟動 HTTP 服務器（API 橋樑）
	go func() {
		logger.Info(ctx, "正在啟動 HTTP API 橋樑...", logger.WithAction("start_http"))
		if err := server.Start(repos); err != nil {
			logger.Errorf(ctx, "HTTP 服務器啟動失敗: %v", err)
		}
	}()

	// 等待一下讓服務器啟動
	time.Sleep(2 * time.Second)
	logger.Info(ctx, "服務器啟動完成，等待中斷信號...")

	// 等待中斷信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info(ctx, "正在關閉服務器...", logger.WithAction("shutdown"))
	grpcServer.Stop()

	return nil
}
