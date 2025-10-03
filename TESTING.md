# 測試文檔

本文檔提供 Chat Gateway 項目的測試指南和測試策略。

## 目錄

- [測試策略](#測試策略)
- [環境準備](#環境準備)
- [單元測試](#單元測試)
- [集成測試](#集成測試)
- [端到端測試](#端到端測試)
- [性能測試](#性能測試)
- [測試數據](#測試數據)

## 測試策略

### 測試金字塔

```
        /\
       /E2E\          少量端到端測試
      /------\
     /  集成  \        中等數量集成測試
    /----------\
   /  單元測試  \      大量單元測試
  /--------------\
```

### 測試覆蓋率目標

- 核心業務邏輯：90%+
- 安全相關代碼：100%
- HTTP Handlers：80%+
- 整體覆蓋率：75%+

## 環境準備

### 安裝測試依賴

```bash
# 安裝 Go 測試工具
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go install github.com/onsi/gomega@latest

# 安裝覆蓋率工具
go install github.com/axw/gocov/gocov@latest
go install github.com/AlekSi/gocov-xml@latest
```

### 啟動測試環境

```bash
# 啟動 MongoDB（Docker）
docker run -d --name mongo-test \
  -p 27018:27017 \
  mongo:latest

# 或使用 docker-compose
docker-compose -f docker-compose.test.yml up -d
```

### 設置環境變量

```bash
export MONGO_URI="mongodb://localhost:27018"
export MONGO_DATABASE="chatroom_test"
export MASTER_KEY=$(openssl rand -base64 32)
export GIN_MODE=test
```

## 單元測試

### 運行單元測試

```bash
# 運行所有單元測試
go test ./...

# 運行特定包
go test ./internal/security/encryption/...

# 詳細輸出
go test -v ./...

# 並行運行
go test -parallel 4 ./...
```

### 查看覆蓋率

```bash
# 生成覆蓋率報告
go test -cover ./...

# 詳細覆蓋率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 按函數查看
go tool cover -func=coverage.out
```

### 單元測試示例

#### 測試加密功能

`internal/security/encryption/aes_ctr_test.go`:

```go
package encryption_test

import (
    "testing"
    
    "chat-gateway/internal/security/encryption"
)

func TestAESCTREncryption(t *testing.T) {
    // 生成測試密鑰
    key := make([]byte, 32)
    rand.Read(key)
    
    // 創建加密器
    enc, err := encryption.NewAESCTREncryption(key)
    if err != nil {
        t.Fatalf("Failed to create encryptor: %v", err)
    }
    
    // 測試加密和解密
    plaintext := "Hello, World!"
    ciphertext, err := enc.Encrypt(plaintext)
    if err != nil {
        t.Fatalf("Encryption failed: %v", err)
    }
    
    decrypted, err := enc.Decrypt(ciphertext)
    if err != nil {
        t.Fatalf("Decryption failed: %v", err)
    }
    
    if decrypted != plaintext {
        t.Errorf("Expected %s, got %s", plaintext, decrypted)
    }
}
```

#### 測試密鑰管理

```go
func TestKeyManagerPersistence(t *testing.T) {
    // Setup
    masterKey := make([]byte, 32)
    rand.Read(masterKey)
    
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    km, err := keymanager.NewKeyManagerWithPersistence(masterKey, db)
    if err != nil {
        t.Fatalf("Failed to create KeyManager: %v", err)
    }
    
    // 測試密鑰創建和持久化
    roomID := "test_room_1"
    key1, err := km.GetOrCreateRoomKey(roomID)
    if err != nil {
        t.Fatalf("Failed to create key: %v", err)
    }
    
    // 創建新的 KeyManager 實例（模擬重啟）
    km2, err := keymanager.NewKeyManagerWithPersistence(masterKey, db)
    if err != nil {
        t.Fatalf("Failed to create second KeyManager: %v", err)
    }
    
    // 驗證可以加載持久化的密鑰
    key2, err := km2.GetOrCreateRoomKey(roomID)
    if err != nil {
        t.Fatalf("Failed to load key: %v", err)
    }
    
    // 密鑰應該相同
    if !bytes.Equal(key1, key2) {
        t.Error("Keys do not match after persistence")
    }
}
```

## 集成測試

### 運行集成測試

```bash
# 使用 build tag
go test -tags=integration ./tests/integration/...

# 詳細輸出
go test -tags=integration -v ./tests/integration/...
```

### 集成測試示例

#### 測試完整消息流程

`tests/integration/message_flow_test.go`:

```go
// +build integration

package integration

import (
    "context"
    "testing"
    "time"
    
    "chat-gateway/internal/grpc"
    "chat-gateway/proto/chat"
)

func TestMessageFlow(t *testing.T) {
    // 設置測試服務器
    server, cleanup := setupTestServer(t)
    defer cleanup()
    
    ctx := context.Background()
    
    // 1. 創建聊天室
    roomResp, err := server.CreateRoom(ctx, &chat.CreateRoomRequest{
        Name:      "Test Room",
        Type:      "group",
        OwnerId:   "user_alice",
        MemberIds: []string{"user_alice", "user_bob"},
    })
    if err != nil {
        t.Fatalf("Failed to create room: %v", err)
    }
    roomID := roomResp.Room.Id
    
    // 2. 發送消息
    msgResp, err := server.SendMessage(ctx, &chat.SendMessageRequest{
        RoomId:   roomID,
        SenderId: "user_alice",
        Content:  "Hello!",
        Type:     "text",
    })
    if err != nil {
        t.Fatalf("Failed to send message: %v", err)
    }
    
    // 3. 獲取消息
    messagesResp, err := server.GetMessages(ctx, &chat.GetMessagesRequest{
        RoomId: roomID,
        UserId: "user_bob",
        Limit:  10,
    })
    if err != nil {
        t.Fatalf("Failed to get messages: %v", err)
    }
    
    // 4. 驗證
    if len(messagesResp.Messages) != 1 {
        t.Errorf("Expected 1 message, got %d", len(messagesResp.Messages))
    }
    
    msg := messagesResp.Messages[0]
    if msg.Content != "Hello!" {
        t.Errorf("Expected 'Hello!', got '%s'", msg.Content)
    }
    
    // 5. 標記已讀
    _, err = server.MarkAsRead(ctx, &chat.MarkAsReadRequest{
        RoomId: roomID,
        UserId: "user_bob",
    })
    if err != nil {
        t.Fatalf("Failed to mark as read: %v", err)
    }
    
    // 6. 驗證已讀狀態
    time.Sleep(100 * time.Millisecond) // 等待更新
    
    messagesResp2, err := server.GetMessages(ctx, &chat.GetMessagesRequest{
        RoomId: roomID,
        UserId: "user_bob",
        Limit:  10,
    })
    if err != nil {
        t.Fatalf("Failed to get messages: %v", err)
    }
    
    msg2 := messagesResp2.Messages[0]
    if !contains(msg2.ReadBy, "user_bob") {
        t.Error("user_bob should be in ReadBy list")
    }
}
```

## 端到端測試

### 手動 E2E 測試

使用測試前端進行手動測試：

1. 啟動服務器
```bash
./bin/chat-gateway
```

2. 打開測試前端
```bash
open web/index.html
```

3. 測試場景：
   - 創建聊天室
   - 發送消息
   - 即時接收消息（SSE）
   - 標記已讀
   - 添加/移除成員
   - 加入/離開群組

### 自動化 E2E 測試（TODO）

使用 Selenium 或 Playwright 進行自動化測試。

## 性能測試

### 壓力測試

使用 `go test -bench`:

```bash
# 運行基準測試
go test -bench=. -benchmem ./...

# 特定測試
go test -bench=BenchmarkEncryption -benchmem ./internal/security/encryption/
```

### 基準測試示例

```go
func BenchmarkAESCTREncryption(b *testing.B) {
    key := make([]byte, 32)
    rand.Read(key)
    
    enc, _ := encryption.NewAESCTREncryption(key)
    plaintext := "Hello, World! This is a test message."
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        enc.Encrypt(plaintext)
    }
}

func BenchmarkAESCTRDecryption(b *testing.B) {
    key := make([]byte, 32)
    rand.Read(key)
    
    enc, _ := encryption.NewAESCTREncryption(key)
    plaintext := "Hello, World! This is a test message."
    ciphertext, _ := enc.Encrypt(plaintext)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        enc.Decrypt(ciphertext)
    }
}
```

### 負載測試

使用 Apache Bench 或 k6:

```bash
# 使用 Apache Bench
ab -n 1000 -c 10 -p message.json -T application/json \
  http://localhost:8080/api/v1/messages

# 使用 k6
k6 run tests/load/message_test.js
```

## 測試數據

### 測試用戶

```go
const (
    UserAlice   = "user_alice"
    UserBob     = "user_bob"
    UserCharlie = "user_charlie"
    UserDavid   = "user_david"
)
```

### 測試助手函數

`tests/helpers/setup.go`:

```go
package helpers

import (
    "context"
    "testing"
    
    "chat-gateway/internal/platform/config"
    "chat-gateway/internal/platform/driver"
    "chat-gateway/internal/storage/database"
)

func SetupTestDB(t *testing.T) *mongo.Database {
    config.Load("test")
    
    if err := driver.ConnectMongo(); err != nil {
        t.Fatalf("Failed to connect to test DB: %v", err)
    }
    
    return driver.GetMongoDatabase()
}

func CleanupTestDB(t *testing.T, db *mongo.Database) {
    ctx := context.Background()
    
    // 清理測試數據
    db.Collection("chatrooms").Drop(ctx)
    db.Collection("messages").Drop(ctx)
    db.Collection("encryption_keys").Drop(ctx)
    
    driver.CloseMongo()
}

func CreateTestRoom(t *testing.T, server *grpc.Server, ownerID string, memberIDs []string) string {
    ctx := context.Background()
    
    resp, err := server.CreateRoom(ctx, &chat.CreateRoomRequest{
        Name:      "Test Room",
        Type:      "group",
        OwnerId:   ownerID,
        MemberIds: memberIDs,
    })
    
    if err != nil {
        t.Fatalf("Failed to create test room: %v", err)
    }
    
    return resp.Room.Id
}
```

## 測試最佳實踐

### 1. 測試命名

- 測試文件：`*_test.go`
- 測試函數：`TestXxx` 或 `Test_xxx`
- 基準測試：`BenchmarkXxx`
- 測試表驅動：使用 subtests

```go
func TestEncryption(t *testing.T) {
    tests := []struct {
        name      string
        plaintext string
        wantErr   bool
    }{
        {"empty string", "", true},
        {"normal text", "Hello", false},
        {"unicode", "你好世界", false},
        {"long text", strings.Repeat("a", 10000), false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 測試邏輯
        })
    }
}
```

### 2. 清理資源

始終使用 `defer` 清理資源：

```go
func TestSomething(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    // 測試邏輯
}
```

### 3. 並發測試

測試並發安全：

```go
func TestConcurrentAccess(t *testing.T) {
    var wg sync.WaitGroup
    
    for i := 0; i < 100; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            // 並發操作
        }()
    }
    
    wg.Wait()
}
```

### 4. Mock 外部依賴

使用接口和 mock：

```go
type MessageRepository interface {
    Create(ctx context.Context, message *Message) error
    GetByID(ctx context.Context, id string) (*Message, error)
}

type MockMessageRepository struct {
    CreateFunc  func(ctx context.Context, message *Message) error
    GetByIDFunc func(ctx context.Context, id string) (*Message, error)
}
```

## 持續集成

### GitHub Actions 配置

`.github/workflows/test.yml`:

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    
    services:
      mongodb:
        image: mongo:latest
        ports:
          - 27017:27017
    
    steps:
      - uses: actions/checkout@v2
      
      - name: Set up Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.21
      
      - name: Install dependencies
        run: go mod download
      
      - name: Run tests
        run: go test -v -cover ./...
        env:
          MONGO_URI: mongodb://localhost:27017
          MONGO_DATABASE: chatroom_test
      
      - name: Run integration tests
        run: go test -tags=integration -v ./tests/integration/...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v2
        with:
          file: ./coverage.out
```

## 故障排除

### 常見測試問題

**問題：測試失敗 "connection refused"**
```
解決：確保 MongoDB 正在運行
docker ps | grep mongo
```

**問題：測試超時**
```
解決：增加超時時間
go test -timeout 30s ./...
```

**問題：並發測試隨機失敗**
```
解決：檢查 race condition
go test -race ./...
```

**問題：覆蓋率報告不準確**
```
解決：排除生成的代碼
go test -coverprofile=coverage.out $(go list ./... | grep -v /proto/)
```

## 參考資源

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Table Driven Tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [Ginkgo BDD Framework](https://onsi.github.io/ginkgo/)
- [gomock](https://github.com/golang/mock)
