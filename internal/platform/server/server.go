package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/driver"
	"chat-gateway/internal/platform/logger"
	"chat-gateway/internal/storage/database"
)

// Start 啟動伺服器.
func Start(repos *database.Repositories) error {
	// 初始化日誌系統
	if err := logger.InitLogger(); err != nil {
		return err
	}
	defer logger.CloseLogger()

	logger.LogInfof("正在啟動 ChatGateway API 伺服器...")

	// 載入設定
	if err := config.Load(); err != nil {
		logger.LogErrorf("載入設定失敗: %v", err)
		return err
	}

	cfg := config.Get()
	logger.LogInfof("設定載入成功，環境: %s", config.GetEnv())

	// connect db
	if err := driver.ConnectMongo(); err != nil {
		logger.LogErrorf("資料庫連接失敗: %v", err)
		return err
	}
	defer func() {
		if err := driver.CloseMongo(); err != nil {
			logger.LogErrorf("關閉 MongoDB 連接失敗: %v", err)
		}
	}()

	logger.LogInfof("儲存庫集合初始化完成")

	// setting router
	router := Router()

	// create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  time.Duration(cfg.Server.Timeout) * time.Second,
		WriteTimeout: 0, // SSE 需要長連接，設為 0 表示不超時
		IdleTimeout:  120 * time.Second,
	}

	// start server
	go func() {
		logger.LogInfof("伺服器正在監聽埠口: %s", cfg.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.LogErrorf("伺服器啟動失敗: %v", err)
			os.Exit(1)
		}
	}()

	// 等待關閉信號
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.LogInfof("收到關閉信號，正在優雅關閉伺服器...")

	// 優雅關閉
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.LogErrorf("伺服器關閉失敗: %v", err)
		return err
	}

	logger.LogInfof("伺服器已優雅關閉")
	return nil
}
