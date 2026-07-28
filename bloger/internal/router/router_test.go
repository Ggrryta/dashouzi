package router

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

func TestPing_ReturnsPong(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := jwt.New("test", 1)
	r := Setup(nil, j, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
	assert.Equal(t, "ok", resp["message"])
	assert.Equal(t, "pong", resp["data"])
}

func TestRegisterRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := jwt.New("test", 1)
	r := Setup(nil, j, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/users/register", nil)
	r.ServeHTTP(w, req)
	// 空 body 返回 400，路由至少注册成功
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthRoute_MissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	j := jwt.New("test", 1)
	r := Setup(nil, j, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/users/me", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
