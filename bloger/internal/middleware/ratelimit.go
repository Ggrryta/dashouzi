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
	mu          sync.Mutex
	entries     map[string]*rateEntry
	limit       int
	window      time.Duration
	lastCleanup time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		entries: make(map[string]*rateEntry),
		limit:   limit,
		window:  window,
	}
}

// allow 判断是否放行。true=放行,false=超限。
func (l *rateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// M2: 惰性清理过期 entry,避免内存泄漏(每分钟至多一次)
	if now.Sub(l.lastCleanup) > time.Minute {
		for k, e := range l.entries {
			if now.Sub(e.window) > l.window {
				delete(l.entries, k)
			}
		}
		l.lastCleanup = now
	}

	entry, exists := l.entries[key]
	if !exists || now.Sub(entry.window) > l.window {
		l.entries[key] = &rateEntry{count: 1, window: now}
		return true
	}

	entry.count++
	return entry.count <= l.limit
}

// RateLimit 基于固定窗口的限流中间件。
// M2: 已认证用户按 user_id 限流(符合 DESIGN 评论按用户限流),未认证按 IP;
// 惰性清理过期条目防内存泄漏;移除原全局 limiters map 泄漏。
func RateLimit(limit int, window time.Duration) gin.HandlerFunc {
	l := newRateLimiter(limit, window)

	return func(c *gin.Context) {
		var key string
		if uid, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("u:%v:%s", uid, c.Request.URL.Path)
		} else {
			key = fmt.Sprintf("ip:%s:%s", c.ClientIP(), c.Request.URL.Path)
		}

		if !l.allow(key) {
			response.Error(c, errcode.ErrRateLimited)
			c.Abort()
			return
		}

		c.Next()
	}
}
