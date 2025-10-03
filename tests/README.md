# 測試目錄

此目錄包含 Chat Gateway 項目的所有測試文件。

## 目錄結構

```
tests/
├── README.md                 # 本文件
├── integration/             # 集成測試
│   ├── message_flow_test.go
│   ├── room_test.go
│   └── encryption_test.go
├── e2e/                     # 端到端測試（TODO）
│   └── web_test.go
├── load/                    # 負載測試（TODO）
│   ├── message_test.js
│   └── sse_test.js
├── helpers/                 # 測試輔助函數
│   ├── setup.go
│   ├── assertions.go
│   └── fixtures.go
└── mocks/                   # Mock 對象（TODO）
    ├── repository_mock.go
    └── client_mock.go
```

## 測試類型

### 1. 單元測試

位置：與源代碼同目錄
命名：`*_test.go`

運行：
```bash
go test ./...
```

### 2. 集成測試

位置：`tests/integration/`
標籤：`// +build integration`

運行：
```bash
go test -tags=integration ./tests/integration/...
```

### 3. 端到端測試

位置：`tests/e2e/`（TODO）

運行：
```bash
go test -tags=e2e ./tests/e2e/...
```

### 4. 負載測試

位置：`tests/load/`（TODO）

運行：
```bash
k6 run tests/load/message_test.js
```

## 快速開始

### 運行所有測試

```bash
# 1. 啟動測試數據庫
docker run -d --name mongo-test -p 27018:27017 mongo:latest

# 2. 設置環境變量
export MONGO_URI="mongodb://localhost:27018"
export MONGO_DATABASE="chatroom_test"
export MASTER_KEY=$(openssl rand -base64 32)

# 3. 運行測試
go test ./...

# 4. 運行集成測試
go test -tags=integration ./tests/integration/...

# 5. 清理
docker stop mongo-test && docker rm mongo-test
```

### 運行特定測試

```bash
# 單個測試文件
go test ./internal/security/encryption/aes_ctr_test.go

# 單個測試函數
go test -run TestAESCTREncryption ./internal/security/encryption/

# 使用詳細輸出
go test -v ./...
```

### 查看覆蓋率

```bash
# 生成覆蓋率報告
go test -coverprofile=coverage.out ./...

# 查看 HTML 報告
go tool cover -html=coverage.out

# 查看覆蓋率摘要
go tool cover -func=coverage.out
```

## 測試輔助工具

### helpers/setup.go

提供測試環境設置函數：

```go
// SetupTestDB 設置測試數據庫
func SetupTestDB(t *testing.T) *mongo.Database

// CleanupTestDB 清理測試數據庫
func CleanupTestDB(t *testing.T, db *mongo.Database)

// CreateTestServer 創建測試 gRPC 服務器
func CreateTestServer(t *testing.T) (*grpc.Server, func())

// CreateTestRoom 創建測試聊天室
func CreateTestRoom(t *testing.T, server *grpc.Server, ownerID string, members []string) string
```

### helpers/assertions.go

提供自定義斷言函數：

```go
// AssertNoError 斷言沒有錯誤
func AssertNoError(t *testing.T, err error)

// AssertEqual 斷言相等
func AssertEqual(t *testing.T, expected, actual interface{})

// AssertContains 斷言包含
func AssertContains(t *testing.T, list []string, item string)

// AssertEncrypted 斷言內容已加密
func AssertEncrypted(t *testing.T, content string)
```

### helpers/fixtures.go

提供測試數據：

```go
// TestUsers 測試用戶列表
var TestUsers = []string{
    "user_alice",
    "user_bob",
    "user_charlie",
    "user_david",
}

// GenerateTestMessage 生成測試消息
func GenerateTestMessage(roomID, senderID string) *chat.SendMessageRequest

// GenerateTestRoom 生成測試聊天室
func GenerateTestRoom(ownerID string, members []string) *chat.CreateRoomRequest
```

## 測試數據管理

### 測試數據隔離

每個測試應該：
1. 使用獨立的測試數據庫
2. 測試前清理數據
3. 測試後清理數據

示例：
```go
func TestSomething(t *testing.T) {
    db := helpers.SetupTestDB(t)
    defer helpers.CleanupTestDB(t, db)
    
    // 測試邏輯
}
```

### 測試數據生成

使用 fixtures 生成一致的測試數據：

