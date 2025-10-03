package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// SSEConnectionLimiter SSE 連接限制器
type SSEConnectionLimiter struct {
	mu                sync.RWMutex
	connections       map[string]int       // IP -> 連接數
	lastConnect       map[string]time.Time // IP -> 最後連接時間
	maxPerIP          int                  // 每個 IP 最大連接數
	minInterval       time.Duration        // 最小連接間隔
	cleanupInterval   time.Duration        // 清理間隔
	maxTotalConns     int                  // 全局最大連接數
	currentTotalConns int                  // 當前總連接數
}

// NewSSEConnectionLimiter 創建 SSE 連接限制器
func NewSSEConnectionLimiter(maxPerIP int, minInterval time.Duration, maxTotal int) *SSEConnectionLimiter {
	limiter := &SSEConnectionLimiter{
		connections:     make(map[string]int),
		lastConnect:     make(map[string]time.Time),
		maxPerIP:        maxPerIP,
		minInterval:     minInterval,
		cleanupInterval: 5 * time.Minute,
		maxTotalConns:   maxTotal,
	}

	// 啟動定期清理
	go limiter.cleanup()

	return limiter
}

// Middleware SSE 連接限制中間件
func (l *SSEConnectionLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		// 檢查是否允許連接
		if !l.allowConnection(clientIP) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "SSE 連接數已達上限，請稍後再試",
				"success": false,
			})
			c.Abort()
			return
		}

		// 註冊連接
		l.registerConnection(clientIP)

		// 使用 defer 確保連接關閉時移除
		c.Writer.Header().Set("X-SSE-Connection-Registered", "true")

		// 創建一個完成通道
		done := make(chan struct{})

		// 在連接關閉時清理
		go func() {
			<-c.Request.Context().Done()
			close(done)
			l.removeConnection(clientIP)
		}()

		c.Next()
	}
}

// allowConnection 檢查是否允許建立新連接
func (l *SSEConnectionLimiter) allowConnection(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 檢查全局連接數限制
	if l.currentTotalConns >= l.maxTotalConns {
		return false
	}

	// 檢查單個 IP 連接數
	if count, exists := l.connections[ip]; exists && count >= l.maxPerIP {
		return false
	}

	// 檢查連接頻率
	if lastTime, exists := l.lastConnect[ip]; exists {
		if time.Since(lastTime) < l.minInterval {
			return false
		}
	}

	return true
}

// registerConnection 註冊新連接
func (l *SSEConnectionLimiter) registerConnection(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.connections[ip]++
	l.currentTotalConns++
	l.lastConnect[ip] = time.Now()
}

// removeConnection 移除連接
func (l *SSEConnectionLimiter) removeConnection(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if count, exists := l.connections[ip]; exists {
		if count <= 1 {
			delete(l.connections, ip)
		} else {
			l.connections[ip]--
		}
		l.currentTotalConns--
	}
}

// cleanup 定期清理過期數據
func (l *SSEConnectionLimiter) cleanup() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for ip, lastTime := range l.lastConnect {
			// 清理 10 分鐘無活動的記錄
			if now.Sub(lastTime) > 10*time.Minute {
				delete(l.lastConnect, ip)
				// 如果連接數為 0，也刪除
				if count, exists := l.connections[ip]; !exists || count == 0 {
					delete(l.connections, ip)
				}
			}
		}
		l.mu.Unlock()
	}
}

// Stats 獲取統計信息
func (l *SSEConnectionLimiter) Stats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return map[string]interface{}{
		"total_connections": l.currentTotalConns,
		"unique_ips":        len(l.connections),
		"max_total":         l.maxTotalConns,
		"max_per_ip":        l.maxPerIP,
	}
}

