package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"seckill/pkg/errcode"
)

func setupGinContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	return c, w
}

func TestSuccess_ReturnsCodeZero(t *testing.T) {
	c, w := setupGinContext()
	Success(c, map[string]string{"key": "value"})
	assert.Equal(t, 200, w.Code)
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "ok", resp.Message)
}

func TestError_ReturnsCorrectCode(t *testing.T) {
	c, w := setupGinContext()
	Error(c, errcode.ErrNotFound)
	assert.Equal(t, 404, w.Code)
}

func TestError_SoldOut(t *testing.T) {
	c, w := setupGinContext()
	Error(c, errcode.ErrSoldOut)
	assert.Equal(t, 200, w.Code) // 秒杀售罄返回 200 但有业务错误码
}
