# 聊天室測試指南

本指南將幫助你測試聊天室的一對一和群組聊天功能。

## 目錄

1. [環境準備](#環境準備)
2. [啟動服務](#啟動服務)
3. [測試方法](#測試方法)
   - [方法一：使用測試腳本](#方法一使用測試腳本)
   - [方法二：使用網頁介面](#方法二使用網頁介面)
   - [方法三：使用 WebSocket 工具](#方法三使用-websocket-工具)
4. [測試場景](#測試場景)
5. [故障排除](#故障排除)

## 環境準備

### 安裝必要工具

```bash
# 安裝 grpcurl (用於 gRPC 測試)
brew install grpcurl

# 安裝 wscat (用於 WebSocket 測試)
npm install -g wscat

# 安裝 jq (用於 JSON 處理)
brew install jq
```

### 檢查依賴

確保你的 Go 環境已正確設置，並且 MongoDB 正在運行。

## 啟動服務

1. **啟動聊天服務**：
   ```bash
   cd /Users/wayne_chen/_project/chat-gateway
   go run cmd/api/main.go
   ```

2. **驗證服務運行**：
   - HTTP API: http://localhost:8080/health
   - gRPC: localhost:8081
   - WebSocket: ws://localhost:8080/ws

## 測試方法

### 方法一：使用測試腳本

#### 1. gRPC 測試

```bash
# 運行完整的 gRPC 測試
./scripts/test_grpc.sh

# 測試流式消息（需要先獲取房間 ID）
./scripts/test_grpc.sh stream <room_id>
```

#### 2. HTTP API 測試

```bash
# 運行完整的 HTTP API 測試
./scripts/test_chat.sh
```

#### 3. WebSocket 測試

```bash
# 運行完整的 WebSocket 測試
./scripts/test_websocket.sh test

# 創建測試聊天室
./scripts/test_websocket.sh create direct user_alice
./scripts/test_websocket.sh create group user_alice

# 連接到聊天室
./scripts/test_websocket.sh connect <room_id> <user_id>

# 發送測試消息
./scripts/test_websocket.sh send <room_id> <user_id> "測試消息"
```

### 方法二：使用網頁介面

1. **打開網頁介面**：
   ```bash
   open web/index.html
   ```
   或者直接在瀏覽器中打開 `web/index.html`

2. **功能說明**：
   - 選擇用戶身份（Alice, Bob, Charlie, David）
   - 查看現有聊天室列表
   - 創建新的聊天室（一對一或群組）
   - 加入聊天室並發送消息
   - 即時接收消息（通過 WebSocket）

3. **測試步驟**：
   - 創建一個一對一聊天室
   - 創建一個群組聊天室
   - 在不同用戶身份間切換
   - 發送消息並觀察即時更新

### 方法三：使用 WebSocket 工具

#### 使用 wscat 直接連接

```bash
# 連接到一對一聊天室
wscat -c "ws://localhost:8080/ws?room_id=<room_id>&user_id=user_alice"

# 連接到群組聊天室
wscat -c "ws://localhost:8080/ws?room_id=<room_id>&user_id=user_bob"
```

## 測試場景

### 場景一：一對一聊天測試

1. **創建一對一聊天室**：
   ```bash
   curl -X POST "http://localhost:8080/api/v1/rooms" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "Alice 和 Bob 的私聊",
       "type": "direct",
       "owner_id": "user_alice",
       "members": [
         {"user_id": "user_alice", "role": "admin"},
         {"user_id": "user_bob", "role": "member"}
       ]
     }'
   ```

2. **發送消息**：
   ```bash
   curl -X POST "http://localhost:8080/api/v1/messages" \
     -H "Content-Type: application/json" \
     -d '{
       "room_id": "<room_id>",
       "sender_id": "user_alice",
       "content": "你好 Bob！",
       "type": "text"
     }'
   ```

3. **獲取消息**：
   ```bash
   curl "http://localhost:8080/api/v1/messages?room_id=<room_id>&user_id=user_alice&limit=10"
   ```

### 場景二：群組聊天測試

1. **創建群組聊天室**：
   ```bash
   curl -X POST "http://localhost:8080/api/v1/rooms" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "開發團隊討論組",
       "type": "group",
       "owner_id": "user_alice",
       "members": [
         {"user_id": "user_alice", "role": "admin"},
         {"user_id": "user_bob", "role": "member"},
         {"user_id": "user_charlie", "role": "member"},
         {"user_id": "user_david", "role": "member"}
       ]
     }'
   ```

2. **多用戶發送消息**：
   ```bash
   # Alice 發送消息
   curl -X POST "http://localhost:8080/api/v1/messages" \
     -H "Content-Type: application/json" \
     -d '{
       "room_id": "<room_id>",
       "sender_id": "user_alice",
       "content": "大家好！",
       "type": "text"
     }'

   # Bob 發送消息
   curl -X POST "http://localhost:8080/api/v1/messages" \
     -H "Content-Type: application/json" \
     -d '{
       "room_id": "<room_id>",
       "sender_id": "user_bob",
       "content": "Hi Alice！",
       "type": "text"
     }'
   ```

### 場景三：即時消息測試

1. **開啟兩個終端**：
   - 終端 1：連接到聊天室
   - 終端 2：發送消息

2. **終端 1 - 監聽消息**：
   ```bash
   wscat -c "ws://localhost:8080/ws?room_id=<room_id>&user_id=user_alice"
   ```

3. **終端 2 - 發送消息**：
   ```bash
   curl -X POST "http://localhost:8080/api/v1/messages" \
     -H "Content-Type: application/json" \
     -d '{
       "room_id": "<room_id>",
       "sender_id": "user_bob",
       "content": "即時消息測試！",
       "type": "text"
     }'
   ```

4. **觀察結果**：終端 1 應該即時收到消息

## 故障排除

### 常見問題

1. **服務無法啟動**：
   - 檢查端口是否被占用
   - 確認 MongoDB 正在運行
   - 檢查配置文件

2. **WebSocket 連接失敗**：
   - 確認 HTTP 服務正在運行
   - 檢查 WebSocket 端點是否正確
   - 查看服務日誌

3. **消息發送失敗**：
   - 確認房間 ID 和用戶 ID 正確
   - 檢查用戶是否在房間中
   - 查看 API 響應錯誤信息

4. **網頁介面無法載入**：
   - 確認服務正在運行
   - 檢查瀏覽器控制台錯誤
   - 確認 CORS 設置正確

### 日誌查看

```bash
# 查看應用日誌
tail -f logs/app.log

# 查看實時日誌
go run cmd/api/main.go 2>&1 | tee -a logs/app.log
```

### 調試模式

```bash
# 啟用調試模式
export DEBUG=true
go run cmd/api/main.go
```

## 測試檢查清單

- [ ] 服務正常啟動
- [ ] 一對一聊天室創建成功
- [ ] 群組聊天室創建成功
- [ ] 消息發送成功
- [ ] 消息接收成功
- [ ] WebSocket 連接正常
- [ ] 即時消息推送正常
- [ ] 多用戶同時在線正常
- [ ] 消息歷史查詢正常
- [ ] 網頁介面功能正常

## 進階測試

### 壓力測試

```bash
# 使用 Apache Bench 進行壓力測試
ab -n 1000 -c 10 -H "Content-Type: application/json" \
   -p test_message.json \
   http://localhost:8080/api/v1/messages
```

### 並發測試

```bash
# 同時開啟多個 WebSocket 連接
for i in {1..10}; do
  wscat -c "ws://localhost:8080/ws?room_id=<room_id>&user_id=user_$i" &
done
```

### 長時間運行測試

```bash
# 運行長時間測試
./scripts/test_websocket.sh connect <room_id> user_alice
# 保持連接並觀察穩定性
```

## 總結

這個測試指南提供了多種方式來測試聊天室功能：

1. **自動化測試**：使用腳本進行快速功能驗證
2. **手動測試**：使用網頁介面進行直觀測試
3. **技術測試**：使用命令行工具進行深度測試

建議按照以下順序進行測試：
1. 先運行自動化測試腳本
2. 使用網頁介面進行用戶體驗測試
3. 使用 WebSocket 工具進行技術驗證
4. 進行壓力測試和長時間運行測試

這樣可以確保聊天室功能在各種場景下都能正常工作。
