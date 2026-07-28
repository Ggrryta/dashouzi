package router

import (
	"testing"
	"net/http"
	"net/http/httptest"
	"encoding/json"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"seckill/pkg/logger"
)

func init() {
	logger.Init("error", "json")
}

func TestPing_ReturnsPong(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := Setup(nil, nil, nil)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ping", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, float64(0), resp["code"])
	assert.Equal(t, "pong", resp["data"])
}
