package httputil

import "github.com/gin-gonic/gin"

// 成功訊息常數.
const (
	ImageUploadedSuccess = "Image uploaded successfully"
	DataRetrieved        = "Data retrieved successfully"
	DataCreated          = "Data created successfully"
	DataUpdated          = "Data updated successfully"
	DataDeleted          = "Data deleted successfully"
)

// 錯誤訊息常數.
const (
	InvalidParameter  = "Invalid parameter"
	InvalidFileFormat = "Invalid file format"
	FileTooLarge      = "File too large"
	ProcessingFailed  = "Processing failed"
	DatabaseError     = "Database error"
	NotFound          = "Not found"
	RecordNotFound    = "Record not found"
)

// Error 自定義錯誤結構.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Success 回傳簡單的成功訊息回應.
func Success(message string) gin.H {
	return gin.H{"message": message}
}

// SuccessWithCount 回傳包含計數的成功回應.
func SuccessWithCount(message string, count int) gin.H {
	return gin.H{
		"message": message,
		"count":   count,
	}
}

// ErrorMessage 回傳簡單的錯誤訊息回應.
func ErrorMessage(message string) gin.H {
	return gin.H{"error": message}
}

// ErrorWithCode 回傳包含錯誤代碼的錯誤回應.
func ErrorWithCode(code int, message string) gin.H {
	return gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	}
}

// ErrorWithCustomError 回傳自定義錯誤結構的回應.
func ErrorWithCustomError(err *Error) gin.H {
	return gin.H{
		"error": err,
	}
}

// SuccessResponse 成功回應結構.
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Count   int         `json:"count,omitempty"`
}

// NewSuccessResponse 創建成功回應.
func NewSuccessResponse(message string, data interface{}) *SuccessResponse {
	return &SuccessResponse{
		Message: message,
		Data:    data,
	}
}

// NewSuccessResponseWithCount 創建帶計數的成功回應.
func NewSuccessResponseWithCount(message string, count int) *SuccessResponse {
	return &SuccessResponse{
		Message: message,
		Count:   count,
	}
}
