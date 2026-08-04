package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"auction/pkg/logger"
)

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		cost := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()
		errors := c.Errors.ByType(gin.ErrorTypePrivate).String()

		fields := []zap.Field{
			zap.Int("status", status),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("client_ip", clientIP),
			zap.Duration("cost", cost),
		}
		if errors != "" {
			fields = append(fields, zap.String("errors", errors))
		}

		if status >= 500 {
			logger.Log.Error("server error", fields...)
		} else if status >= 400 {
			logger.Log.Warn("client error", fields...)
		} else {
			logger.Log.Info("request", fields...)
		}
	}
}
