# 測試 (Tests)

這個目錄包含了專案的測試檔案，包括單元測試和整合測試。

## 目錄結構

```
tests/
├── README.md              # 測試說明文件
├── integration/           # 整合測試
│   └── test_container.go  # 測試容器設置
└── test_data/             # 測試資料目錄（存放測試所需的靜態資料檔案）
```

## 測試類型

### 整合測試 (Integration Tests)
- 測試多個組件之間的互動
- 使用真實的資料庫和外部服務
- 驗證完整的業務流程

## 執行測試

### 執行整合測試
```bash
go test ./tests/integration/... -v
```

## 測試環境設置

### 整合測試環境
整合測試使用 `testcontainers` 來創建隔離的測試環境：
- 獨立的 MongoDB 容器
- 動態分配的端口
- 自動清理測試資源

### 環境變數
測試會使用專門的測試配置：
- 測試專用的資料庫
- 測試專用的日誌設置
- 隔離的測試環境

## 測試資料

### test_data/ 目錄
`test_data/` 目錄用於存放測試所需的靜態資料檔案。開發者可以根據自己的測試需求在此目錄中放置各種格式的測試資料。

#### 注意事項
- 不要使用真實的個人資料
- 使用虛擬的測試資料
- 確保資料的匿名性
- 控制測試資料的大小
- 定期清理過時的測試資料

## 依賴要求

### 整合測試
- Docker Desktop 正在運行
- Go 1.24+
- testcontainers-go v0.38.0+
- testify v1.10.0+

## 故障排除

### Docker 相關問題
```
Error: failed to start container
```
解決方案：確保 Docker Desktop 正在運行

### 測試資料問題
```
Error: test data file not found
```
解決方案：檢查 `test_data/` 目錄中的檔案是否存在

### 端口衝突
```
Error: port already in use
```
解決方案：等待一段時間後重試，或檢查是否有其他服務佔用端口

## 持續整合

測試會在以下情況自動執行：
- 代碼提交到版本控制
- 創建 Pull Request
- 定期構建

確保所有測試都通過後才能合併代碼。

## 測試覆蓋率

查看測試覆蓋率：
```bash
go test ./tests/... -cover
go test ./tests/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

目標測試覆蓋率：> 80%
