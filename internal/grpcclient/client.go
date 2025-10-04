package grpcclient

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"

	"chat-gateway/internal/platform/config"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	conn *grpc.ClientConn
	mu   sync.RWMutex
)

// GetConnection 獲取或創建 gRPC 客戶端連接（單例模式）
// 自動從配置讀取地址
func GetConnection() (*grpc.ClientConn, error) {
	mu.RLock()
	if conn != nil {
		mu.RUnlock()
		return conn, nil
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()

	// 再次檢查（雙重檢查鎖定）
	if conn != nil {
		return conn, nil
	}

	// 從配置讀取 gRPC 服務器地址
	cfg := config.Get()
	if cfg == nil {
		return nil, fmt.Errorf("config not loaded")
	}

	address := fmt.Sprintf("%s:%s", cfg.GRPC.Host, cfg.GRPC.Port)

	// 創建新連接
	var err error
	if cfg.Security.TLS.Enabled {
		// 使用 TLS
		conn, err = dialWithTLS(address, cfg.Security.TLS)
	} else {
		// 開發環境：不使用 TLS
		conn, err = dialInsecure(address)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to gRPC server at %s: %w", address, err)
	}

	return conn, nil
}

// dialWithTLS 使用 TLS 連接
func dialWithTLS(address string, tlsConfig config.TLSConfig) (*grpc.ClientConn, error) {
	var tlsCreds credentials.TransportCredentials

	// 如果有客戶端證書（雙向 TLS）
	if tlsConfig.CertFile != "" && tlsConfig.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client cert: %w", err)
		}

		// 創建證書池
		certPool := x509.NewCertPool()
		if tlsConfig.CAFile != "" {
			ca, err := os.ReadFile(tlsConfig.CAFile)
			if err != nil {
				return nil, fmt.Errorf("failed to read CA cert: %w", err)
			}
			if ok := certPool.AppendCertsFromPEM(ca); !ok {
				return nil, fmt.Errorf("failed to append CA cert")
			}
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      certPool,
			MinVersion:   tls.VersionTLS12,
		}

		tlsCreds = credentials.NewTLS(tlsConfig)
	} else {
		// 只驗證服務器證書
		tlsCreds = credentials.NewTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
		})
	}

	return grpc.NewClient(address, grpc.WithTransportCredentials(tlsCreds))
}

// dialInsecure 不使用 TLS 連接（僅開發環境）
func dialInsecure(address string) (*grpc.ClientConn, error) {
	fmt.Println("[WARNING] gRPC 使用不安全連接（開發環境）")
	return grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
}

// CloseConnection 關閉 gRPC 連接
func CloseConnection() error {
	mu.Lock()
	defer mu.Unlock()

	if conn != nil {
		err := conn.Close()
		conn = nil
		return err
	}
	return nil
}

// IsConnected 檢查是否已連接
func IsConnected() bool {
	mu.RLock()
	defer mu.RUnlock()
	return conn != nil
}
