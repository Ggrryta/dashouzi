package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"seckill/internal/repository"
	"seckill/pkg/errcode"
	"seckill/pkg/response"
)

// rateLimitLua 原子计数 + 首次设过期：返回当前窗口计数。
const rateLimitLua = `local n = redis.call('incr', KEYS[1])
if n == 1 then
	redis.call('expire', KEYS[1], ARGV[1])
end
return n`

// RateLimit 基于 Redis 的分布式固定窗口限流：limit 次 / window 时间内。
// key 维度为 用户ID(已鉴权) 或 IP(未鉴权) + Path，多实例共享计数。
// Redis 故障时放行（可用性优先于限流），避免拖垮整个秒杀链路。
func RateLimit(redis repository.RedisCmdClient, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if redis == nil {
			c.Next()
			return
		}

		ident := c.ClientIP()
		if uid, ok := UserIDFromContext(c); ok {
			ident = "u:" + strconv.FormatUint(uint64(uid), 10)
		}
		key := "seckill:rl:" + ident + ":" + c.Request.URL.Path

		n, err := redis.Eval(c.Request.Context(), rateLimitLua,
			[]string{key}, int(window.Seconds()))
		if err != nil {
			c.Next()
			return
		}

		if n > int64(limit) {
			response.Error(c, errcode.ErrRateLimited)
			c.Abort()
			return
		}
		c.Next()
	}
}
