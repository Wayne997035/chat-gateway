# Chat Gateway - 聊天室微服務

一個基於 gRPC 的聊天室微服務，提供一對一和群組聊天功能，類似 Facebook Messenger / LINE 的聊天體驗。

## 🎯 專案特色

- **gRPC 優先設計** - 核心服務採用 gRPC，提供高效能的服務間通訊
- **防重複聊天室** - 自動檢測並防止創建重複的一對一聊天室
- **即時已讀狀態** - 支援一對一和群組的已讀/送達狀態追蹤
- **成員管理** - 完整的群組成員管理（添加/移除/退出）
- **MongoDB 儲存** - 使用 MongoDB 進行持久化儲存，支援高效查詢
- **🔒 安全功能** - 消息加密、審計日誌、TLS 支援（詳見安全章節）
- **📊 GCP Cloud Logging** - 結構化 JSON 日誌，支援 trace ID 追蹤

## 🏗️ 系統架構

### 核心服務

本專案的核心是 **gRPC 服務**，其他系統可以直接調用 gRPC 接口進行通訊。

```
┌─────────────┐
│ 其他微服務  │ ──→ gRPC (Port 8081)
└─────────────┘

┌─────────────┐
│ Web前端     │ ──→ HTTP API Bridge (Port 8080) ──→ gRPC (Port 8081)
└─────────────┘                                     ↓
                                                MongoDB
```

### 已實現功能

#### gRPC 服務 (核心)
1. ✅ **CreateRoom** - 創建聊天室（一對一/群組）
2. ✅ **ListUserRooms** - 列出用戶的聊天室
3. ✅ **JoinRoom** - 加入聊天室
4. ✅ **LeaveRoom** - 離開聊天室
5. ✅ **SendMessage** - 發送消息
6. ✅ **GetMessages** - 獲取消息列表
7. ✅ **MarkAsRead** - 標記消息為已讀
8. ✅ **GetRoomInfo** - 獲取聊天室信息
9. ⏳ **StreamMessages** - 流式獲取消息（待實現）
10. ⏳ **GetUnreadCount** - 獲取未讀數量（待實現）

#### HTTP API Bridge (測試用)
- POST `/api/v1/rooms` - 創建聊天室
- GET `/api/v1/rooms` - 獲取聊天室列表
- POST `/api/v1/rooms/:room_id/members` - 添加成員
- DELETE `/api/v1/rooms/:room_id/members/:user_id` - 移除成員/退出群組
- POST `/api/v1/messages` - 發送消息
- GET `/api/v1/messages` - 獲取消息
- POST `/api/v1/messages/read` - 標記已讀

> **注意**: HTTP API 僅供測試使用，正式環境請使用 gRPC 接口。

### 技術棧

- **後端**: Go 1.24 + gRPC + Protocol Buffers
- **數據庫**: MongoDB 4.4+
- **開發工具**: Air (熱重載)
- **測試工具**: grpcurl, curl
- **消息推送**: HTTP 輪詢 (WebSocket 待實現)

## 📁 專案結構

```
chat-gateway/
├── cmd/
│   ├── api/                    # 應用程式入口
│   │   └── main.go            # 啟動 gRPC 和 HTTP 服務
│   └── test/                   # 測試工具
├── internal/
│   ├── grpc/                  # gRPC 服務實現 ⭐ 核心
│   │   └── server.go          # 聊天室服務邏輯
│   ├── platform/              # 平台層
│   │   ├── config/            # 配置管理
│   │   ├── driver/            # 數據庫驅動 (MongoDB)
│   │   ├── health/            # 健康檢查
│   │   ├── logger/            # 日誌管理
│   │   └── server/            # HTTP 服務器 (API Bridge)
│   ├── storage/               # 數據存儲層
│   │   └── database/
│   │       ├── chatroom/      # 聊天室資料庫操作
│   │       │   ├── chatroom.go    # 聊天室 CRUD
│   │       │   ├── message.go     # 消息 CRUD
│   │       │   └── indexes.go     # 數據庫索引
│   │       └── repositories.go
│   ├── security/              # 安全模組
│   │   ├── encryption/        # 加密實現 (Signal Protocol)
│   │   └── audit/             # 審計日誌
│   └── message/               # 消息處理
├── proto/                     # Protocol Buffers 定義 ⭐
│   ├── chat.proto            # gRPC 服務定義
│   └── chat/                 # 生成的 gRPC 代碼
├── web/                      # 測試用前端 (僅測試)
│   └── index.html            # 聊天測試介面
├── scripts/                  # 測試腳本
│   ├── test_grpc.sh         # gRPC 測試腳本
│   └── test_chat.sh         # HTTP API 測試腳本
├── configs/                  # 配置文件
│   ├── local.yml            # 本地開發配置
│   ├── development.yaml     # 開發環境
│   ├── staging.yaml         # 測試環境
│   └── production.yaml      # 生產環境
└── build/                   # 構建配置
    ├── Dockerfile
    └── docker-compose.yml
```

