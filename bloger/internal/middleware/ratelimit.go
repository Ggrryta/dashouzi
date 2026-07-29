package middleware

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"bloger/pkg/errcode"
	"bloger/pkg/logger"
	"bloger/pkg/response"
)

// ---------------------------------------------------------------------------
// Limiter 接口：限流器抽象
// ---------------------------------------------------------------------------

// Limiter 限流器接口，抽象出 Allow 方法供中间件调用。
// 实现方可以是 Redis 分布式限流器，也可以是本地内存限流器（用于单元测试）。
type Limiter interface {
	// Allow 判断 key 是否放行。true=放行，false=超限。
	// 返回 error 表示限流器自身故障（如 Redis 不可达），由调用方决定 fail-open/fail-closed。
	Allow(ctx context.Context, key string) (bool, error)
}

// ---------------------------------------------------------------------------
// Redis 滑动窗口限流器
// ---------------------------------------------------------------------------

// 滑动窗口日志算法（Sliding Window Log）的 Lua 脚本。
//
// 原理：用 Redis Sorted Set 记录窗口内每次请求的时间戳（作为 score），
// 每次请求时：
//   1. ZREMRANGEBYSCORE 移除窗口外的过期请求
//   2. ZCARD 统计当前窗口内请求数
//   3. 未超限则 ZADD 写入本次请求并重置 TTL，超限则拒绝
//
// 整个流程在单个 Lua 脚本中执行，Redis 保证原子性，多实例下安全。
//
// 相比固定窗口，滑动窗口消除了"窗口边界 2x 突发"问题；
// 对于博客的低阈值场景（20-30/min），ZSET 内存开销可忽略。

const slidingWindowLua = `
local key    = KEYS[1]
local now    = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit  = tonumber(ARGV[3])
local member = ARGV[4]

-- 移除窗口外的过期请求（score <= now - window）
redis.call('ZREMRANGEBYSCORE', key, '-inf', now - window)

-- 统计当前窗口内请求数
local count = redis.call('ZCARD', key)

if count < limit then
    -- 写入本次请求，score 为当前时间戳
    redis.call('ZADD', key, now, member)
    -- 设置过期时间避免 key 永久残留（window + 1s 缓冲）
    redis.call('PEXPIRE', key, window + 1000)
    return 1
else
    return 0
end
`

// redisLimiter 基于 Redis 滑动窗口的分布式限流器。
type redisLimiter struct {
	client *redis.Client
	limit  int
	window time.Duration
	script *redis.Script // go-redis 内置 EVALSHA 优化，首次 EVAL 后后续走 EVALSHA
}

// NewRedisLimiter 创建一个 Redis 滑动窗口限流器。
// limit/window 对应每组路由的独立配额，多个限流器共享同一个 Redis 客户端。
func NewRedisLimiter(client *redis.Client, limit int, window time.Duration) Limiter {
	return &redisLimiter{
		client: client,
		limit:  limit,
		window: window,
		script: redis.NewScript(slidingWindowLua),
	}
}

func (l *redisLimiter) Allow(ctx context.Context, key string) (bool, error) {
	// 生成唯一 member，避免同毫秒请求在 ZSET 中互相覆盖
	member := fmt.Sprintf("%d:%d", time.Now().UnixNano(), rand.Int63())

	result, err := l.script.Run(ctx, l.client,
		[]string{key},
		time.Now().UnixMilli(),
		l.window.Milliseconds(),
		l.limit,
		member,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

// ---------------------------------------------------------------------------
// 本地内存限流器（用于单元测试，不依赖 Redis）
// ---------------------------------------------------------------------------

type rateEntry struct {
	count  int
	window time.Time
}

type localLimiter struct {
	mu          sync.Mutex
	entries     map[string]*rateEntry
	limit       int
	window      time.Duration
	lastCleanup time.Time
}

// NewLocalLimiter 创建本地内存限流器，仅在单元测试中使用。
// 生产环境请使用 NewRedisLimiter。
func NewLocalLimiter(limit int, window time.Duration) Limiter {
	return &localLimiter{
		entries: make(map[string]*rateEntry),
		limit:   limit,
		window:  window,
	}
}

func (l *localLimiter) Allow(ctx context.Context, key string) (bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	// 惰性清理过期 entry，避免内存泄漏
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
		return true, nil
	}

	entry.count++
	return entry.count <= l.limit, nil
}

// ---------------------------------------------------------------------------
// Gin 中间件
// ---------------------------------------------------------------------------

// RateLimit 限流中间件。
//
// 已认证用户按 user_id 限流，未认证按 IP；
// Redis 故障时 fail-open（放行 + 告警日志），避免限流依赖拖垮核心链路。
func RateLimit(limiter Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		var key string
		if uid, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("rl:u:%v:%s", uid, c.Request.URL.Path)
		} else {
			key = fmt.Sprintf("rl:ip:%s:%s", c.ClientIP(), c.Request.URL.Path)
		}

		allowed, err := limiter.Allow(c.Request.Context(), key)
		if err != nil {
			// fail-open：Redis 不可用时放行，避免限流故障导致服务不可用
			logger.Log.Warn("rate limiter unavailable, fail open",
				zap.String("key", key), zap.Error(err))
			c.Next()
			return
		}

		if !allowed {
			response.Error(c, errcode.ErrRateLimited)
			c.Abort()
			return
		}

		c.Next()
	}
}
