package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"im/pkg/errcode"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

func Error(c *gin.Context, err *errcode.ErrCode) {
	c.JSON(err.HTTP, Response{Code: err.Code, Message: err.Message, Data: nil})
}
