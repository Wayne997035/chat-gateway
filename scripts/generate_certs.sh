#!/bin/bash

# 生成 TLS 證書腳本
# 用於開發環境的自簽證書

set -e

# 創建證書目錄
mkdir -p certs

echo "正在生成自簽 TLS 證書..."

# 生成 CA 私鑰和證書（可選）
openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
  -keyout certs/ca-key.pem \
  -out certs/ca-cert.pem \
  -subj "/C=TW/ST=Taipei/L=Taipei/O=ChatGateway Dev/OU=Dev/CN=localhost/emailAddress=dev@example.com"

echo "✅ CA 證書已生成"

# 生成服務器私鑰
openssl genrsa -out certs/server-key.pem 4096

echo "✅ 服務器私鑰已生成"

# 創建服務器證書簽名請求 (CSR)
openssl req -new -key certs/server-key.pem -out certs/server-req.pem \
  -subj "/C=TW/ST=Taipei/L=Taipei/O=ChatGateway/OU=Server/CN=localhost/emailAddress=server@example.com"

echo "✅ 證書簽名請求已生成"

# 創建擴展配置文件（允許 localhost 和 127.0.0.1）
cat > certs/server-ext.cnf <<EOF
subjectAltName = DNS:localhost,IP:127.0.0.1,IP:0.0.0.0
EOF

# 使用 CA 簽名服務器證書
openssl x509 -req -in certs/server-req.pem \
  -days 365 -CA certs/ca-cert.pem -CAkey certs/ca-key.pem -CAcreateserial \
  -out certs/server-cert.pem \
  -extfile certs/server-ext.cnf

echo "✅ 服務器證書已簽名"

# 清理臨時文件
rm certs/server-req.pem certs/server-ext.cnf

# 創建符號鏈接（方便配置）
ln -sf server-cert.pem certs/server.crt
ln -sf server-key.pem certs/server.key

echo ""
echo "證書生成完成！"
echo "================================"
echo "CA 證書:        certs/ca-cert.pem"
echo "服務器證書:     certs/server.crt"
echo "服務器私鑰:     certs/server.key"
echo "================================"
echo ""
echo "配置文件設置："
echo "security:"
echo "  tls:"
echo "    enabled: true"
echo "    cert_file: \"certs/server.crt\""
echo "    key_file: \"certs/server.key\""
echo "    ca_file: \"certs/ca-cert.pem\"  # 可選"
echo ""
echo "⚠️  這些是自簽證書，僅用於開發環境！"
echo "⚠️  生產環境請使用受信任的 CA 簽發的證書！"