## 🚀 快速開始

### 環境要求

- Go 1.24+
- MongoDB 4.4+
- grpcurl (測試 gRPC 用)

### 1. 安裝依賴

```bash
# 安裝 Go 依賴
go mod download

# 安裝 grpcurl (測試 gRPC 用)
brew install grpcurl  # macOS
```

### 2. 啟動 MongoDB

```bash
# 使用 Docker（推薦）
docker run -d --name mongodb -p 27017:27017 mongo:latest

# 或使用本地 MongoDB
mongod --dbpath /path/to/your/db
```

### 3. 配置環境

編輯配置文件 `configs/local.yml`：

```yaml
server:
  grpc_port: "8081"   # gRPC 服務端口
  http_port: "8080"   # HTTP API Bridge 端口

database:
  mongodb:
    uri: "mongodb://localhost:27017"
    database: "chatroom"
```

### 4. 運行服務

```bash
# 使用 Air（推薦，支持熱重載）
air

# 或直接運行
go run cmd/api/main.go

# 或編譯後運行
go build -o bin/chat-gateway cmd/api/main.go
./bin/chat-gateway
```

服務啟動後：
- gRPC 服務: `localhost:8081`
- HTTP API: `localhost:8080`
- 測試頁面: `http://localhost:8080` (使用 Live Server 打開 `web/index.html`)

### 5. 測試服務

```bash
# 檢查健康狀態
curl http://localhost:8080/health

# 運行 gRPC 測試
./scripts/test_grpc.sh

# 運行 HTTP API 測試
./scripts/test_chat.sh
```

## 🧪 測試

### gRPC 測試 (推薦)

```bash
# 完整測試流程（一對一、群組、消息）
./scripts/test_grpc.sh

# 測試創建聊天室
grpcurl -plaintext -import-path proto -proto chat.proto \
  -d '{
    "name": "測試聊天室",
    "type": "group",
    "owner_id": "user_alice",
    "member_ids": ["user_alice", "user_bob", "user_charlie"]
  }' \
  localhost:8081 chat.ChatRoomService/CreateRoom

# 測試發送消息
grpcurl -plaintext -import-path proto -proto chat.proto \
  -d '{
    "room_id": "YOUR_ROOM_ID",
    "sender_id": "user_alice",
    "content": "Hello, World!",
    "type": "text"
  }' \
  localhost:8081 chat.ChatRoomService/SendMessage
```

### HTTP API 測試 (測試用)

```bash
# 創建群組
curl -X POST http://localhost:8080/api/v1/rooms \
  -H "Content-Type: application/json" \
  -d '{
    "name": "測試群組",
    "type": "group",
    "owner_id": "user_alice",
    "members": [
      {"user_id": "user_alice", "role": "admin"},
      {"user_id": "user_bob", "role": "member"}
    ]
  }'

# 發送消息
curl -X POST http://localhost:8080/api/v1/messages \
  -H "Content-Type: application/json" \
  -d '{
    "room_id": "YOUR_ROOM_ID",
    "sender_id": "user_alice",
    "content": "Hello!",
    "type": "text"
  }'

# 獲取消息
curl "http://localhost:8080/api/v1/messages?room_id=YOUR_ROOM_ID&user_id=user_alice&limit=20"
```

### Web 測試界面 (僅供測試)

1. 使用 VS Code Live Server 打開 `web/index.html`
2. 選擇用戶身份（Alice、Bob、Charlie、David）
3. 點擊聯絡人開始一對一聊天
4. 或點擊「創建群組」創建群組聊天
5. 測試消息發送、已讀狀態等功能

