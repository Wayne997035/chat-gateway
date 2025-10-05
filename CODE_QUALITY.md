# 程式碼品質檢查指南

本指南詳細說明 Chat Gateway 聊天室微服務的程式碼品質檢查流程、工具配置和常見問題範例。

## 執行檢查

### 方式一：使用 Taskfile (推薦)
```bash
cd build && task check
```

### 方式二：直接執行 golangci-lint
```bash
golangci-lint run ./...
```

## 檢查項目總覽

### 品質檢查內容

- **go vet**: Go 官方 static analysis，檢查常見程式碼錯誤
- **go mod tidy**: 整理 dependency management，移除未使用的套件
- **staticcheck**: 進階 static analysis，檢查潛在 bug 和效能問題
- **gosec**: security analysis，檢查安全漏洞和不良實踐
- **golangci-lint**: 綜合 linting (25個 linters)
- **goconst**: 重複字串檢測，建議提取為 constant
- **unit tests**: 單元測試

## golangci-lint 25個 Linter 詳細說明

### 1. 程式碼格式化和風格檢查

#### gofmt - Go 官方格式化工具
確保程式碼符合 Go 標準格式。

**會自動修正的問題：**
- 縮排不一致
- 空行使用不當
- 運算子周圍的空格

#### goimports - 自動管理 import 語句
自動整理和排序 import 順序，移除未使用的 import。

**會自動修正：**
```go
// 不良寫法
import (
    "fmt"
    "os"
    "strings"
    "time"
    "net/http"  // 未使用
)

// 正確寫法
import (
    "fmt"
    "os"
    "strings"
    "time"
)
```

#### gofumpt - 更嚴格的程式碼格式化
提供額外的格式化規則，比 gofmt 更嚴格。

**額外規則：**
- 移除不必要的空行
- 統一空格的使用
- 更嚴格的縮排規則

### 2. 官方 Go 工具

#### govet - Go 官方靜態分析工具
檢查常見的程式碼錯誤和可疑結構。

**會抓到的問題：**

##### printf 格式錯誤
```go
// 不良寫法
fmt.Printf("User %s has %d items", userId)  // 缺少第二個參數
fmt.Printf("Value: %d", "hello")            // 型別不匹配

// 正確寫法
fmt.Printf("User %s has %d items", userId, itemCount)
fmt.Printf("Value: %s", "hello")
```

##### 無法到達的程式碼
```go
// 不良寫法
func example() {
    return
    fmt.Println("這行永遠不會執行")  // unreachable code
}
```

##### 錯誤的 struct tag
```go
// 不良寫法
type User struct {
    Name string `json:"name,omitempty,invalid"`  // 無效的 tag
    Age  int    `json:name`                      // 缺少引號
}

// 正確寫法
type User struct {
    Name string `json:"name,omitempty"`
    Age  int    `json:"age"`
}
```

### 3. 錯誤處理檢查

#### errcheck - 檢查錯誤是否被正確處理
檢查函數返回的錯誤是否被正確處理，避免忽略錯誤。

**會抓到的問題：**
```go
// 不良寫法
func processFile(filename string) {
    file, _ := os.Open(filename)  // 忽略錯誤
    defer file.Close()
}

// 正確寫法
func processFile(filename string) error {
    file, err := os.Open(filename)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer file.Close()
    return nil
}
```

#### rowserrcheck - 檢查 rows.Err() 是否被調用
檢查 database/sql 的 rows.Err() 是否被調用，確保資料庫錯誤被處理。

**會抓到的問題：**
```go
// 不良寫法
func queryUsers(db *sql.DB) ([]User, error) {
    rows, err := db.Query("SELECT * FROM users")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        // 處理資料
    }
    // 缺少 rows.Err() 檢查
    return users, nil
}
```

**解決方案：**
```go
// 正確寫法
func queryUsers(db *sql.DB) ([]User, error) {
    rows, err := db.Query("SELECT * FROM users")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        // 處理資料
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating rows: %w", err)
    }
    return users, nil
}
```

### 4. 靜態分析工具

#### staticcheck - 進階靜態分析工具
檢查潛在的 bug、效能問題和程式碼簡化機會。

**會抓到的問題：**

##### 未使用的變數
```go
// 不良寫法
func processUser(user User) {
    name := user.Name  // 變數被賦值但從未使用
    fmt.Println("Processing user")
}

// 正確寫法
func processUser(user User) {
    fmt.Printf("Processing user: %s", user.Name)
}
```

##### 簡化的程式碼
```go
// 不良寫法
if x == true {
    return true
}
return false

// 正確寫法
return x
```

#### gosimple - 檢查可以簡化的程式碼
建議更簡潔的寫法。

**會抓到的問題：**
```go
// 不良寫法
for i, _ := range items {
    fmt.Println(items[i])
}

// 正確寫法
for i := range items {
    fmt.Println(items[i])
}
```

#### ineffassign - 檢查無效的賦值
找出被賦值但從未使用的變數。

