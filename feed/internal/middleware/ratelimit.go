package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter 固定窗口限流中间件
type RateLimiter struct {
	mu       sync.Mutex
	windows  map[string]*rateWindow
	limit    int
	interval time.Duration
}

type rateWindow struct {
	count    int
	resetAt  time.Time
}

// NewRateLimiter 创建限流器
// limit: 每个时间窗口内允许的请求数
// interval: 时间窗口大小
func NewRateLimiter(limit int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		windows:  make(map[string]*rateWindow),
		limit:    limit,
		interval: interval,
	}
}

func (rl *RateLimiter) key(c *gin.Context) string {
	// 按客户端 IP + 用户 ID 区分
	uid := c.GetHeader("X-User-ID")
	if uid == "" {
		uid = c.ClientIP()
	}
	return uid
}

func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	w, ok := rl.windows[key]
	if !ok || now.After(w.resetAt) {
		// 创建新窗口
		rl.windows[key] = &rateWindow{count: 1, resetAt: now.Add(rl.interval)}
		return true
	}

	if w.count >= rl.limit {
		return false
	}
	w.count++
	return true
}

// Gin中间件
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rl.key(c)
		if !rl.allow(key) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code": -1,
				"msg":  "rate limit exceeded",
			})
			return
		}
		c.Next()
	}
}
