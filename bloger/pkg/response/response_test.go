package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"bloger/pkg/errcode"
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

	assert.Equal(t, http.StatusOK, w.Code)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "ok", resp.Message)
	assert.Equal(t, map[string]interface{}{"key": "value"}, resp.Data)
}

func TestSuccess_WithNilData(t *testing.T) {
	c, w := setupGinContext()

	Success(c, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 0, resp.Code)
	assert.Equal(t, "ok", resp.Message)
	assert.Nil(t, resp.Data)
}

func TestError_ReturnsCorrectStatusCode(t *testing.T) {
	c, w := setupGinContext()

	Error(c, errcode.ErrNotFound)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, errcode.ErrNotFound.Code, resp.Code)
	assert.Equal(t, errcode.ErrNotFound.Message, resp.Message)
	assert.Nil(t, resp.Data)
}

func TestError_InternalError(t *testing.T) {
	c, w := setupGinContext()

	Error(c, errcode.ErrInternal)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, 10000, resp.Code)
}

func TestError_UsesErrCodeMsg(t *testing.T) {
	c, w := setupGinContext()

	Error(c, errcode.ErrForbidden)

	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "forbidden", resp.Message)
}

func TestErrorWithMsg_OverridesMessage(t *testing.T) {
	c, w := setupGinContext()

	ErrorWithMsg(c, errcode.ErrBadRequest, "custom error message")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, errcode.ErrBadRequest.Code, resp.Code)
	assert.Equal(t, "custom error message", resp.Message)
	assert.Nil(t, resp.Data)
}

func TestErrorWithMsg_UsesErrCodeHTTP(t *testing.T) {
	c, w := setupGinContext()

	ErrorWithMsg(c, errcode.ErrInternal, "something broke")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