**會抓到的問題：**
```go
// 不良寫法
func process() {
    x := 1
    x = 2  // 第一次賦值從未使用
    fmt.Println(x)
}

// 正確寫法
func process() {
    x := 2
    fmt.Println(x)
}
```

#### unused - 檢查未使用的程式碼
檢查未使用的變數、函數和 import，清理無用的程式碼。

### 5. 程式碼品質檢查

#### misspell - 檢查英文拼寫錯誤
確保程式碼中的英文單字正確。

**會抓到的問題：**
```go
// 不良寫法
func processRecievedData() {  // 拼寫錯誤：recieved -> received
    fmt.Println("Processing data")
}

// 正確寫法
func processReceivedData() {
    fmt.Println("Processing data")
}
```

#### gocyclo - 檢查函數複雜度
避免過於複雜的函數，閾值設定為 15。

**複雜度計算：**
- 每個 if、for、switch、case 語句 +1
- 每個 &&、|| 運算子 +1
- 每個 catch 語句 +1

**解決方案：拆分成多個函數**

#### dupl - 檢測重複程式碼
檢測重複的程式碼片段，建議重構為共用函數，閾值設定為 150 tokens（忽略小型 CRUD 重複，專注於大型重複邏輯）。

#### goconst - 檢測重複字串常數
檢測重複的字串常數，建議提取為常數以提高維護性。

**配置參數：**
- `-min-occurrences 3`: 3次以上才警告
- `-min-length 1`: 最小長度1字元

**會抓到的問題：**
```go
// 不良寫法（如果出現3次以上）
func validateUser(user User) error {
    if user.Name == "" {
        return fmt.Errorf("field is required")  // 重複字串
    }
    if user.Email == "" {
        return fmt.Errorf("field is required")  // 重複字串
    }
    if user.Age == 0 {
        return fmt.Errorf("field is required")  // 重複字串
    }
    return nil
}
```

**解決方案：**
```go
// 正確寫法 - 提取為常數
const ErrFieldRequired = "field is required"

func validateUser(user User) error {
    if user.Name == "" {
        return fmt.Errorf(ErrFieldRequired)
    }
    if user.Email == "" {
        return fmt.Errorf(ErrFieldRequired)
    }
    if user.Age == 0 {
        return fmt.Errorf(ErrFieldRequired)
    }
    return nil
}
```

### 6. 程式碼風格和慣例檢查

#### gocritic - 綜合程式碼風格檢查
提供多種程式碼品質建議。

#### lll - 檢查行長度
避免過長的行影響可讀性，限制 140 字元。

#### nakedret - 檢查裸返回語句
建議明確指定返回值。

#### noctx - 檢查沒有 context 的函數
建議在適當情況下使用 context。

**會抓到的問題：**
```go
// 不良寫法
func fetchData(url string) ([]byte, error) {
    resp, err := http.Get(url)  // 缺少 context
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return ioutil.ReadAll(resp.Body)
}
```

**解決方案：**
```go
// 正確寫法 - 使用 context
func fetchData(ctx context.Context, url string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, err
    }

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    return ioutil.ReadAll(resp.Body)
}
```

### 7. 安全性檢查

#### gosec - 安全分析工具
檢查潛在的安全漏洞和不良實踐。

**會抓到的問題：**

##### 硬編碼憑證
```go
// 不良寫法
password := "hardcoded_password_123"

// 正確寫法
password := os.Getenv("DB_PASSWORD")
```

##### 不安全的隨機數生成
```go
// 不良寫法
import "math/rand"

func generateToken() string {
    return fmt.Sprintf("%d", rand.Int())  // 使用不安全的隨機數
}

// 正確寫法
import "crypto/rand"

func generateToken() string {
    b := make([]byte, 16)
    rand.Read(b)
    return fmt.Sprintf("%x", b)
}
```

##### main 函數中的 exitAfterDefer 問題
gosec 會檢查 main 函數中是否直接使用 `log.Fatal` 或 `os.Exit`，因為這些函數會立即終止程式，導致 `defer` 函數無法正常執行。

**會抓到的問題：**
```go
// 不良寫法
func main() {
    db, err := connectDB()
    if err != nil {
        log.Fatal("資料庫連接失敗:", err)  // 直接終止，defer 無法執行
    }
    defer db.Close()  // 這個 defer 永遠不會執行
    startServer()
}
```

**解決方案：**
```go
// 正確寫法 - 分離主要邏輯
func main() {
    if err := mainNoExit(); err != nil {
        log.Fatal(err)
    }
}

func mainNoExit() error {
    db, err := connectDB()
    if err != nil {
        return fmt.Errorf("資料庫連接失敗: %w", err)
    }
    defer db.Close()  // 現在可以正常執行
    return startServer()
}
```

### 8. 魔法數字檢查 (已停用)

#### mnd - Magic Number Detector
檢查魔法數字，已排除（某些硬編碼數字在業務邏輯中是合理的）。

