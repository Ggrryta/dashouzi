package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"seckill/pkg/logger"
	"seckill/pkg/response"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Log.Error("panic recovered",
					zap.Any("error", r),
					zap.String("path", c.Request.URL.Path),
				)
				c.JSON(http.StatusInternalServerError, gin.H{
					"code": 50000, "message": "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// 确保接口实现
var _ = response.Success
