package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
)

// RequestMetadata 請求元數據
type RequestMetadata struct {
	IPAddress string
	UserAgent string
	UserID    string
}

// Context keys
type contextKey string

const (
	requestMetadataKey contextKey = "request_metadata"
)

// RequestMetadataMiddleware 提取請求元數據並存儲到 context
func RequestMetadataMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		metadata := &RequestMetadata{
			IPAddress: GetClientIP(c),
			UserAgent: c.Request.UserAgent(),
			UserID:    c.Query("user_id"), // 也可以從 JWT token 中提取
		}

		// 將元數據添加到 gin.Context
		c.Set(string(requestMetadataKey), metadata)

		// 將元數據添加到 context.Context
		ctx := context.WithValue(c.Request.Context(), requestMetadataKey, metadata)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// GetClientIP 獲取客戶端真實 IP
func GetClientIP(c *gin.Context) string {
	// 優先從 X-Forwarded-For 頭部獲取（反向代理）
	if forwarded := c.Request.Header.Get("X-Forwarded-For"); forwarded != "" {
		// X-Forwarded-For 可能包含多個 IP，取第一個
		return forwarded
	}

	// 從 X-Real-IP 頭部獲取
	if realIP := c.Request.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}

	// 直接獲取遠程地址
	return c.ClientIP()
}

// GetRequestMetadata 從 context 獲取請求元數據
func GetRequestMetadata(ctx context.Context) *RequestMetadata {
	if metadata, ok := ctx.Value(requestMetadataKey).(*RequestMetadata); ok {
		return metadata
	}
	return &RequestMetadata{
		IPAddress: "unknown",
		UserAgent: "unknown",
	}
}

// GetRequestMetadataFromGin 從 gin.Context 獲取請求元數據
func GetRequestMetadataFromGin(c *gin.Context) *RequestMetadata {
	if metadata, exists := c.Get(string(requestMetadataKey)); exists {
		if meta, ok := metadata.(*RequestMetadata); ok {
			return meta
		}
	}
	return &RequestMetadata{
		IPAddress: GetClientIP(c),
		UserAgent: c.Request.UserAgent(),
	}
}
