package driver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"regexp"
	"time"

	"chat-gateway/internal/platform/config"
	"chat-gateway/internal/platform/logger"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var mongoClient *mongo.Client
var mongoDB *mongo.Database

// ConnectMongo 連接 MongoDB.
func ConnectMongo() error {
	cfg := config.Get()
	if cfg == nil {
		return fmt.Errorf("配置未載入")
	}

	return InitMongo(&cfg.Database.Mongo)
}

// InitMongo 初始化 MongoDB 連接.
func InitMongo(cfg *config.MongoConfig) error {
	logger.LogInfof("[MongoDB] 連接中... %s", maskPassword(cfg.URL))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.ConnectTimeout)*time.Second)
	defer cancel()

	// 設置連接選項
	clientOptions := options.Client().ApplyURI(cfg.URL)

	// 如果有認證信息，設置認證
	if cfg.Username != "" && cfg.Password != "" {
		credential := options.Credential{
			Username: cfg.Username,
			Password: cfg.Password,
		}
		clientOptions.SetAuth(credential)
	}

	// 如果啟用 TLS，配置 TLS
	if cfg.TLSEnabled {
		tlsConfig, err := loadMongoTLSConfig(cfg)
		if err != nil {
			return fmt.Errorf("failed to load MongoDB TLS config: %w", err)
		}
		clientOptions.SetTLSConfig(tlsConfig)
	}

	clientOptions.SetMaxPoolSize(cfg.MaxPoolSize)
	clientOptions.SetMinPoolSize(cfg.MinPoolSize)
	clientOptions.SetMaxConnIdleTime(time.Duration(cfg.MaxConnIdleTime) * time.Second)
	clientOptions.SetServerSelectionTimeout(time.Duration(cfg.ServerSelectionTimeout) * time.Second)

	// 連接到 MongoDB
	client, err := mongo.Connect(clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// 測試連接
	if err := client.Ping(ctx, nil); err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	mongoClient = client
	mongoDB = client.Database(cfg.Database)

	logger.LogInfof("[MongoDB] 連接成功")
	return nil
}

// GetMongoDatabase 獲取 MongoDB 數據庫實例.
func GetMongoDatabase() *mongo.Database {
	return mongoDB
}

// GetMongoClient 獲取 MongoDB 客戶端實例.
func GetMongoClient() *mongo.Client {
	return mongoClient
}

// IsConnected 檢查是否已連接.
func IsConnected() bool {
	return mongoClient != nil
}

// CloseMongo 關閉 MongoDB 連接.
func CloseMongo() error {
	if mongoClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return mongoClient.Disconnect(ctx)
	}
	return nil
}

// loadMongoTLSConfig 載入 MongoDB TLS 配置
func loadMongoTLSConfig(cfg *config.MongoConfig) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// 如果設置了跳過驗證（僅開發環境）
	if cfg.TLSInsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
		logger.LogInfof("[WARNING] MongoDB TLS 證書驗證已跳過（僅開發環境）")
		return tlsConfig, nil
	}

	// 載入 CA 證書
	if cfg.TLSCAFile != "" {
		caCert, err := os.ReadFile(cfg.TLSCAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
			return nil, fmt.Errorf("failed to append CA certs")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// 如果有客戶端證書，載入它
	if cfg.TLSCertFile != "" && cfg.TLSKeyFile != "" {
		clientCert, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{clientCert}
	}

	return tlsConfig, nil
}

// maskPassword 遮蔽 URL 中的密碼
func maskPassword(url string) string {
	if url == "" {
		return "(空)"
	}

	// 遮蔽密碼部分
	re := regexp.MustCompile(`(mongodb(?:\+srv)?://[^:]+:)([^@]+)(@.+)`)
	masked := re.ReplaceAllString(url, `$1***$3`)

	if masked == url {
		return url
	}

	return masked
}