**為什麼停用 mnd：**
1. **業務邏輯中的常數是合理的**：某些數字在業務邏輯中有明確的意義
2. **避免過度抽象**：不是所有數字都需要提取為常數
3. **減少誤報**：mnd 會對很多合理的數字發出警告

**實際例子：**
```go
// 時間單位轉換 - 標準的時間轉換常數
rotatelogs.WithMaxAge(time.Duration(maxAge)*24*time.Hour)

// 檔案權限設定 - Unix 標準權限
if err := os.MkdirAll(logDir, 0o750); err != nil {
    // 0o750 是標準的目錄權限
}
```

## Chat Gateway 專案特色檢查

### 1. 加密和密鑰管理
- 確保所有敏感數據使用 AES-256-CTR 加密
- 檢查密鑰是否從環境變量讀取，而非硬編碼
- 驗證密鑰輪替邏輯的正確性

### 2. gRPC 服務
- 檢查 context 的正確使用
- 確保錯誤處理完整
- 驗證 proto 定義和實現的一致性

### 3. MongoDB 操作
- 確保使用 ObjectID 驗證
- 檢查是否正確處理 MongoDB 錯誤
- 驗證查詢的安全性（防注入）

### 4. SSE 實時通訊
- 檢查連接管理的正確性
- 確保心跳機制正常運作
- 驗證訊息推送的可靠性

### 5. 審計和日誌
- 確保敏感信息不記錄到日誌
- 檢查審計事件的完整性
- 驗證結構化日誌格式

## 配置檔案詳解

### 配置檔案位置
`chat-gateway/.golangci.yml`

### 主要配置項目

#### Linter 啟用設定
```yaml
linters:
  enable:
    - gofmt          # Go 官方格式化工具
    - goimports      # 自動管理 import 語句
    - gofumpt        # 更嚴格的格式化規則
    - govet          # Go 官方靜態分析
    - errcheck       # 檢查錯誤是否被正確處理
    - rowserrcheck   # 檢查 rows.Err() 是否被調用
    - staticcheck    # 進階靜態分析
    - gosimple       # 檢查可以簡化的程式碼
    - ineffassign    # 檢查無效的賦值
    - unused         # 檢查未使用的程式碼
    - misspell       # 檢查英文拼寫錯誤
    - gocyclo        # 檢查函數複雜度
    - dupl           # 檢測重複程式碼
    - goconst        # 檢測重複字串常數
    - gocritic       # 綜合程式碼風格檢查
    # - godot        # 檢查註解完整性和格式 (已停用)
    - goprintffuncname  # 檢查 printf 風格函數命名
    - lll            # 檢查行長度
    - nakedret       # 檢查裸返回語句
    - noctx          # 檢查沒有 context 的函數
    - nolintlint     # 檢查 nolint 指令的使用
    - stylecheck     # 檢查程式碼風格
    - unconvert      # 檢查不必要的類型轉換
    - unparam        # 檢查未使用的函數參數
    - whitespace     # 檢查空白字元使用
    - gosec          # 安全分析工具
```

#### 閾值設定
```yaml
linters-settings:
  gocyclo:
    min-complexity: 15  # 函數複雜度閾值
  dupl:
    threshold: 150      # 重複程式碼閾值（tokens），忽略小型 CRUD 重複
  lll:
    line-length: 140    # 行長度限制
  goconst:
    min-occurrences: 3  # 最少出現次數才警告
    min-length: 1       # 最小字串長度
```

## 常見問題和解決方案

### 1. 函數複雜度過高 (gocyclo)
**解決方案：** 拆分成多個函數

### 2. 重複程式碼 (dupl)
**解決方案：** 提取共用函數

### 3. 未使用的變數 (unused)
**解決方案：** 使用變數或移除

### 4. 錯誤處理不當 (errcheck)
**解決方案：** 正確處理錯誤

### 5. 安全性問題 (gosec)
**解決方案：** 使用安全的實踐

## 最佳實踐

### 1. 定期執行檢查
```bash
# 開發過程中定期執行
cd build && task check

# 或直接執行
golangci-lint run ./...
```

### 2. 逐步改善
- 不要一次修復所有問題
- 優先修復高嚴重程度的問題
- 逐步降低複雜度和重複度

### 3. 團隊協作
- 在 code review 時檢查 linter 結果
- 討論和統一程式碼風格
- 定期更新 linter 配置

### 4. 程式碼品質優先級
1. **高優先級**：安全性問題 (gosec)、錯誤處理 (errcheck)
2. **中優先級**：程式碼複雜度 (gocyclo)、重複程式碼 (dupl)
3. **低優先級**：風格問題 (whitespace)、行長度 (lll)

## 相關資源

- [golangci-lint 官方文檔](https://golangci-lint.run/)
- [Go 官方工具文檔](https://golang.org/cmd/)
- [staticcheck 文檔](https://staticcheck.io/)
- [gosec 安全檢查文檔](https://github.com/securecodewarrior/gosec)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

---

本指南會根據專案發展持續更新，如有問題請聯繫開發團隊。

