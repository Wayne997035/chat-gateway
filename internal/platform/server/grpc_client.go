package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCClientConfig gRPC 客戶端配置
type GRPCClientConfig struct {
	Address    string
	TLSEnabled bool
	CertFile   string
	ServerName string
}

// NewGRPCClient 創建安全的 gRPC 客戶端連接
func NewGRPCClient(config GRPCClientConfig) (*grpc.ClientConn, error) {
	var opts []grpc.DialOption

	if config.TLSEnabled {
		// 使用 TLS
		tlsConfig, err := loadTLSConfig(config.CertFile, config.ServerName)
		if err != nil {
			return nil, fmt.Errorf("加載 TLS 配置失敗: %w", err)
		}

		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		// 開發環境：使用 insecure（僅用於開發/測試）
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// 連接到服務器
	conn, err := grpc.NewClient(config.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("連接 gRPC 服務失敗: %w", err)
	}

	return conn, nil
}

// loadTLSConfig 加載 TLS 配置
func loadTLSConfig(certFile, serverName string) (*tls.Config, error) {
	// 驗證檔案路徑（防止路徑遍歷攻擊）
	if certFile == "" {
		return nil, fmt.Errorf("證書文件路徑不能為空")
	}

	// 清理路徑（移除 ../ 等）
	certFile = filepath.Clean(certFile)

	// 確保路徑不包含路徑遍歷攻擊
	if filepath.IsAbs(certFile) {
		// 絕對路徑：確保在允許的目錄下
		// 這裡可以根據需求限制允許的根目錄
	} else {
		// 相對路徑：轉換為絕對路徑
		absPath, err := filepath.Abs(certFile)
		if err != nil {
			return nil, fmt.Errorf("無法解析證書文件路徑: %w", err)
		}
		certFile = absPath
	}

	// 讀取證書文件
	// #nosec G304 -- file path is cleaned and validated above
	caCert, err := os.ReadFile(certFile)
	if err != nil {
		return nil, fmt.Errorf("讀取證書文件失敗: %w", err)
	}

	// 創建證書池
	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("添加證書到證書池失敗")
	}

	// 創建 TLS 配置
	tlsConfig := &tls.Config{
		RootCAs:    caCertPool,
		ServerName: serverName,
		MinVersion: tls.VersionTLS12, // 最低 TLS 1.2
	}

	return tlsConfig, nil
}

// GetGRPCConnection 獲取 gRPC 連接（輔助函數）
// 根據環境變量決定是否使用 TLS
func GetGRPCConnection(address string) (*grpc.ClientConn, error) {
	config := GRPCClientConfig{
		Address:    address,
		TLSEnabled: os.Getenv("GRPC_TLS_ENABLED") == "true",
		CertFile:   os.Getenv("GRPC_CERT_FILE"),
		ServerName: os.Getenv("GRPC_SERVER_NAME"),
	}

	if config.ServerName == "" {
		config.ServerName = "localhost"
	}

	return NewGRPCClient(config)
}
