package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// T4.11: 限流中间件 — 超频返回 429
func TestRateLimit_Exceeded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	limiter := NewRateLimiter(5, time.Minute)
	router.Use(limiter.Middleware())
	router.GET("/test", func(c *gin.Context) { c.Status(200) })

	srv := httptest.NewServer(router)
	defer srv.Close()

	// 前 5 次成功
	for i := 0; i < 5; i++ {
		resp, err := http.Get(srv.URL + "/test")
		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode,
			"request %d should pass, got %d", i+1, resp.StatusCode)
		resp.Body.Close()
	}

	// 第 6 次返回 429 Too Many Requests
	resp, err := http.Get(srv.URL + "/test")
	assert.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode, "6th request should be rate limited")
	resp.Body.Close()
}

// 不同用户独立限流
func TestRateLimit_PerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	limiter := NewRateLimiter(3, time.Minute)
	router.Use(limiter.Middleware())
	router.GET("/test", func(c *gin.Context) { c.Status(200) })

	srv := httptest.NewServer(router)
	defer srv.Close()

	// 用户1消耗限额
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", srv.URL+"/test", nil)
		req.Header.Set("X-User-ID", "1")
		resp, _ := http.DefaultClient.Do(req)
		assert.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()
	}

	// 用户1超频
	req1, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req1.Header.Set("X-User-ID", "1")
	resp1, _ := http.DefaultClient.Do(req1)
	assert.Equal(t, 429, resp1.StatusCode)
	resp1.Body.Close()

	// 用户2不受影响
	req2, _ := http.NewRequest("GET", srv.URL+"/test", nil)
	req2.Header.Set("X-User-ID", "2")
	resp2, _ := http.DefaultClient.Do(req2)
	assert.Equal(t, 200, resp2.StatusCode, "user 2 should not be rate limited")
	resp2.Body.Close()
}
