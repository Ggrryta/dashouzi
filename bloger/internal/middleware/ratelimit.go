package middleware

import (
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"bloger/pkg/errcode"
	"bloger/pkg/response"
)

type rateEntry struct {
	count  int
	window time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateEntry
	limit   int
	window  time.Duration
}

var (
	limiters   = make(map[string]*rateLimiter)
	limitersMu sync.Mutex
	limiterSeq int
)

// RateLimit 基于固定窗口的限流中间件。每次调用创建独立的限流器实例。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	limitersMu.Lock()
	limiterSeq++
	id := fmt.Sprintf("rl-%d", limiterSeq)
	l := &rateLimiter{
		entries: make(map[string]*rateEntry),
		limit:   limit,
		window:  window,
	}
	limiters[id] = l
	limitersMu.Unlock()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		path := c.Request.URL.Path
		entryKey := ip + ":" + path

		l.mu.Lock()
		defer l.mu.Unlock()

		now := time.Now()
		entry, exists := l.entries[entryKey]

		if !exists || now.Sub(entry.window) > l.window {
			l.entries[entryKey] = &rateEntry{count: 1, window: now}
			c.Next()
			return
		}

		entry.count++
		if entry.count > l.limit {
			response.Error(c, errcode.ErrRateLimited)
			c.Abort()
			return
		}

		c.Next()
	}
}
