# Chat Gateway - 聊天室微服務

一個基於 gRPC 的企業級聊天室微服務，提供端到端加密、即時通訊、群組管理等完整功能。

## 目錄

- [系統架構](#系統架構)
- [核心功能](#核心功能)
- [技術棧](#技術棧)
- [快速開始](#快速開始)
- [配置說明](#配置說明)
- [API 文檔](#api-文檔)
- [安全特性](#安全特性)
- [開發指南](#開發指南)
- [測試](#測試)
- [部署](#部署)
- [常見問題](#常見問題)

## 系統架構

### 服務設計

Chat Gateway 採用分層架構設計，核心為 gRPC 服務，支持多種客戶端接入：

```
┌──────────────────┐
│   客戶端應用     │
│  (Web/Mobile)    │
└─────────┬────────┘
          │
          ↓
┌──────────────────┐         ┌──────────────────┐
│   HTTP Gateway   │←────────│  其他微服務      │
│   (Port 8080)    │         │  (直接 gRPC)     │
└─────────┬────────┘         └────────┬─────────┘
          │                           │
          ↓                           ↓
┌──────────────────────────────────────────────┐
│          gRPC Chat Service (Port 8081)       │
│  ┌──────────────┐  ┌───────────────────┐    │
│  │ Room Service │  │ Message Service   │    │
│  └──────────────┘  └───────────────────┘    │
│  ┌──────────────┐  ┌───────────────────┐    │
│  │ Key Manager  │  │  Audit Service    │    │
│  └──────────────┘  └───────────────────┘    │
└───────────────────┬──────────────────────────┘
                    │
                    ↓
        ┌───────────────────────┐
        │      MongoDB          │
        │  ┌─────────────────┐  │
        │  │   chatrooms     │  │
        │  │   messages      │  │
        │  │   encryption_keys│ │
        │  └─────────────────┘  │
        └───────────────────────┘
```

### 數據流

1. **消息發送流程**
   - 客戶端 -> HTTP API
   - HTTP -> gRPC Service
   - 消息加密 (AES-256-CTR)
   - 存儲到 MongoDB
   - SSE 實時推送給訂閱者

2. **密鑰管理流程**
   - Master Key (環境變量)
   - Room Key 生成 (每個聊天室獨立密鑰)
   - Key 用 Master Key 加密後存儲
   - 支持密鑰輪替和版本管理

## 核心功能

### 聊天室管理

#### 1. 聊天室類型
- **一對一聊天** (direct)
  - 自動防重複創建
  - 兩人之間只能有一個一對一聊天室
  
- **群組聊天** (group)
  - 支持多人群組
  - 成員管理（添加/移除/退出）
  - 群組設置可配置

#### 2. 聊天室功能
- 創建聊天室
- 列出用戶聊天室（支持分頁和游標）
- 獲取聊天室詳情
- 添加/移除成員
- 加入/離開群組
- 系統訊息（加入/離開通知）

### 消息功能

#### 1. 消息類型
- 文字消息
- 圖片（預留）
- 文件（預留）
- 語音（預留）
- 系統消息

#### 2. 消息操作
- 發送消息（端到端加密）
- 獲取消息歷史（支持分頁）
- 實時消息推送（Server-Sent Events）
- 標記已讀/送達
- 未讀數量統計

#### 3. 消息狀態
- 已發送
- 已送達
- 已讀（顯示已讀用戶列表）

### 安全特性

#### 1. 端到端加密 (A 級安全)
- **加密算法**: AES-256-CTR
- **密鑰管理**: 每個聊天室獨立密鑰
- **密鑰存儲**: Master Key 加密後存儲於 MongoDB
- **密鑰輪替**: 支持自動和手動輪替（使用事務保證原子性）
- **版本管理**: 保留歷史密鑰用於解密舊消息
- **並發安全**: Double-Check Locking 防止競爭條件
- **內存保護**: 敏感數據自動清零
- **防禦性編程**: Master Key 和密鑰防禦性複製

#### 2. 傳輸安全
- gRPC TLS 支持（可選）
- MongoDB TLS 連接（可選）
- HTTP Security Headers
  - X-Frame-Options
  - X-Content-Type-Options
  - Content-Security-Policy
  - Referrer-Policy

#### 3. 訪問控制
- CORS 白名單
- Rate Limiting（可配置）
  - 全局限制
  - 端點級別限制
  - IP 級別限制
- SSE 連接限制
  - 每 IP 最大連接數
  - 連接間隔限制
  - 全局連接數限制

#### 4. 輸入驗證
- Room ID 驗證（MongoDB ObjectID）
- User ID 驗證（長度、特殊字符）
- 消息內容驗證（長度、NULL 字符）
- MongoDB 查詢防注入

#### 5. 審計日誌
- 所有關鍵操作記錄
- 包含 IP、User-Agent、Request ID
- 結構化 JSON 格式
- 支持 GCP Cloud Logging
- 無敏感信息洩露（統一錯誤消息）

#### 6. 安全增強（2025-10 更新）
- **並發控制**: Double-Check Locking 模式
- **事務支持**: 密鑰輪替使用 MongoDB 事務
- **內存安全**: 
  - 密鑰生成後自動清零
  - 加密/解密緩衝區清零
  - 防止內存 dump 洩露
- **防禦性複製**: 
  - Master Key 複製防止外部修改
  - 返回密鑰副本而非引用
- **錯誤處理**: 
  - 統一錯誤消息格式
  - 詳細錯誤記錄到日誌
  - 客戶端僅收到通用錯誤

## 技術棧

### 後端
- **語言**: Go 1.21+
- **框架**: 
  - gRPC (服務間通訊)
  - Gin (HTTP Gateway)
- **數據庫**: MongoDB v2
- **配置管理**: Viper
- **日誌**: 自定義 Logger (GCP Cloud Logging 格式)

### 安全
- **加密**: AES-256-CTR
- **密鑰管理**: 自建 KeyManager with Persistence
- **審計**: 自建 Audit Service
- **TLS**: Go crypto/tls

### 開發工具
- Protocol Buffers (Proto3)
- Go Modules
- MongoDB Driver v2

## 快速開始

### 前置需求

- Go 1.21 或更高版本
- MongoDB 4.4 或更高版本
- Protocol Buffers Compiler

### 安裝

1. 克隆專案
```bash
git clone <repository-url>
cd chat-gateway
```

2. 安裝依賴
```bash
go mod download
```

3. 生成 Proto 文件（如果修改了 proto）
```bash
./scripts/generate_proto.sh
```

4. 配置環境變量
```bash
# MongoDB 連接
export MONGO_URI="mongodb://localhost:27017"
export MONGO_DATABASE="chatroom"

# 加密（生產環境必須設置）
export MASTER_KEY=$(openssl rand -base64 32)

# 可選：啟用金鑰輪替
export KEY_ROTATION_ENABLED=true
```

5. 啟動服務
```bash
# 編譯
go build -o bin/chat-gateway cmd/api/main.go

# 運行
./bin/chat-gateway
```

服務將在以下端口啟動：
- gRPC Server: `localhost:8081`
- HTTP Gateway: `localhost:8080`

### 測試

使用提供的測試前端：
```bash
# 直接打開 web/index.html
open web/index.html
```

或使用 Live Server (VSCode):
1. 安裝 Live Server 擴展
2. 右鍵 `web/index.html` -> Open with Live Server

## 配置說明

### 配置文件

主配置文件：`configs/local.yaml`

```yaml
app:
  name: "ChatGateway"
  env: "local"
  debug: true

server:
  host: "0.0.0.0"
  port: 8080
  read_timeout: 30
  write_timeout: 30

grpc:
  host: "localhost"
  port: 8081

database:
  mongo:
    host: "localhost"
    port: 27017
    database: "chatroom"
    connect_timeout: 10
    # TLS 配置（可選）
    tls_enabled: false
    tls_ca_file: ""
    tls_cert_file: ""
    tls_key_file: ""

security:
  encryption:
    enabled: true
  audit:
    enabled: true
  # TLS 配置（可選）
  tls:
    enabled: false
    cert_file: ""
    key_file: ""
    ca_file: ""

limits:
  # 請求限制
  request:
    max_body_size: 10485760      # 10MB
    max_multipart_memory: 10485760

  # Rate Limiting（開發環境超寬鬆，幾乎不限制）
  rate_limiting:
    enabled: true
    default_per_minute: 10000         # 全局限制（每秒 166 個請求，超級寬鬆）
    messages_per_minute: 1000         # 發送訊息（每秒 16 條，隨便打字）
    rooms_per_minute: 500             # 創建聊天室（每分鐘 500 個）
    sse_per_minute: 10000             # SSE 連接（基本無限制）

  # SSE 連接限制（開發環境超寬鬆）
  sse:
    max_connections_per_ip: 50        # 每個 IP 最大連接數（隨便開標籤頁）
    max_total_connections: 100000     # 全局最大連接數（10 萬，基本無限）
    min_connection_interval_seconds: 0  # 最小連接間隔（0，完全不限制）
    heartbeat_interval_seconds: 15    # 心跳間隔
    initial_message_fetch: 100        # 初始訊息抓取數量
    message_channel_buffer: 10        # 訊息通道緩衝區大小

  # 分頁限制
  pagination:
    default_page_size: 20
    max_page_size: 100
    max_history_size: 50

  # 聊天室限制
  room:
    max_members: 1000
    max_name_length: 100

  # 消息限制
  message:
    max_length: 10000

  # MongoDB 查詢限制
  mongodb:
    default_query_limit: 20
    max_query_limit: 100
    max_history_limit: 50
```

### 環境變量

優先級：環境變量 > 配置文件

必需：
- `MONGO_URI` 或 `MONGO_HOST` + `MONGO_PORT`
- `MONGO_DATABASE`
- `MASTER_KEY` (生產環境)

可選：
- `MONGO_USERNAME`
- `MONGO_PASSWORD`
- `KEY_ROTATION_ENABLED`
- `GIN_MODE` (release/debug)

## API 文檔

### HTTP API

#### 聊天室

**創建聊天室**
```http
POST /api/v1/rooms
Content-Type: application/json

{
    "name": "測試群組",
    "type": "group",
    "owner_id": "user_alice",
  "members": ["user_alice", "user_bob"]
}
```

**列出聊天室**
```http
GET /api/v1/rooms?user_id=user_alice&limit=20&cursor=
```

**添加成員**
```http
POST /api/v1/rooms/:room_id/members
Content-Type: application/json

{
  "user_id": "user_charlie"
}
```

**移除成員**
```http
DELETE /api/v1/rooms/:room_id/members/:user_id
```

#### 消息

**發送消息**
```http
POST /api/v1/messages
Content-Type: application/json

{
  "room_id": "507f1f77bcf86cd799439011",
    "sender_id": "user_alice",
    "content": "Hello!",
    "type": "text"
}
```

**獲取消息**
```http
GET /api/v1/messages?room_id=507f1f77bcf86cd799439011&user_id=user_alice&limit=20&cursor=
```

**標記已讀**
```http
POST /api/v1/messages/read
Content-Type: application/json

{
  "room_id": "507f1f77bcf86cd799439011",
  "user_id": "user_alice"
}
```

**SSE 訂閱（實時消息）**
```http
GET /api/v1/messages/stream?room_id=507f1f77bcf86cd799439011&user_id=user_alice
```

### gRPC API

參見 `proto/chat.proto` 文件

主要服務：
- `ChatRoomService.CreateRoom`
- `ChatRoomService.ListUserRooms`
- `ChatRoomService.JoinRoom`
- `ChatRoomService.LeaveRoom`
- `ChatRoomService.SendMessage`
- `ChatRoomService.GetMessages`
- `ChatRoomService.MarkAsRead`
- `ChatRoomService.GetRoomInfo`
- `ChatRoomService.StreamMessages`
- `ChatRoomService.GetUnreadCount`

## 安全特性

### 密鑰管理

#### Master Key
- 256-bit AES 密鑰
- 從環境變量 `MASTER_KEY` 讀取
- 用於加密所有 Room Key
- **生產環境必須設置**
- **防禦性複製**：初始化時複製防止外部修改

生成 Master Key：
```bash
export MASTER_KEY=$(openssl rand -base64 32)
```

#### Room Key
- 每個聊天室獨立的 256-bit AES 密鑰
- 首次發送消息時自動生成
- 用 Master Key 加密後存儲於 MongoDB
- 支持版本管理和輪替
- **並發安全**：使用 Double-Check Locking 防止重複創建
- **內存保護**：使用後自動清零
- **安全返回**：返回副本而非引用

#### 密鑰輪替

自動輪替（可選）：
```bash
export KEY_ROTATION_ENABLED=true
```

配置策略：
- 輪替間隔：24 小時（默認）
- 密鑰最大年齡：30 天（默認）
- 保留歷史密鑰：5 個版本（默認）

手動輪替：
```go
keyManager.ForceRotateKey(roomID)
```

**事務保證**：密鑰輪替使用 MongoDB 事務確保原子性：
1. 標記舊密鑰為非活躍
2. 插入新密鑰
3. 兩步驟在同一事務中完成，避免數據不一致

#### 密鑰持久化

- **存儲位置**：MongoDB `encryption_keys` 集合
- **加密方式**：Room Key 用 Master Key 加密（AES-256-CTR）
- **啟動加載**：服務啟動時從 DB 加載所有密鑰到內存
- **三層緩存**：
  1. 內存緩存（`keys` map）
  2. 數據庫持久化（`encryption_keys` 集合）
  3. 歷史密鑰緩存（`oldKeys` map）

#### 安全增強（2025-10）

1. **並發控制**
   - Double-Check Locking 模式
   - 讀鎖（快速路徑）+ 寫鎖（慢速路徑）
   - 防止競爭條件

2. **內存安全**
   - 密鑰生成後使用 `defer` 自動清零
   - 加密/解密緩衝區清零
   - 舊密鑰歸檔前清零
   - 防止內存 dump 洩露

3. **防禦性複製**
   - Master Key 初始化時複製
   - 返回密鑰副本而非引用
   - 防止外部修改內部狀態

4. **事務支持**
   - 密鑰輪替使用 MongoDB 事務
   - 確保操作原子性
   - 避免短暫無活躍密鑰

### 加密實現

#### 消息加密流程
1. 獲取或創建聊天室密鑰（Double-Check Locking）
2. 使用 AES-256-CTR 加密消息
3. Base64 編碼密文
4. 添加前綴 `aes256ctr:`
5. 存儲到數據庫
6. **內存清零**：明文字節、臨時緩衝區自動清零

#### 消息解密流程
1. 從數據庫讀取加密消息
2. 檢查前綴確認加密格式
3. Base64 解碼
4. 獲取對應版本的密鑰
5. AES-256-CTR 解密
6. **內存清零**：密文數據、解碼後數據自動清零
7. 返回明文

#### 系統消息
系統消息（type=system）不加密，直接存儲明文。

#### 錯誤處理
- **統一錯誤消息**：客戶端僅收到通用錯誤（如 "key generation error"）
- **詳細日誌**：敏感錯誤詳情記錄到日誌，包含 Request ID
- **防信息洩露**：避免洩露系統實現細節

### Rate Limiting

三層限制策略（配置可調整）：

1. **全局限制**
   - 開發環境：10,000 請求/分鐘（超寬鬆）
   - 生產環境建議：100 請求/分鐘

2. **端點限制**
   - 發送消息：1,000 次/分鐘（開發）/ 30 次（生產）
   - 創建聊天室：500 次/分鐘（開發）/ 10 次（生產）
   - SSE 連接：10,000 次/分鐘（開發）/ 30 次（生產）

3. **SSE 連接限制**
   - 每 IP 最大連接數：50（開發環境）/ 5（生產環境）
   - 連接間隔：0 秒（開發，無限制）/ 1 秒（生產）
   - 全局最大連接數：100,000（開發）/ 1,000（生產）

**注意**：
- `min_connection_interval_seconds` 可設為 `0` 以允許無限制快速切換
- 配置修改後需重啟服務才會生效
- 開發環境可設置得很寬鬆，生產環境應設置合理限制

### 審計日誌

記錄內容：
- 操作類型（create_room, send_message, etc.）
- 用戶 ID
- 聊天室 ID
- IP 地址
- User-Agent
- 時間戳
- Request ID
- 操作結果

日誌格式：GCP Cloud Logging JSON

## 開發指南

### 項目結構

```
chat-gateway/
├── cmd/
│   └── api/
│       └── main.go           # 應用入口
├── internal/
│   ├── constants/            # 常數定義
│   ├── grpc/                 # gRPC 服務實現
│   ├── grpcclient/           # gRPC 客戶端管理
│   ├── httputil/             # HTTP 工具函數
│   ├── platform/
│   │   ├── config/           # 配置管理
│   │   ├── driver/           # 數據庫驅動
│   │   ├── health/           # 健康檢查
│   │   ├── logger/           # 日誌系統
│   │   ├── middleware/       # 中間件
│   │   └── server/           # HTTP 服務器
│   ├── security/
│   │   ├── audit/            # 審計服務
│   │   ├── encryption/       # 加密服務
│   │   └── keymanager/       # 密鑰管理
│   └── storage/
│       └── database/
│           ├── chatroom/     # 聊天室數據訪問
│           └── security.go   # 數據庫安全
├── proto/                    # Protocol Buffers 定義
├── configs/                  # 配置文件
├── scripts/                  # 工具腳本
├── web/                      # 測試前端
└── tests/                    # 測試文件
```

### 添加新功能

#### 1. 定義 Proto

編輯 `proto/chat.proto`：
```protobuf
service ChatRoomService {
  rpc YourNewMethod(YourRequest) returns (YourResponse);
}

message YourRequest {
  string field = 1;
}

message YourResponse {
  bool success = 1;
  string message = 2;
}
```

#### 2. 生成代碼

```bash
./scripts/generate_proto.sh
```

#### 3. 實現服務

在 `internal/grpc/server.go` 添加：
```go
func (s *Server) YourNewMethod(ctx context.Context, req *chat.YourRequest) (*chat.YourResponse, error) {
    // 實現邏輯
    return &chat.YourResponse{
        Success: true,
        Message: "success",
    }, nil
}
```

#### 4. 添加 HTTP 端點（如需要）

在 `internal/platform/server/http.go` 添加路由和處理器。

### 代碼規範

- 使用 `gofmt` 格式化代碼
- 函數和類型添加註釋
- 錯誤處理要完整
- 使用結構化日誌
- 敏感信息不要記錄到日誌

### 日誌規範

使用統一的 logger：
```go
logger.Info(ctx, "操作描述",
    logger.WithUserID(userID),
    logger.WithRoomID(roomID),
    logger.WithAction("action_name"),
    logger.WithDetails(map[string]interface{}{
        "key": "value",
    }))
```

## 測試

參見 `tests/README.md` 獲取詳細測試文檔和輔助工具。

### 單元測試

```bash
# 運行所有測試
go test ./...

# 運行特定包的測試
go test ./internal/security/encryption/...

# 查看覆蓋率
go test -cover ./...
```

### 集成測試

```bash
# 確保 MongoDB 運行
docker-compose up -d mongo

# 運行集成測試
go test -tags=integration ./tests/...
```

### 手動測試

使用提供的測試前端 `web/index.html` 進行端到端測試。

## 部署

### Docker 部署（TODO）

```bash
# 構建鏡像
docker build -t chat-gateway:latest .

# 運行容器
docker run -d \
  -p 8080:8080 \
  -p 8081:8081 \
  -e MONGO_URI=mongodb://mongo:27017 \
  -e MASTER_KEY=your_master_key \
  chat-gateway:latest
```

### Kubernetes 部署（TODO）

參見 `k8s/` 目錄中的配置文件。

### 生產環境檢查清單

- [ ] 設置 `MASTER_KEY` 環境變量
- [ ] 配置 MongoDB 認證
- [ ] 啟用 TLS（gRPC 和 MongoDB）
- [ ] 配置 CORS 白名單
- [ ] 設置適當的 Rate Limiting
- [ ] 啟用審計日誌
- [ ] 配置日誌收集（Filebeat, Fluentd 等）
- [ ] 設置監控告警
- [ ] 定期備份數據庫
- [ ] 配置密鑰輪替策略
- [ ] 設置 `GIN_MODE=release`

## 常見問題

### Q: 如何重置所有數據？

```bash
mongosh
use chatroom
db.dropDatabase()
```

### Q: 消息顯示「訊息格式錯誤」？

原因：Master Key 改變導致無法解密舊消息。

解決：
1. 使用固定的 Master Key（生產環境必須）
2. 或清空數據庫重新開始

### Q: SSE 連接一直 429 錯誤？

原因：觸發了 Rate Limiting 或連接間隔限制。

解決方法：

1. **調整配置** (`configs/local.yaml`)：
```yaml
limits:
  sse:
    min_connection_interval_seconds: 0  # 設為 0 允許無限制切換
    max_connections_per_ip: 50          # 增加連接限制
    max_total_connections: 100000       # 增加全局限制
```

2. **重啟服務**：配置修改後必須重啟後端服務才會生效

3. **檢查後端日誌**：確認配置是否正確載入

注意：開發環境可以設置得很寬鬆，但生產環境應該設置合理的限制以防止 DDoS 攻擊。

### Q: 如何查看密鑰？

```bash
mongosh
use chatroom
db.encryption_keys.find().pretty()
```

注意：密鑰是加密存儲的，無法直接查看明文。

### Q: 如何手動輪替密鑰？

目前需要通過代碼調用：
```go
keyManager.ForceRotateKey(roomID)
```

未來會添加管理 API。

### Q: 為什麼重啟後可以解密舊消息？

因為密鑰持久化在 MongoDB 中：
1. 啟動時自動從 DB 加載密鑰
2. 使用相同的 Master Key 解密 Room Key
3. 因此可以解密所有歷史消息

## 授權

MIT License

## 聯繫方式

- 問題反饋：提交 Issue
- 功能建議：提交 Feature Request