```go
func TestMessageFlow(t *testing.T) {
    room := helpers.GenerateTestRoom("user_alice", helpers.TestUsers)
    message := helpers.GenerateTestMessage(roomID, "user_alice")
    
    // 使用生成的測試數據
}
```

## CI/CD 集成

### GitHub Actions

測試自動在以下情況運行：
- Push 到任何分支
- Pull Request
- 每日定時任務（夜間測試）

配置文件：`.github/workflows/test.yml`

### 本地 CI 模擬

```bash
# 運行與 CI 相同的測試
./scripts/ci_test.sh
```

## 測試最佳實踐

### 1. 測試命名

清晰描述測試內容：
```go
// 好
func TestAESCTREncryption_WithEmptyString_ReturnsError(t *testing.T)

// 不好
func TestEncrypt(t *testing.T)
```

### 2. 測試結構

使用 AAA 模式（Arrange-Act-Assert）：
```go
func TestSomething(t *testing.T) {
    // Arrange - 準備測試數據
    input := "test"
    
    // Act - 執行被測試的操作
    result, err := DoSomething(input)
    
    // Assert - 驗證結果
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result != expected {
        t.Errorf("expected %v, got %v", expected, result)
    }
}
```

### 3. 表驅動測試

處理多個測試用例：
```go
func TestEncryption(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"empty", "", true},
        {"normal", "hello", false},
        {"unicode", "你好", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := Encrypt(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("want error: %v, got: %v", tt.wantErr, err)
            }
        })
    }
}
```

### 4. 使用 Subtests

組織相關測試：
```go
func TestRoomManagement(t *testing.T) {
    t.Run("Create", func(t *testing.T) {
        // 測試創建
    })
    
    t.Run("Join", func(t *testing.T) {
        // 測試加入
    })
    
    t.Run("Leave", func(t *testing.T) {
        // 測試離開
    })
}
```

### 5. 清理資源

始終清理測試資源：
```go
func TestWithResources(t *testing.T) {
    resource := acquireResource()
    defer releaseResource(resource)
    
    // 測試邏輯
}
```

### 6. 避免測試依賴

每個測試應該獨立：
```go
// 不好 - 測試順序依賴
func TestCreate(t *testing.T) { /* ... */ }
func TestUpdate(t *testing.T) { /* 依賴 TestCreate */ }

// 好 - 每個測試獨立
func TestCreate(t *testing.T) { /* ... */ }
func TestUpdate(t *testing.T) {
    // 自己創建所需數據
    setup()
    // 測試更新邏輯
}
```

### 7. 並發測試

測試並發安全性：
```go
func TestConcurrentAccess(t *testing.T) {
    t.Parallel() // 標記為可並行
    
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

### 8. Race Detector

檢測競態條件：
```bash
go test -race ./...
```

## 故障排除

### 測試失敗時

1. **查看詳細輸出**
```bash
go test -v ./...
```

2. **運行特定失敗的測試**
```bash
go test -run TestName ./package/path
```

3. **增加超時時間**
```bash
go test -timeout 30s ./...
```

4. **檢查 race condition**
```bash
go test -race ./...
```

### 測試卡住時

1. **添加超時**
```bash
go test -timeout 10s ./...
```

2. **檢查死鎖**
   - 查看測試中的 goroutine
   - 檢查 channel 操作
   - 檢查 mutex 使用

### 覆蓋率低時

1. **識別未覆蓋的代碼**
```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

2. **添加缺失的測試用例**
   - 正常情況
   - 錯誤情況
   - 邊界條件
   - 並發情況

## 貢獻測試

### 添加新測試

1. 確定測試類型（單元/集成/E2E）
2. 選擇正確的位置
3. 遵循命名約定
4. 使用輔助函數
5. 添加清理邏輯
6. 運行並驗證

### 測試 PR 檢查清單

提交測試相關的 PR 時：

- [ ] 所有測試通過
- [ ] 新代碼有對應測試
- [ ] 覆蓋率沒有下降
- [ ] 沒有 race condition
- [ ] 測試文檔已更新
- [ ] CI 通過

## 參考資源

- 主測試文檔：`../TESTING.md`
- 測試指南：`../TESTING_GUIDE.md`
- Go 測試文檔：https://golang.org/pkg/testing/
- 測試最佳實踐：https://github.com/golang/go/wiki/TableDrivenTests

## 聯繫

測試相關問題：
- 提交 Issue
- 查看測試文檔
- 參考現有測試用例