> **注意**: 目前使用 HTTP 輪詢（每3秒）獲取新消息，非即時推送。WebSocket 待實現。

## 📊 功能詳解

### 1. 一對一聊天

- **自動防重複**: 系統自動檢測是否已存在兩人之間的聊天室
- **點擊即聊**: 點擊聯絡人直接開啟聊天，無需手動創建
- **已讀狀態**: 顯示對方是否已讀你的消息

### 2. 群組聊天

- **成員管理**: 支援添加成員、移除成員、退出群組
- **已讀統計**: 顯示「N已讀」，統計群組內已讀人數
- **角色管理**: 支援管理員和普通成員角色

### 3. 消息功能

- **即時發送**: gRPC 高效能消息傳遞
- **歷史記錄**: MongoDB 持久化儲存
- **已讀/送達**: 追蹤消息狀態
- **ID 管理**: 自動生成 MongoDB ObjectID
- **消息推送**: 目前使用 HTTP 輪詢（每3秒），WebSocket 待實現

### 4. 數據庫設計

```javascript
// ChatRoom 聊天室
{
  _id: ObjectId,
  id: "string (hex)",
  name: "聊天室名稱",
  description: "描述",
  type: "direct | group",
  owner_id: "擁有者ID",
  members: [
    {
      user_id: "用戶ID",
      username: "用戶名",
      role: "admin | member",
      joined_at: ISODate,
      last_seen: ISODate
    }
  ],
  created_at: ISODate,
  updated_at: ISODate
}

// Message 消息
{
  _id: ObjectId,
  id: "string (hex)",
  room_id: "聊天室ID",
  sender_id: "發送者ID",
  content: "消息內容",
  type: "text | image | file | ...",
  status: "sent | delivered | read",
  read_by: [
    {
      user_id: "用戶ID",
      read_at: ISODate
    }
  ],
  delivered_to: [...],
  created_at: ISODate,
  updated_at: ISODate
}
```

## 🔐 安全設計

### 已實現
- ✅ MongoDB 連接安全
- ✅ 數據驗證
- ✅ 錯誤處理

### 規劃中
- ⏳ TLS/SSL 加密傳輸
- ⏳ 端到端加密 (Signal Protocol)
- ⏳ JWT 身份驗證
- ⏳ 審計日誌

## 🐳 Docker 部署

```bash
# 構建鏡像
cd build
docker build -t chat-gateway .

# 運行服務
docker-compose up -d

# 查看日誌
docker-compose logs -f chat-gateway
```

## 📈 性能優化

### 數據庫索引

服務啟動時自動創建索引：
- `members.user_id` - 用戶聊天室查詢
- `room_id + created_at` - 消息查詢
- `sender_id` - 發送者查詢
- `type` - 消息類型查詢

### 查詢優化

- 使用游標分頁避免深度分頁問題
- 合理的查詢限制（最大 100 條/次）
- MongoDB 投影減少網絡傳輸

## 📚 相關文檔

- [快速開始指南](QUICK_START.md)
- [測試指南](TESTING.md)
- [gRPC Proto 定義](proto/chat.proto)

## 🔧 開發指南

### 生成 gRPC 代碼

```bash
# 安裝 protoc 工具
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 生成代碼
protoc --go_out=. --go-grpc_out=. proto/chat.proto
```

### 代碼規範

```bash
# 格式化代碼
go fmt ./...

# 運行 linter
golangci-lint run

# 運行測試
go test ./...
```

## 🤝 貢獻

歡迎提交 Issue 和 Pull Request！

## 📄 授權

本專案採用 MIT 授權條款。

---

## 🎯 下一步計劃

### 高優先級
- [ ] **添加 WebSocket 支援** (實時消息推送，取代 HTTP 輪詢)
- [ ] **實現 StreamMessages** (gRPC 流式消息)
- [ ] **實現 GetUnreadCount** (未讀數量統計)

### 中優先級
- [ ] 添加消息搜索功能
- [ ] 實現文件上傳/分享
- [ ] 添加單元測試和集成測試
- [ ] 性能測試和優化

### 低優先級（規劃中）
- [ ] 實現端到端加密 (Signal Protocol)
- [ ] JWT 身份驗證
- [ ] 審計日誌系統
