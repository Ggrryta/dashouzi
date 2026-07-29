package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 单元测试使用 localLimiter（内存实现），不依赖 Redis。
// Redis 限流器的集成测试见 build tag "integration"，通过 Docker 运行。

func TestRateLimit_UnderLimit_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(NewLocalLimiter(10, time.Minute)))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_Exceeded_Returns429(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limit := 3
	r := gin.New()
	r.Use(RateLimit(NewLocalLimiter(limit, time.Minute)))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// 发 limit 次请求，都应成功
	for i := 0; i < limit; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
		assert.Equal(t, 200, w.Code, "request %d should succeed", i+1)
	}

	// 第 limit+1 次应被拒绝
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(10006), resp["code"])
}

func TestRateLimit_DifferentPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(NewLocalLimiter(1, time.Minute)))
	r.GET("/a", func(c *gin.Context) { c.JSON(200, gin.H{}) })
	r.GET("/b", func(c *gin.Context) { c.JSON(200, gin.H{}) })

	// /a 用掉配额
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/a", nil))
	assert.Equal(t, 200, w1.Code)

	// /b 应有自己的配额
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest("GET", "/b", nil))
	assert.Equal(t, 200, w2.Code)
}

// TestRateLimit_LimiterError_FailOpen 验证 Limiter 返回 error 时 fail-open 放行。
func TestRateLimit_LimiterError_FailOpen(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RateLimit(&errorLimiter{}))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/test", nil))

	// Limiter 报错时应 fail-open，返回 200
	assert.Equal(t, http.StatusOK, w.Code)
}

// errorLimiter 始终返回 error，用于测试 fail-open 逻辑。
type errorLimiter struct{}

func (e *errorLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return false, fmt.Errorf("redis unavailable")
}
