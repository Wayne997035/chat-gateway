package middleware

import (
	"encoding/hex"
	"strings"

	"chat-gateway/internal/platform/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TraceMiddleware sets a trace_id on every request.
//
// Priority order:
//  1. traceparent header (W3C): 00-<32hex>-<16hex>-<flags>
//  2. X-Request-ID header
//  3. Generated UUID (32 lowercase hex, no dashes)
func TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := extractTraceID(c)

		// Store in gin context for downstream handlers.
		c.Set("trace_id", traceID)

		// Propagate into request context so gRPC interceptors can read it.
		ctx := logger.WithTraceID(c.Request.Context(), traceID)
		c.Request = c.Request.WithContext(ctx)

		// Echo back so callers can correlate.
		c.Header("X-Request-ID", traceID)

		c.Next()
	}
}

// extractTraceID derives the trace ID from request headers or generates one.
func extractTraceID(c *gin.Context) string {
	// W3C traceparent: version-traceID-parentID-flags
	if tp := c.GetHeader("traceparent"); tp != "" {
		parts := strings.Split(tp, "-")
		if len(parts) == 4 && len(parts[1]) == 32 {
			return parts[1]
		}
	}

	if xid := c.GetHeader("X-Request-ID"); xid != "" {
		return xid
	}

	id := uuid.New()
	return hex.EncodeToString(id[:])
}
