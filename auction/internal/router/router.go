package router

import (
	"github.com/gin-gonic/gin"

	"auction/internal/handler"
	"auction/internal/middleware"
)

// NewRouter 创建路由器
func NewRouter(healthHandler *handler.HealthHandler) *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	// 探针
	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Ready)
	r.GET("/startup", healthHandler.Startup)

	return r
}
