package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRole_ReaderAccessLowestRoute_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "reader")
		c.Next()
	})
	r.Use(Role("reader")) // 要求 reader
	r.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRole_ReaderAccessAuthorRoute_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "reader")
		c.Next()
	})
	r.Use(Role("author"))
	r.GET("/write", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/write", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(10004), resp["code"]) // forbidden
}

func TestRole_AuthorAccessAuthorRoute_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "author")
		c.Next()
	})
	r.Use(Role("author"))
	r.GET("/write", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/write", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRole_AdminAccessAnyRoute_Allowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.Use(Role("admin"))
	r.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/admin", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRole_AdminAccessLowerRole_AlsoAllowed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.Use(Role("author")) // admin >= author
	r.GET("/write", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/write", nil))

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRole_AuthorAccessAdminRoute_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "author")
		c.Next()
	})
	r.Use(Role("admin")) // author < admin
	r.GET("/admin", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/admin", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRole_NoRoleInContext_Returns403(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Role("reader"))
	r.GET("/api", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api", nil))

	assert.Equal(t, http.StatusForbidden, w.Code)
}
