package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"bloger/pkg/jwt"
	"bloger/pkg/logger"
)

func init() {
	logger.Init("error", "json")
}

func setupJWT() *jwt.JWT {
	return jwt.New("test-secret-for-middleware", 24)
}

func TestAuth_MissingToken_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(setupJWT()))
	r.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(20006), resp["code"]) // missing auth header
}

func TestAuth_InvalidToken_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(setupJWT()))
	r.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ExpiredToken_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := jwt.New("test-secret", -1) // 立即过期
	r := gin.New()
	r.Use(Auth(j))
	r.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	token, _ := j.GenerateToken(1, "test", "reader")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ValidToken_SetsContextAndPasses(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := setupJWT()
	r := gin.New()
	r.Use(Auth(j))
	r.GET("/me", func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		role, _ := c.Get("role")
		c.JSON(200, gin.H{
			"user_id": userID,
			"role":    role,
		})
	})

	token, _ := j.GenerateToken(42, "admin", "admin")

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(42), resp["user_id"])
	assert.Equal(t, "admin", resp["role"])
}

func TestAuth_WrongAuthFormat_Returns401(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth(setupJWT()))
	r.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Basic abc123")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
