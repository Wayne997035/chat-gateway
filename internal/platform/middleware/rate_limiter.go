package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 速率限制器
type RateLimiter struct {
	visitors map[string]*Visitor
	mu       sync.RWMutex
	rate     int           // 每個時間窗口允許的請求數
	window   time.Duration // 時間窗口
}

// Visitor 訪問者信息
type Visitor struct {
	lastSeen  time.Time
	requests  int
	resetTime time.Time
}

// NewRateLimiter 創建新的速率限制器
// rate: 每個時間窗口允許的請求數
// window: 時間窗口（例如：time.Minute）
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*Visitor),
		rate:     rate,
		window:   window,
	}

	// 啟動清理 goroutine，定期清理過期的訪問者記錄
	go rl.cleanupVisitors()

	return rl
}

// Middleware 返回 Gin 中間件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 獲取客戶端 IP
		ip := c.ClientIP()

		// 檢查是否超過速率限制
		if !rl.allowRequest(ip) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "請求過於頻繁，請稍後再試",
				"success": false,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// allowRequest 檢查是否允許請求
func (rl *RateLimiter) allowRequest(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	visitor, exists := rl.visitors[ip]

	if !exists {
		// 新訪問者
		rl.visitors[ip] = &Visitor{
			lastSeen:  now,
			requests:  1,
			resetTime: now.Add(rl.window),
		}
		return true
	}

	// 檢查時間窗口是否已過期
	if now.After(visitor.resetTime) {
		// 重置計數器
		visitor.requests = 1
		visitor.resetTime = now.Add(rl.window)
		visitor.lastSeen = now
		return true
	}

	// 檢查是否超過速率限制
	if visitor.requests >= rl.rate {
		visitor.lastSeen = now
		return false
	}

	// 增加請求計數
	visitor.requests++
	visitor.lastSeen = now
	return true
}

// cleanupVisitors 定期清理過期的訪問者記錄
func (rl *RateLimiter) cleanupVisitors() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		for ip, visitor := range rl.visitors {
			// 如果訪問者超過 10 分鐘沒有活動，刪除記錄
			if now.Sub(visitor.lastSeen) > 10*time.Minute {
				delete(rl.visitors, ip)
			}
		}

		rl.mu.Unlock()
	}
}

// PerEndpointRateLimiter 為不同端點設置不同的速率限制
type PerEndpointRateLimiter struct {
	limiters map[string]*RateLimiter
	default_ *RateLimiter
}

// NewPerEndpointRateLimiter 創建端點級速率限制器
func NewPerEndpointRateLimiter(defaultRate int, defaultWindow time.Duration) *PerEndpointRateLimiter {
	return &PerEndpointRateLimiter{
		limiters: make(map[string]*RateLimiter),
		default_: NewRateLimiter(defaultRate, defaultWindow),
	}
}

// SetLimit 為特定端點設置限制
func (p *PerEndpointRateLimiter) SetLimit(path string, rate int, window time.Duration) {
	p.limiters[path] = NewRateLimiter(rate, window)
}

// Middleware 返回 Gin 中間件
func (p *PerEndpointRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// 檢查是否有特定端點的限制器
		if limiter, exists := p.limiters[path]; exists {
			ip := c.ClientIP()
			if !limiter.allowRequest(ip) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "請求過於頻繁，請稍後再試",
					"success": false,
				})
				c.Abort()
				return
			}
		} else {
			// 使用默認限制器
			ip := c.ClientIP()
			if !p.default_.allowRequest(ip) {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":   "請求過於頻繁，請稍後再試",
					"success": false,
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

