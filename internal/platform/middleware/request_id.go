package middleware

import (
	"github.com/gin-gonic/gin"
)

const (
	RequestIDHeader = "X-Request-ID"
	RequestIDKey    = "request_id"
)

// RequestIDMiddleware delegates to TraceMiddleware so trace_id and
// X-Request-ID are both set consistently.
func RequestIDMiddleware() gin.HandlerFunc {
	return TraceMiddleware()
}

// GetRequestID returns the trace_id stored in the gin context.
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get("trace_id"); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}
	return ""
}
