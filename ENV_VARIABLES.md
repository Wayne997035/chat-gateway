# 環境變量配置說明

本文檔列出了所有可用的環境變量配置選項。

## 應用配置

```bash
APP_ENV=development          # 環境：development, staging, production
APP_DEBUG=true              # 是否啟用調試模式
```

## CORS 配置

```bash
# 允許的來源（逗號分隔）
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080,https://yourdomain.com
```

## Rate Limiting 配置

```bash
RATE_LIMIT_ENABLED=true     # 是否啟用速率限制
RATE_LIMIT_PER_MINUTE=100   # 每分鐘允許的請求數
RATE_LIMIT_PER_HOUR=1000    # 每小時允許的請求數
```

## TLS/HTTPS 配置

```bash
TLS_ENABLED=false                    # 是否啟用 TLS（生產環境建議 true）
TLS_CERT_FILE=/path/to/cert.pem     # TLS 證書文件路徑
TLS_KEY_FILE=/path/to/key.pem       # TLS 密鑰文件路徑
```

## JWT 配置（未來用）

```bash
JWT_SECRET=your-secret-key           # JWT 密鑰（生產環境必須使用強密鑰）
JWT_EXPIRATION=15                    # JWT 過期時間（分鐘）
```

## 審計日誌配置

```bash
AUDIT_ENABLED=true          # 是否啟用審計日誌
```

## MongoDB 配置

```bash
MONGO_URL=mongodb://localhost:27017  # MongoDB 連接 URL
MONGO_DATABASE=chat_gateway          # 數據庫名稱
MONGO_TLS_ENABLED=false              # 是否啟用 MongoDB TLS
MONGO_USERNAME=                      # MongoDB 用戶名（生產環境建議使用）
MONGO_PASSWORD=                      # MongoDB 密碼（生產環境建議使用）
```

## gRPC 配置

```bash
GRPC_ADDRESS=localhost:8081              # gRPC 服務地址
GRPC_TLS_ENABLED=false                   # 是否啟用 gRPC TLS
GRPC_CERT_FILE=/path/to/grpc-cert.pem   # gRPC 證書文件路徑
GRPC_KEY_FILE=/path/to/grpc-key.pem     # gRPC 密鑰文件路徑
GRPC_SERVER_NAME=localhost               # gRPC 服務器名稱
```

## 伺服器配置

```bash
SERVER_HOST=0.0.0.0         # HTTP 服務器監聽地址
SERVER_PORT=8080            # HTTP 服務器端口
```

## 日誌配置

```bash
LOG_LEVEL=info              # 日誌級別：debug, info, warn, error
LOG_FORMAT=json             # 日誌格式：json, text
```

## 開發環境配置示例

```bash
# 開發環境 .env
APP_ENV=development
APP_DEBUG=true
ALLOWED_ORIGINS=http://localhost:3000,http://localhost:8080
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=100
TLS_ENABLED=false
GRPC_TLS_ENABLED=false
AUDIT_ENABLED=true
MONGO_URL=mongodb://localhost:27017
MONGO_DATABASE=chat_gateway
```

## 生產環境配置示例

```bash
# 生產環境 .env
APP_ENV=production
APP_DEBUG=false
ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com
RATE_LIMIT_ENABLED=true
RATE_LIMIT_PER_MINUTE=100
RATE_LIMIT_PER_HOUR=1000

# TLS 必須啟用
TLS_ENABLED=true
TLS_CERT_FILE=/etc/ssl/certs/chat-gateway.pem
TLS_KEY_FILE=/etc/ssl/private/chat-gateway-key.pem

# gRPC TLS
GRPC_TLS_ENABLED=true
GRPC_CERT_FILE=/etc/ssl/certs/grpc.pem
GRPC_KEY_FILE=/etc/ssl/private/grpc-key.pem
GRPC_SERVER_NAME=grpc.yourdomain.com

# JWT
JWT_SECRET=your-very-strong-secret-key-change-this
JWT_EXPIRATION=15

# MongoDB 安全配置
MONGO_URL=mongodb://mongo1.yourdomain.com:27017,mongo2.yourdomain.com:27017
MONGO_DATABASE=chat_gateway
MONGO_TLS_ENABLED=true
MONGO_USERNAME=chat_user
MONGO_PASSWORD=your-strong-password

# 審計
AUDIT_ENABLED=true

# 日誌
LOG_LEVEL=warn
LOG_FORMAT=json
```

## 安全建議

### ⚠️ 重要提示

1. **永遠不要將 .env 文件提交到版本控制系統**
2. **生產環境必須使用強密碼和密鑰**
3. **生產環境必須啟用 TLS/HTTPS**
4. **定期輪換密鑰和證書**

### 生成強密鑰

```bash
# 生成 JWT 密鑰
openssl rand -base64 32

# 生成 TLS 證書（用於測試）
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes
```

### MongoDB 安全配置

```bash
# 創建 MongoDB 用戶
mongo admin --eval "db.createUser({
  user: 'chat_user',
  pwd: 'your-strong-password',
  roles: [{role: 'readWrite', db: 'chat_gateway'}]
})"

# 啟用認證後的連接字符串
MONGO_URL=mongodb://chat_user:your-strong-password@localhost:27017/chat_gateway?authSource=admin
```

## 配置優先級

1. 環境變量（最高）
2. .env 文件
3. 配置文件 (configs/*.yaml)
4. 默認值（最低）

## 配置驗證

在應用啟動時，會自動驗證配置的有效性：

- TLS 文件是否存在
- 端口是否有效
- MongoDB 連接是否成功
- gRPC 連接是否正常

如果配置無效，應用將無法啟動並顯示錯誤消息。

