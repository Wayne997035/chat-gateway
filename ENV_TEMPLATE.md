# 環境變量配置範本

複製以下內容創建 `.env` 文件：

```bash
# MongoDB 配置
MONGO_USERNAME=your_username
MONGO_PASSWORD=your_secure_password

# JWT 配置（等待 user 服務實現）
JWT_SECRET=your-secure-jwt-secret-at-least-32-characters-long

# 加密主密鑰（256-bit，base64 編碼）
# 生成方式：openssl rand -base64 32
MASTER_KEY=your-base64-encoded-32-byte-master-key

# 密鑰輪換配置
KEY_ROTATION_ENABLED=false  # 是否啟用自動密鑰輪換（生產環境建議 true）
```

## 開發環境範例

```bash
# 留空則使用無認證 MongoDB（開發環境）
MONGO_USERNAME=
MONGO_PASSWORD=

# 開發環境 JWT secret
JWT_SECRET=dev-jwt-secret-change-in-production

# 留空則自動生成臨時主密鑰（重啟後舊訊息無法解密）
MASTER_KEY=
```

## 生產環境範例

```bash
# MongoDB 認證（必須設置）
MONGO_USERNAME=chat_user
MONGO_PASSWORD=your_very_secure_password_here

# JWT secret（必須設置，至少 32 字符）
JWT_SECRET=prod-jwt-secret-min-32-chars-random-string

# 主密鑰（必須設置，base64 編碼的 32 bytes）
MASTER_KEY=$(openssl rand -base64 32)
```

## 生成安全密鑰

```bash
# 生成主密鑰
openssl rand -base64 32

# 生成 JWT secret
openssl rand -base64 32

# 生成隨機密碼
openssl rand -base64 24
```

