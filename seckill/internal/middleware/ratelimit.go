package middleware

import (
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"seckill/pkg/errcode"
	"seckill/pkg/response"
)

type rateEntry struct {
	count  int
	window time.Time
}

// RateLimit 固定窗口限流：limit 次 / window 时间内。
// key = IP + Path，每个 key 独立计数。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	entries := make(map[string]*rateEntry)

	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.Request.URL.Path

		mu.Lock()
		defer mu.Unlock()

		now := time.Now()
		e, exist := entries[key]

		if !exist || now.Sub(e.window) > window {
			entries[key] = &rateEntry{count: 1, window: now}
			c.Next()
			return
		}

		e.count++
		if e.count > limit {
			response.Error(c, errcode.ErrRateLimited)
			c.Abort()
			return
		}

		c.Next()
	}
}
