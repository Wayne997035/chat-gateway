#!/bin/bash

# TLS 憑證生成腳本
# 用於開發/測試環境生成自簽名憑證
# 生產環境請使用正式 CA 簽發的憑證（如 Let's Encrypt）

set -e

# 顏色定義
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 配置
CERTS_DIR="./certs"
DAYS_VALID=365
COUNTRY="TW"
STATE="Taiwan"
CITY="Taipei"
ORG="ChatGateway"
COMMON_NAME="localhost"

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}TLS 憑證生成工具${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

# 檢查 openssl
if ! command -v openssl &> /dev/null; then
    echo -e "${RED}錯誤: 未找到 openssl 命令${NC}"
    echo "請先安裝 openssl:"
    echo "  macOS: brew install openssl"
    echo "  Ubuntu/Debian: sudo apt-get install openssl"
    exit 1
fi

# 創建憑證目錄
mkdir -p "$CERTS_DIR"
echo -e "${GREEN}✓${NC} 創建憑證目錄: $CERTS_DIR"

# 生成 CA 私鑰
echo -e "${BLUE}生成 CA 私鑰...${NC}"
openssl genrsa -out "$CERTS_DIR/ca.key" 4096
echo -e "${GREEN}✓${NC} CA 私鑰生成完成: $CERTS_DIR/ca.key"

# 生成 CA 證書
echo -e "${BLUE}生成 CA 證書...${NC}"
openssl req -new -x509 -days $DAYS_VALID -key "$CERTS_DIR/ca.key" -out "$CERTS_DIR/ca.crt" -subj "/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORG/CN=$ORG-CA"
echo -e "${GREEN}✓${NC} CA 證書生成完成: $CERTS_DIR/ca.crt"

# 生成服務器私鑰
echo -e "${BLUE}生成服務器私鑰...${NC}"
openssl genrsa -out "$CERTS_DIR/server.key" 4096
echo -e "${GREEN}✓${NC} 服務器私鑰生成完成: $CERTS_DIR/server.key"

# 生成服務器 CSR（證書簽名請求）
echo -e "${BLUE}生成服務器 CSR...${NC}"
openssl req -new -key "$CERTS_DIR/server.key" -out "$CERTS_DIR/server.csr" -subj "/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORG/CN=$COMMON_NAME"
echo -e "${GREEN}✓${NC} 服務器 CSR 生成完成: $CERTS_DIR/server.csr"

# 創建擴展配置文件（支持多個域名和 IP）
cat > "$CERTS_DIR/server.ext" << EOF
authorityKeyIdentifier=keyid,issuer
basicConstraints=CA:FALSE
keyUsage = digitalSignature, nonRepudiation, keyEncipherment, dataEncipherment
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = *.localhost
DNS.3 = chat-gateway
DNS.4 = *.chat-gateway.local
IP.1 = 127.0.0.1
IP.2 = ::1
EOF

# 使用 CA 簽署服務器證書
echo -e "${BLUE}使用 CA 簽署服務器證書...${NC}"
openssl x509 -req -in "$CERTS_DIR/server.csr" -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" -CAcreateserial -out "$CERTS_DIR/server.crt" -days $DAYS_VALID -sha256 -extfile "$CERTS_DIR/server.ext"
echo -e "${GREEN}✓${NC} 服務器證書生成完成: $CERTS_DIR/server.crt"

# 生成客戶端私鑰（可選，用於雙向 TLS）
echo -e "${BLUE}生成客戶端私鑰...${NC}"
openssl genrsa -out "$CERTS_DIR/client.key" 4096
echo -e "${GREEN}✓${NC} 客戶端私鑰生成完成: $CERTS_DIR/client.key"

# 生成客戶端 CSR
echo -e "${BLUE}生成客戶端 CSR...${NC}"
openssl req -new -key "$CERTS_DIR/client.key" -out "$CERTS_DIR/client.csr" -subj "/C=$COUNTRY/ST=$STATE/L=$CITY/O=$ORG/CN=client"
echo -e "${GREEN}✓${NC} 客戶端 CSR 生成完成: $CERTS_DIR/client.csr"

# 使用 CA 簽署客戶端證書
echo -e "${BLUE}使用 CA 簽署客戶端證書...${NC}"
openssl x509 -req -in "$CERTS_DIR/client.csr" -CA "$CERTS_DIR/ca.crt" -CAkey "$CERTS_DIR/ca.key" -CAcreateserial -out "$CERTS_DIR/client.crt" -days $DAYS_VALID -sha256
echo -e "${GREEN}✓${NC} 客戶端證書生成完成: $CERTS_DIR/client.crt"

# 設置權限
chmod 600 "$CERTS_DIR"/*.key
chmod 644 "$CERTS_DIR"/*.crt

# 清理臨時文件
rm -f "$CERTS_DIR"/*.csr "$CERTS_DIR"/*.ext "$CERTS_DIR"/*.srl

echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}✓ 憑證生成完成！${NC}"
echo -e "${GREEN}================================${NC}"
echo ""
echo -e "${BLUE}生成的文件：${NC}"
echo "  CA 證書:       $CERTS_DIR/ca.crt"
echo "  CA 私鑰:       $CERTS_DIR/ca.key"
echo "  服務器證書:    $CERTS_DIR/server.crt"
echo "  服務器私鑰:    $CERTS_DIR/server.key"
echo "  客戶端證書:    $CERTS_DIR/client.crt"
echo "  客戶端私鑰:    $CERTS_DIR/client.key"
echo ""
echo -e "${YELLOW}使用說明：${NC}"
echo "1. 在配置文件中啟用 TLS："
echo "   security:"
echo "     tls:"
echo "       enabled: true"
echo "       cert_file: \"$CERTS_DIR/server.crt\""
echo "       key_file: \"$CERTS_DIR/server.key\""
echo "       ca_file: \"$CERTS_DIR/ca.crt\""
echo ""
echo "2. 測試 gRPC 連接（使用 grpcurl）："
echo "   grpcurl -cacert $CERTS_DIR/ca.crt \\"
echo "     -import-path proto -proto chat.proto \\"
echo "     localhost:8081 list"
echo ""
echo -e "${YELLOW}注意：${NC}"
echo "• 這些是自簽名憑證，僅適用於開發/測試環境"
echo "• 生產環境請使用正式 CA（如 Let's Encrypt）簽發的憑證"
echo "• 憑證有效期: $DAYS_VALID 天"
echo "• 請妥善保管私鑰文件（已設置為 600 權限）"
echo ""

