# 測試指南 - 快速開始

本指南提供快速測試 Chat Gateway 的步驟和常見場景。

## 快速開始

### 1. 環境準備

```bash
# 啟動 MongoDB
docker run -d --name mongo-dev -p 27017:27017 mongo:latest

# 設置環境變量
export MONGO_URI="mongodb://localhost:27017"
export MONGO_DATABASE="chatroom"
export MASTER_KEY=$(openssl rand -base64 32)
```

### 2. 啟動服務

```bash
# 編譯
go build -o bin/chat-gateway cmd/api/main.go

# 運行
./bin/chat-gateway
```

### 3. 使用測試前端

```bash
# 使用瀏覽器打開
open web/index.html

# 或使用 Live Server (VSCode)
# 右鍵 web/index.html -> Open with Live Server
```

## 測試場景

### 場景 1：一對一聊天

#### 步驟：

1. **登入用戶 A (Alice)**
   - 在測試前端選擇用戶：Alice
   - 重新整理頁面

2. **創建一對一聊天**
   - 點擊「創建聊天室」
   - 選擇類型：一對一
   - 選擇對方：Bob
   - 點擊「創建」

3. **發送消息**
   - 輸入消息：「Hi Bob!」
   - 按 Enter 或點擊發送

4. **切換到用戶 B (Bob)**
   - 選擇用戶：Bob
   - 重新整理頁面
   - 應該看到來自 Alice 的聊天室
   - 點擊進入聊天室

5. **驗證**
   - Bob 應該能看到 Alice 的消息
   - 未讀數字應該顯示
   - 點擊進入後未讀數字應該消失
   - Alice 那邊應該看到「已讀」狀態

### 場景 2：群組聊天

#### 步驟：

1. **創建群組**
   - 用戶：Alice
   - 點擊「創建聊天室」
   - 選擇類型：群組
   - 輸入名稱：「測試群組」
   - 選擇成員：Bob, Charlie, David
   - 點擊「創建」

2. **發送群組消息**
   - Alice 發送：「大家好！」
   - 消息應該立即顯示

3. **其他成員接收**
   - 切換到 Bob
   - 應該看到新的群組
   - 點擊進入
   - 看到 Alice 的消息

4. **多人已讀**
   - Bob 標記已讀
   - 切換回 Alice
   - 應該看到 Bob 已讀

### 場景 3：即時消息（SSE）

#### 步驟：

1. **開啟兩個瀏覽器窗口**
   - 窗口 A：Alice
   - 窗口 B：Bob

2. **兩邊都進入同一個聊天室**

3. **在窗口 A 發送消息**
   - Alice 輸入並發送消息

4. **驗證即時推送**
   - 窗口 B 應該立即收到消息
   - 不需要刷新頁面

### 場景 4：成員管理

#### 步驟：

1. **添加成員**
   - 進入群組聊天室
   - 點擊「成員」按鈕
   - 點擊「添加成員」
   - 選擇新成員
   - 確認添加
   - 應該看到系統消息：「XXX 已加入群組」

2. **移除成員**
   - 點擊成員列表中的「移除」
   - 確認移除
   - 應該看到系統消息：「XXX 已離開群組」

### 場景 5：消息加密

#### 驗證加密：

1. **發送消息**
   - 發送任意消息

2. **查看數據庫**
```bash
mongosh
use chatroom
db.messages.find().pretty()
```

3. **驗證**
   - `content` 字段應該是 `aes256ctr:` 開頭的加密字符串
   - 不是明文

4. **查看密鑰**
```bash
db.encryption_keys.find().pretty()
```

5. **驗證**
   - 每個聊天室有獨立的密鑰
   - `encrypted_key` 是用 Master Key 加密的

### 場景 6：密鑰輪替

#### 步驟：

1. **啟用自動輪替**
```bash
export KEY_ROTATION_ENABLED=true
./bin/chat-gateway
```

2. **查看初始密鑰**
```bash
mongosh
use chatroom
db.encryption_keys.find({room_id: "YOUR_ROOM_ID"}).pretty()
```

記錄 `key_version` 值

3. **等待輪替**（24小時後，測試時可以修改配置縮短時間）

4. **驗證新版本**
- 應該有新的 `key_version: 2`
- 舊密鑰 `is_active: false`
- 新密鑰 `is_active: true`

5. **驗證舊消息仍可解密**
- 打開聊天室
- 舊消息應該正常顯示
- 不會顯示「解密失敗」

### 場景 7：重啟後數據持久化

#### 步驟：

1. **發送加密消息**
   - 發送幾條測試消息

2. **記錄 Master Key**
```bash
echo $MASTER_KEY
```

3. **停止服務器**
```bash
# Ctrl+C
```

4. **使用相同 Master Key 重啟**
```bash
export MASTER_KEY="<之前的值>"
./bin/chat-gateway
```

5. **驗證**
   - 重新打開測試前端
   - 進入聊天室
   - 所有歷史消息應該正常顯示
   - 密鑰已從 MongoDB 加載

## 性能測試

### 測試消息吞吐量

使用 Apache Bench:

```bash
# 準備測試數據
cat > message.json << EOF
{
  "room_id": "YOUR_ROOM_ID",
  "sender_id": "user_test",
  "content": "Performance test message",
  "type": "text"
}
EOF

# 運行測試
ab -n 1000 -c 10 \
   -p message.json \
   -T application/json \
   http://localhost:8080/api/v1/messages
```

### 測試 SSE 連接

```bash
# 使用 curl 測試 SSE
curl -N http://localhost:8080/api/v1/messages/stream?room_id=YOUR_ROOM_ID&user_id=user_test
```

應該看到：
- 立即收到 `connected` 事件
- 每 15 秒收到 `ping` 事件
- 有新消息時收到 `message` 事件

