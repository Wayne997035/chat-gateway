# gRPC TLS 配置指南

## 概述

Chat Gateway 支持通過 TLS 加密 gRPC 通訊，保護數據傳輸安全。

## 開發環境配置

### 1. 生成自簽證書

```bash
# 執行證書生成腳本
./scripts/generate_certs.sh
```

這將在 `certs/` 目錄生成：
- `ca-cert.pem` - CA 證書（用於驗證）
- `server.crt` - 服務器證書
- `server.key` - 服務器私鑰

### 2. 啟用 TLS

修改 `configs/local.yaml`:

```yaml
security:
  tls:
    enabled: true                    # 啟用 TLS
    cert_file: "certs/server.crt"
    key_file: "certs/server.key"
    ca_file: ""                      # 可選：啟用雙向 TLS 時使用
```

### 3. 啟動服務

```bash
go run cmd/api/main.go
```

應該看到日誌：
```
gRPC TLS 已啟用
gRPC 服務器初始化 - 加密: true, 審計: true, TLS: true
```

## 生產環境配置

### 1. 使用受信任的證書

**不要使用自簽證書！** 從受信任的 CA（如 Let's Encrypt）獲取證書。

```bash
# 使用 Let's Encrypt（範例）
certbot certonly --standalone -d your-domain.com

# 證書位置（通常）
# /etc/letsencrypt/live/your-domain.com/fullchain.pem
# /etc/letsencrypt/live/your-domain.com/privkey.pem
```

### 2. 配置文件

```yaml
security:
  tls:
    enabled: true
    cert_file: "/etc/letsencrypt/live/your-domain.com/fullchain.pem"
    key_file: "/etc/letsencrypt/live/your-domain.com/privkey.pem"
    ca_file: ""
```

### 3. 文件權限

確保證書和私鑰文件權限正確：

```bash
# 私鑰只能由服務讀取
chmod 600 /path/to/server.key

# 證書可以公開讀取
chmod 644 /path/to/server.crt
```

## 雙向 TLS（mTLS）

如果需要客戶端證書驗證：

### 1. 生成客戶端證書

```bash
# 生成客戶端私鑰
openssl genrsa -out certs/client-key.pem 4096

# 生成客戶端證書請求
openssl req -new -key certs/client-key.pem -out certs/client-req.pem \
  -subj "/C=TW/ST=Taipei/L=Taipei/O=ChatGateway/OU=Client/CN=client"

# 簽名客戶端證書
openssl x509 -req -in certs/client-req.pem \
  -days 365 -CA certs/ca-cert.pem -CAkey certs/ca-key.pem -CAcreateserial \
  -out certs/client-cert.pem
```

### 2. 配置服務器

```yaml
security:
  tls:
    enabled: true
    cert_file: "certs/server.crt"
    key_file: "certs/server.key"
    ca_file: "certs/ca-cert.pem"  # 啟用客戶端驗證
```

## TLS 配置詳解

### MinVersion

當前設置為 **TLS 1.2** 作為最低版本，符合安全標準。

### 加密套件

使用強加密套件（按優先級排序）：
1. `TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384`
2. `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`
3. `TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384`
4. `TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256`

所有套件都支持：
- ✅ 前向保密（Forward Secrecy）
- ✅ AEAD 加密模式（GCM）
- ✅ 強加密算法（AES-256/AES-128）

## 測試 TLS 連接

### 使用 grpcurl

```bash
# 無 TLS（開發環境）
grpcurl -plaintext localhost:8081 list

# 有 TLS（需要指定證書）
grpcurl -cacert certs/ca-cert.pem \
  localhost:8081 list
```

### 使用 openssl

```bash
# 測試 TLS 握手
openssl s_client -connect localhost:8081 -showcerts
```

## 常見問題

### 1. 證書驗證失敗

**錯誤**: `x509: certificate signed by unknown authority`

**解決**: 
- 開發環境：確保使用 `-cacert` 指定 CA 證書
- 生產環境：使用受信任 CA 簽發的證書

### 2. 找不到證書文件

**錯誤**: `failed to load key pair: open certs/server.crt: no such file or directory`

**解決**: 
- 執行 `./scripts/generate_certs.sh` 生成證書
- 檢查配置文件中的路徑是否正確

### 3. 權限錯誤

**錯誤**: `permission denied`

**解決**: 
```bash
chmod 600 certs/server.key
chmod 644 certs/server.crt
```

## 安全建議

### 開發環境
- ✅ 可以使用自簽證書
- ✅ 可以關閉 TLS（`enabled: false`）
- ⚠️ 不要將私鑰提交到版本控制

### 生產環境
- ✅ **必須**啟用 TLS
- ✅ 使用受信任 CA 簽發的證書
- ✅ 定期更新證書（Let's Encrypt 90天）
- ✅ 配置自動續簽
- ✅ 使用強密碼保護私鑰
- ✅ 定期輪換證書
- ❌ 不要使用自簽證書

## 證書續簽

### Let's Encrypt 自動續簽

```bash
# 設置 cron job
0 3 * * * certbot renew --post-hook "systemctl restart chat-gateway"
```

### 監控證書到期

```bash
# 檢查證書有效期
openssl x509 -in /path/to/cert.pem -noout -dates
```

## 參考資料

- [gRPC Authentication](https://grpc.io/docs/guides/auth/)
- [Let's Encrypt](https://letsencrypt.org/)
- [TLS Best Practices](https://wiki.mozilla.org/Security/Server_Side_TLS)

