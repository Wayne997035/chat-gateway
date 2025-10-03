package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	RequestIDHeader = "X-Request-ID"
	RequestIDKey    = "request_id"
)

// RequestIDMiddleware 為每個請求生成唯一 ID
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 優先使用客戶端提供的 Request ID（如果有的話）
		requestID := c.GetHeader(RequestIDHeader)
		
		// 如果客戶端沒有提供，生成新的 UUID
		if requestID == "" {
			requestID = uuid.New().String()
		}
		
		// 將 Request ID 設置到 context
		c.Set(RequestIDKey, requestID)
		
		// 將 Request ID 添加到響應頭
		c.Header(RequestIDHeader, requestID)
		
		c.Next()
	}
}

// GetRequestID 從 context 獲取 Request ID
func GetRequestID(c *gin.Context) string {
	if requestID, exists := c.Get(RequestIDKey); exists {
		if id, ok := requestID.(string); ok {
			return id
		}
	}
	return ""
}

