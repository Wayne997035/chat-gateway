# 聊天服務測試指南

## 服務啟動

### 使用 Air 啟動（推薦）
```bash
# 安裝 Air（如果還沒安裝）
go install github.com/cosmtrek/air@latest

# 啟動服務
air
```

### 使用 Go 直接啟動
```bash
# 啟動服務
go run cmd/api/main.go

# 或者編譯後運行
go build -o bin/chat-gateway cmd/api/main.go
./bin/chat-gateway
```

## 測試環境準備

### 1. 確保 MongoDB 運行
```bash
# 使用 Docker 啟動 MongoDB
docker run -d --name mongodb -p 27017:27017 mongo:latest

# 或者使用本地 MongoDB
mongod --dbpath /path/to/your/db
```

### 2. 檢查服務狀態
```bash
curl http://localhost:8080/health
```

應該返回：
```json
{
  "status": "ok",
  "timestamp": "2024-01-01T00:00:00Z",
  "services": {
    "database": "ok",
    "memory": "ok"
  }
}
```

## 自動化測試

### 運行完整測試腳本
```bash
# 運行測試腳本
./scripts/test_chat.sh

# 如果沒有執行權限
chmod +x scripts/test_chat.sh
./scripts/test_chat.sh
```

測試腳本會自動測試：
- 1對1 聊天室創建和消息發送
- 群組聊天室創建和消息發送
- 歷史消息分頁查詢
- WebSocket 連接說明

## 手動 API 測試

### 1. 創建 1對1 聊天室
```bash
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "1對1聊天測試",
    "type": "direct",
    "owner_id": "user1",
    "members": [
      {"user_id": "user1", "role": "admin"},
      {"user_id": "user2", "role": "member"}
    ]
  }'
```

### 2. 創建群組聊天室
```bash
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "群組聊天測試",
    "type": "group",
    "owner_id": "user1",
    "members": [
      {"user_id": "user1", "role": "admin"},
      {"user_id": "user2", "role": "member"},
      {"user_id": "user3", "role": "member"},
      {"user_id": "user4", "role": "member"}
    ]
  }'
```

### 3. 發送消息
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

### 4. 獲取消息列表
```bash
# 獲取普通消息
curl "http://localhost:8080/api/v1/messages?room_id=ROOM_ID&user_id=user1&limit=10"

# 獲取歷史消息（分頁）
curl "http://localhost:8080/api/v1/messages/history?room_id=ROOM_ID&user_id=user1&limit=5"
```

### 5. 搜索消息
```bash
curl "http://localhost:8080/api/v1/messages/search?room_id=ROOM_ID&user_id=user1&query=測試&limit=10"
```

## WebSocket 測試

### 1. 使用測試頁面
```bash
# 在瀏覽器中打開
open scripts/websocket_test.html
```

或者直接訪問：`file:///path/to/chat-gateway/scripts/websocket_test.html`

### 2. 使用 wscat 工具
```bash
# 安裝 wscat
npm install -g wscat

# 連接 WebSocket
wscat -c "ws://localhost:8080/ws?room_id=ROOM_ID&user_id=USER_ID"
```

### 3. WebSocket 消息格式
```json
// 發送消息
{
  "type": "message",
  "content": "你好",
  "timestamp": "2024-01-01T00:00:00Z"
}

// 加入聊天室
{
  "type": "join_room",
  "room_id": "ROOM_ID",
  "user_id": "USER_ID"
}

// 離開聊天室
{
  "type": "leave_room",
  "room_id": "ROOM_ID",
  "user_id": "USER_ID"
}
```

## 性能測試

### 1. 大量消息測試
```bash
# 發送 100 條消息
for i in {1..100}; do
  curl -X POST http://localhost:8080/api/v1/messages \
    -H "Content-Type: application/json" \
    -d "{
      \"room_id\": \"ROOM_ID\",
      \"sender_id\": \"user$((i % 4 + 1))\",
      \"content\": \"測試消息 #$i\",
      \"type\": \"text\"
    }"
  sleep 0.1
done
```

### 2. 分頁性能測試
```bash
# 測試不同分頁大小
for limit in 10 20 50 100; do
  echo "測試分頁大小: $limit"
  time curl -s "http://localhost:8080/api/v1/messages/history?room_id=ROOM_ID&user_id=user1&limit=$limit" > /dev/null
done
```

## 錯誤處理測試

### 1. 無效的聊天室 ID
```bash
curl "http://localhost:8080/api/v1/messages?room_id=invalid&user_id=user1"
```

### 2. 非聊天室成員
```bash
curl "http://localhost:8080/api/v1/messages?room_id=ROOM_ID&user_id=unauthorized_user"
```

### 3. 無效的分頁參數
```bash
curl "http://localhost:8080/api/v1/messages/history?room_id=ROOM_ID&user_id=user1&limit=1000"
```

## 監控和日誌

### 1. 查看服務日誌
```bash
# 如果使用 Air
# 日誌會直接顯示在終端

# 如果使用 systemd
journalctl -u chat-gateway -f
```

### 2. 檢查數據庫索引
```bash
# 連接到 MongoDB
mongo

# 查看索引
use chatroom
db.messages.getIndexes()
db.chat_rooms.getIndexes()
```

### 3. 性能監控
```bash
# 檢查內存使用
curl http://localhost:8080/health

# 檢查數據庫連接
curl http://localhost:8080/health | jq '.services.database'
```

## 常見問題

### 1. 服務無法啟動
- 檢查 MongoDB 是否運行
- 檢查端口 8080 是否被占用
- 查看錯誤日誌

### 2. WebSocket 連接失敗
- 檢查聊天室 ID 和用戶 ID 是否正確
- 確認用戶是聊天室成員
- 檢查防火牆設置

### 3. 消息發送失敗
- 檢查聊天室是否存在
- 確認用戶權限
- 查看數據庫連接狀態

### 4. 分頁查詢慢
- 檢查數據庫索引是否創建
- 確認查詢參數是否合理
- 考慮增加緩存

## 測試數據清理

```bash
# 清理測試數據
mongo chatroom --eval "db.messages.deleteMany({}); db.chat_rooms.deleteMany({});"
```

## 持續集成測試

```bash
# 運行所有測試
go test ./...

# 運行特定測試
go test ./internal/chatroom/...

# 運行集成測試
go test ./tests/...
```