## API 測試

### 使用 curl

#### 創建聊天室

```bash
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Test Room",
    "type": "group",
    "owner_id": "user_alice",
    "members": ["user_alice", "user_bob"]
  }'
```

#### 發送消息

```bash
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "room_id": "YOUR_ROOM_ID",
    "sender_id": "user_alice",
    "content": "Hello from API!",
    "type": "text"
  }'
```

#### 獲取消息

```bash
curl "http://localhost:8080/api/v1/messages?room_id=YOUR_ROOM_ID&user_id=user_alice&limit=20"
```

#### 標記已讀

```bash
curl -X POST http://localhost:8080/api/v1/messages/read \
  -H "Content-Type: application/json" \
  -d '{
    "room_id": "YOUR_ROOM_ID",
    "user_id": "user_alice"
  }'
```

### 使用 Postman

1. 導入 API Collection（TODO: 提供 Postman Collection）
2. 設置環境變量：
   - `base_url`: http://localhost:8080
   - `room_id`: 你的聊天室 ID
3. 依序執行請求

## 故障排除

### 問題：前端無法連接

**症狀**：
- 無法載入聊天室列表
- Console 顯示 CORS 錯誤

**解決**：
1. 檢查服務器是否運行：
```bash
curl http://localhost:8080/health
```

2. 檢查 CORS 設置：
   - 確認前端 URL 在白名單中
   - 查看 `internal/platform/server/http.go` 中的 `allowedOrigins`

### 問題：消息顯示「解密失敗」

**症狀**：
- 消息內容顯示 `[解密失敗]`

**可能原因**：
1. Master Key 改變了
2. 密鑰尚未生成

**解決**：
1. 使用固定的 Master Key：
```bash
export MASTER_KEY="固定的base64字符串"
```

2. 或清空數據庫重新開始：
```bash
mongosh
use chatroom
db.dropDatabase()
```

### 問題：SSE 連接 429 錯誤

**症狀**：
- Console 顯示 `429 Too Many Requests`

**原因**：
- 觸發了 Rate Limiting

**解決**：
1. 調整配置 `configs/local.yaml`：
```yaml
limits:
  sse:
    min_connection_interval_seconds: 1
    max_connections_per_ip: 10
    sse_per_minute: 50
```

2. 重啟服務器

### 問題：消息不即時更新

**症狀**：
- 需要刷新才能看到新消息

**檢查**：
1. SSE 是否正常連接：
   - 打開 DevTools -> Network
   - 查看 `stream` 請求
   - Status 應該是 `200` 並保持連接

2. 查看 Console 是否有錯誤

3. 檢查服務器日誌

### 問題：MongoDB 連接失敗

**症狀**：
- 服務器啟動失敗
- 日誌顯示 MongoDB 錯誤

**解決**：
1. 檢查 MongoDB 是否運行：
```bash
docker ps | grep mongo
```

2. 測試連接：
```bash
mongosh $MONGO_URI
```

3. 檢查環境變量：
```bash
echo $MONGO_URI
echo $MONGO_DATABASE
```

## 測試檢查清單

使用此清單確保完整測試：

### 基本功能
- [ ] 創建一對一聊天室
- [ ] 創建群組聊天室
- [ ] 防止重複創建一對一聊天室
- [ ] 發送文字消息
- [ ] 接收消息
- [ ] 標記已讀
- [ ] 查看未讀數量
- [ ] 列出聊天室
- [ ] 聊天室預覽顯示最後一條消息

### 群組功能
- [ ] 添加群組成員
- [ ] 移除群組成員
- [ ] 離開群組
- [ ] 系統消息顯示正確

### 即時功能
- [ ] SSE 連接建立
- [ ] 即時接收消息
- [ ] 即時更新已讀狀態
- [ ] SSE 心跳正常
- [ ] SSE 重連機制

### 安全功能
- [ ] 消息已加密存儲
- [ ] 消息可正常解密
- [ ] 系統消息不加密
- [ ] 密鑰持久化
- [ ] 重啟後可解密舊消息
- [ ] Rate Limiting 生效

### 性能
- [ ] 加載 20 條消息響應快速（< 500ms）
- [ ] 發送消息響應快速（< 200ms）
- [ ] SSE 推送延遲低（< 100ms）
- [ ] 快速切換聊天室不卡頓

### 錯誤處理
- [ ] 無效 Room ID 返回錯誤
- [ ] 無效 User ID 返回錯誤
- [ ] 超長消息被拒絕
- [ ] MongoDB 斷線後可恢復

## 自動化測試腳本

### 完整測試腳本

`scripts/test_all.sh`:

```bash
#!/bin/bash

set -e

echo "=== 啟動測試環境 ==="
docker-compose -f docker-compose.test.yml up -d

echo "=== 等待 MongoDB 就緒 ==="
sleep 5

echo "=== 設置環境變量 ==="
export MONGO_URI="mongodb://localhost:27018"
export MONGO_DATABASE="chatroom_test"
export MASTER_KEY=$(openssl rand -base64 32)
export GIN_MODE=test

echo "=== 運行單元測試 ==="
go test -v -cover ./...

echo "=== 運行集成測試 ==="
go test -tags=integration -v ./tests/integration/...

echo "=== 生成覆蓋率報告 ==="
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

echo "=== 清理測試環境 ==="
docker-compose -f docker-compose.test.yml down

echo "=== 測試完成 ==="
echo "覆蓋率報告: coverage.html"
```

使用：
```bash
chmod +x scripts/test_all.sh
./scripts/test_all.sh
```

## 參考

- 詳細測試文檔：`TESTING.md`
- API 文檔：`README.md#api-文檔`
- 配置說明：`README.md#配置說明`
