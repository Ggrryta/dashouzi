package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"bloger/pkg/errcode"
	"bloger/pkg/logger"
	"bloger/pkg/response"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Log.Error("panic recovered",
					zap.Any("error", r),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
				)

				response.Error(c, errcode.ErrInternal)
				c.Abort()
			}
		}()
		c.Next()
	}
}
