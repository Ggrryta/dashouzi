package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"auction/pkg/errcode"
)

type Response struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

func Error(c *gin.Context, err error) {
	if err == nil {
		return
	}
	if ec, ok := err.(*errcode.ErrCode); ok {
		c.JSON(ec.HTTP, Response{Code: ec.Code, Message: ec.Message, Data: nil})
		return
	}
	c.JSON(errcode.ErrInternal.HTTP, Response{
		Code:    errcode.ErrInternal.Code,
		Message: errcode.ErrInternal.Message,
		Data:    nil,
	})
}

func ErrorWithMsg(c *gin.Context, err *errcode.ErrCode, msg string) {
	c.JSON(err.HTTP, Response{Code: err.Code, Message: msg, Data: nil})
}
