package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"chat-gateway/internal/platform/logger"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
	logger.Init("debug", true)
}

func TestTraceMiddleware_GeneratesID(t *testing.T) {
	r := gin.New()
	r.Use(TraceMiddleware())
	r.GET("/", func(c *gin.Context) {
		id, exists := c.Get("trace_id")
		if !exists {
			c.JSON(500, gin.H{"error": "no trace_id"})
			return
		}
		c.JSON(200, gin.H{"id": id})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	xid := w.Header().Get("X-Request-ID")
	if xid == "" {
		t.Error("X-Request-ID header must be set")
	}
	if len(xid) != 32 {
		t.Errorf("generated trace_id should be 32 hex chars, got %q (len=%d)", xid, len(xid))
	}
}

func TestTraceMiddleware_ReadsXRequestID(t *testing.T) {
	r := gin.New()
	r.Use(TraceMiddleware())
	r.GET("/", func(c *gin.Context) {
		id, _ := c.Get("trace_id")
		c.JSON(200, gin.H{"id": id})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Request-ID", "custom-request-id")
	r.ServeHTTP(w, req)

	xid := w.Header().Get("X-Request-ID")
	if xid != "custom-request-id" {
		t.Errorf("expected trace_id to be 'custom-request-id', got %q", xid)
	}
}

func TestTraceMiddleware_ReadsTraceparent(t *testing.T) {
	r := gin.New()
	r.Use(TraceMiddleware())
	r.GET("/", func(c *gin.Context) {
		id, _ := c.Get("trace_id")
		c.JSON(200, gin.H{"id": id})
	})

	const traceID = "4bf92f3577b34da6a3ce929d0e0e4736"
	traceparent := "00-" + traceID + "-00f067aa0ba902b7-01"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("traceparent", traceparent)
	r.ServeHTTP(w, req)

	xid := w.Header().Get("X-Request-ID")
	if xid != traceID {
		t.Errorf("expected trace_id %q from traceparent, got %q", traceID, xid)
	}
}

func TestTraceMiddleware_InvalidTraceparentFallsBack(t *testing.T) {
	r := gin.New()
	r.Use(TraceMiddleware())
	r.GET("/", func(c *gin.Context) {
		id, _ := c.Get("trace_id")
		c.JSON(200, gin.H{"id": id})
	})

	cases := []struct {
		name       string
		header     string
		wantXReqID string // empty means "should auto-generate"
	}{
		{
			name:       "malformed traceparent - too few parts",
			header:     "00-abc-01",
			wantXReqID: "",
		},
		{
			name:       "malformed traceparent - wrong trace id length",
			header:     "00-abc123-00f067aa0ba902b7-01",
			wantXReqID: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set("traceparent", tc.header)
			r.ServeHTTP(w, req)

			xid := w.Header().Get("X-Request-ID")
			if xid == "" {
				t.Error("should have generated a trace_id as fallback")
			}
		})
	}
}

func TestTraceMiddleware_SetsContextTraceID(t *testing.T) {
	r := gin.New()
	r.Use(TraceMiddleware())
	r.GET("/", func(c *gin.Context) {
		ctxID := logger.GetTraceID(c.Request.Context())
		ginID, _ := c.Get("trace_id")
		if ctxID != ginID {
			c.JSON(500, gin.H{"error": "mismatch between ctx and gin trace_id"})
			return
		}
		c.JSON(200, gin.H{"id": ctxID})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Request-ID", "test-trace-123")
	r.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("ctx and gin trace_id should match: %s", w.Body.String())
	}
}

func TestExtractTraceID_Priority(t *testing.T) {
	cases := []struct {
		name            string
		traceparent     string
		xRequestID      string
		wantTracePrefix string // prefix to check or "generated"
	}{
		{
			name:            "traceparent takes priority over X-Request-ID",
			traceparent:     "00-aabbccddeeff00112233445566778899-00f067aa0ba902b7-01",
			xRequestID:      "should-be-ignored",
			wantTracePrefix: "aabbccddeeff00112233445566778899",
		},
		{
			name:            "X-Request-ID used when no traceparent",
			xRequestID:      "my-request-id",
			wantTracePrefix: "my-request-id",
		},
		{
			name:            "auto-generated when nothing provided",
			wantTracePrefix: "", // non-empty auto-generated value
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			if tc.traceparent != "" {
				req.Header.Set("traceparent", tc.traceparent)
			}
			if tc.xRequestID != "" {
				req.Header.Set("X-Request-ID", tc.xRequestID)
			}
			ctx.Request = req

			id := extractTraceID(ctx)
			if id == "" {
				t.Fatal("extractTraceID must never return empty string")
			}
			if tc.wantTracePrefix != "" && !strings.HasPrefix(id, tc.wantTracePrefix) {
				t.Errorf("extractTraceID() = %q, want prefix %q", id, tc.wantTracePrefix)
			}
		})
	}
}
