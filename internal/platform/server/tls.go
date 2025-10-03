package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// TLSConfig TLS 配置
type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
	CAFile   string
}

// LoadTLSCredentials 載入 TLS 憑證
func LoadTLSCredentials(config TLSConfig) (credentials.TransportCredentials, error) {
	if !config.Enabled {
		return nil, nil
	}

	// 載入服務器憑證和私鑰
	serverCert, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %v", err)
	}

	// 創建憑證池
	certPool := x509.NewCertPool()

	// 如果提供了 CA 文件，載入它
	if config.CAFile != "" {
		ca, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %v", err)
		}

		if !certPool.AppendCertsFromPEM(ca) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
	}

	// 配置 TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert, // 不要求客戶端憑證
		MinVersion:   tls.VersionTLS13, // 只接受 TLS 1.3
		ClientCAs:    certPool,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// GenerateSelfSignedCert 生成自簽名憑證（僅用於開發/測試）
// 生產環境應使用正式的憑證（如 Let's Encrypt）
func GenerateSelfSignedCert(certFile, keyFile string) error {
	// 這裡可以使用 crypto/x509 和 crypto/rsa 生成自簽名憑證
	// 但建議使用外部工具如 openssl 或 cfssl
	
	// openssl 命令範例：
	// openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes \
	//   -subj "/C=TW/ST=Taiwan/L=Taipei/O=ChatGateway/CN=localhost"
	
	return fmt.Errorf("請使用 openssl 或其他工具生成憑證")
}

