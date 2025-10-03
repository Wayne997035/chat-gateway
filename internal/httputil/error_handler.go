package httputil

import (
	"fmt"
	"strings"

	"chat-gateway/internal/platform/logger"
	"chat-gateway/internal/platform/middleware"

	"github.com/gin-gonic/gin"
)

// SafeError 安全的錯誤響應（不洩露內部信息）
func SafeError(c *gin.Context, statusCode int, err error, userMessage string) {
	requestID := middleware.GetRequestID(c)
	
	// 記錄真實錯誤到日誌（用於調試）
	logger.Error(c.Request.Context(), fmt.Sprintf("API Error: %v", err),
		logger.WithDetails(map[string]interface{}{
			"request_id": requestID,
			"path":       c.Request.URL.Path,
			"method":     c.Request.Method,
			"status":     statusCode,
		}))
	
	// 根據錯誤類型決定是否顯示詳情
	message := userMessage
	if shouldShowError(err) {
		message = err.Error()
	}
	
	c.JSON(statusCode, gin.H{
		"error":      message,
		"success":    false,
		"request_id": requestID, // 返回 request ID 便於追蹤
	})
}

// shouldShowError 判斷是否可以向用戶顯示錯誤詳情
func shouldShowError(err error) bool {
	if err == nil {
		return false
	}
	
	errMsg := err.Error()
	
	// 不應顯示的錯誤關鍵字（可能洩露敏感信息）
	dangerousKeywords := []string{
		"mongo",
		"database",
		"connection",
		"password",
		"token",
		"secret",
		"credential",
		"grpc",
		"internal",
		"stack",
		"panic",
	}
	
	lowerMsg := strings.ToLower(errMsg)
	for _, keyword := range dangerousKeywords {
		if strings.Contains(lowerMsg, keyword) {
			return false
		}
	}
	
	return true
}

// InternalServerError 內部服務器錯誤
func InternalServerError(c *gin.Context, err error) {
	SafeError(c, 500, err, "服務器內部錯誤，請稍後再試")
}

// BadRequest 錯誤的請求
func BadRequest(c *gin.Context, message string) {
	c.JSON(400, gin.H{
		"error":      message,
		"success":    false,
		"request_id": middleware.GetRequestID(c),
	})
}

// Unauthorized 未授權
func Unauthorized(c *gin.Context, message string) {
	if message == "" {
		message = "未授權訪問"
	}
	c.JSON(401, gin.H{
		"error":      message,
		"success":    false,
		"request_id": middleware.GetRequestID(c),
	})
}

// Forbidden 禁止訪問
func Forbidden(c *gin.Context, message string) {
	if message == "" {
		message = "禁止訪問"
	}
	c.JSON(403, gin.H{
		"error":      message,
		"success":    false,
		"request_id": middleware.GetRequestID(c),
	})
}

// NotFoundError 資源不存在
func NotFoundError(c *gin.Context, message string) {
	if message == "" {
		message = "資源不存在"
	}
	c.JSON(404, gin.H{
		"error":      message,
		"success":    false,
		"request_id": middleware.GetRequestID(c),
	})
}

// RateLimitExceeded 速率限制超過
func RateLimitExceeded(c *gin.Context) {
	c.JSON(429, gin.H{
		"error":      "請求過於頻繁，請稍後再試",
		"success":    false,
		"request_id": middleware.GetRequestID(c),
	})
}

// ValidationError 驗證錯誤
func ValidationError(c *gin.Context, field string, message string) {
	c.JSON(400, gin.H{
		"error":      fmt.Sprintf("%s: %s", field, message),
		"success":    false,
		"request_id": middleware.GetRequestID(c),
	})
}

