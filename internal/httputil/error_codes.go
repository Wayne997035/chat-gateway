package httputil

// API 錯誤代碼常數.
const (
	// 1000-1999: 認證相關錯誤 (401 Unauthorized).
	ErrorCodeMissingAuthHeader = 1001
	ErrorCodeInvalidAuthFormat = 1002
	ErrorCodeInvalidAuthHeader = 1003

	// 2000-2999: 參數相關錯誤 (400 Bad Request).
	ErrorCodeInvalidParameter = 2001

	// 4000-4999: 資源相關錯誤 (404 Not Found).
	ErrorCodeRecordNotFound = 4001

	// 5000-5999: 處理相關錯誤 (500 Internal Server Error).
	ErrorCodeProcessingFailed = 5001
)
