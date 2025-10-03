# 聊天服務快速啟動指南

## 🚀 快速開始

### 1. 啟動 MongoDB
```bash
# 使用 Docker（推薦）
docker run -d --name mongodb -p 27017:27017 mongo:latest

# 或使用本地 MongoDB
mongod --dbpath /path/to/your/db
```

### 2. 啟動聊天服務
```bash
# 使用 Air（推薦，支持熱重載）
air

# 或使用 Go 直接啟動
go run cmd/api/main.go
```

### 3. 測試服務
```bash
# 檢查服務狀態
curl http://localhost:8080/health

# 運行自動化測試
./scripts/test_chat.sh
```

## 📱 測試聊天功能

### 1對1 聊天測試
```bash
# 創建 1對1 聊天室
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "1對1聊天",
    "type": "direct",
    "owner_id": "user1",
    "members": [
      {"user_id": "user1", "role": "admin"},
      {"user_id": "user2", "role": "member"}
    ]
  }'
```

### 群組聊天測試
```bash
# 創建群組聊天室
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "群組聊天",
    "type": "group",
    "owner_id": "user1",
    "members": [
      {"user_id": "user1", "role": "admin"},
      {"user_id": "user2", "role": "member"},
      {"user_id": "user3", "role": "member"}
    ]
  }'
```

### 發送消息
```bash
# 替換 ROOM_ID 為實際的聊天室 ID
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "room_id": "ROOM_ID",
    "sender_id": "user1",
    "content": "你好，這是測試消息",
    "type": "text"
  }'
```

### 獲取消息（分頁）
```bash
# 獲取歷史消息（分頁）
curl "http://localhost:8080/api/v1/messages/history?room_id=ROOM_ID&user_id=user1&limit=10"

# 獲取普通消息
curl "http://localhost:8080/api/v1/messages?room_id=ROOM_ID&user_id=user1&limit=10"
```

## 🌐 WebSocket 測試

### 使用測試頁面
1. 在瀏覽器中打開 `scripts/websocket_test.html`
2. 輸入聊天室 ID 和用戶 ID
3. 點擊連接
4. 發送消息測試

### 使用 wscat 工具
```bash
# 安裝 wscat
npm install -g wscat

# 連接 WebSocket
wscat -c "ws://localhost:8080/ws?room_id=ROOM_ID&user_id=USER_ID"
```

## 📊 分頁功能

### 歷史消息分頁
- 默認每頁 20 條消息
- 最大每頁 50 條消息
- 支持游標分頁，性能優化

### 分頁參數
- `limit`: 每頁消息數量（1-50）
- `cursor`: 分頁游標（時間戳格式）
- `since`: 開始時間（ISO 8601 格式）
- `until`: 結束時間（ISO 8601 格式）

## 🔧 配置

### 環境變量
```bash
# 數據庫配置
export MONGODB_URI="mongodb://localhost:27017"
export DB_NAME="chatroom"

# 服務器配置
export PORT="8080"
export GIN_MODE="debug"
```

### 配置文件
- `configs/local.yaml` - 本地開發配置
- `configs/development.yaml` - 開發環境配置
- `configs/staging.yaml` - 測試環境配置
- `configs/production.yaml` - 生產環境配置

## 🐛 故障排除

### 服務無法啟動
1. 檢查 MongoDB 是否運行
2. 檢查端口 8080 是否被占用
3. 查看錯誤日誌

### WebSocket 連接失敗
1. 檢查聊天室 ID 和用戶 ID
2. 確認用戶是聊天室成員
3. 檢查防火牆設置

### 消息發送失敗
1. 檢查聊天室是否存在
2. 確認用戶權限
3. 查看數據庫連接狀態

## 📈 性能優化

### 數據庫索引
服務啟動時會自動創建以下索引：
- `room_id + created_at` - 消息查詢優化
- `sender_id + created_at` - 發送者查詢優化
- `type` - 消息類型查詢優化
- `content` - 全文搜索優化

### 分頁限制
- 普通消息：最大 100 條/頁
- 歷史消息：最大 50 條/頁
- 自動分頁，防止性能問題

## 📚 更多文檔

- [完整測試指南](TESTING.md)
- [API 文檔](README.md#api-文檔)
- [項目結構](README.md#項目結構)

## 🆘 需要幫助？

如果遇到問題，請：
1. 查看 [TESTING.md](TESTING.md) 詳細測試指南
2. 檢查服務日誌
3. 確認 MongoDB 連接狀態
4. 驗證 API 請求格式
