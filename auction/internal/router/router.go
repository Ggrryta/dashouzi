package router

import (
	"github.com/gin-gonic/gin"

	"auction/internal/handler"
	"auction/internal/middleware"
)

// NewRouter 创建路由器
func NewRouter(healthHandler *handler.HealthHandler, roomHandler *handler.RoomHandler, itemHandler *handler.ItemHandler) *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	// 探针
	r.GET("/health", healthHandler.Health)
	r.GET("/ready", healthHandler.Ready)
	r.GET("/startup", healthHandler.Startup)

	// 房间
	r.POST("/api/v1/rooms", middleware.Auth(), roomHandler.Create)
	r.GET("/api/v1/rooms", roomHandler.List)
	r.GET("/api/v1/rooms/:roomId", roomHandler.Get)
	r.DELETE("/api/v1/rooms/:roomId", middleware.Auth(), roomHandler.Close)

	// 商品
	r.POST("/api/v1/rooms/:roomId/items", middleware.Auth(), itemHandler.Create)
	r.GET("/api/v1/rooms/:roomId/items", itemHandler.ListByRoom)
	r.GET("/api/v1/items/:itemId", itemHandler.Get)
	r.DELETE("/api/v1/items/:itemId", middleware.Auth(), itemHandler.Delete)

	return r
}
