package router

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"

	"seckill/internal/handler"
	"seckill/internal/middleware"
	"seckill/internal/repository"
	"seckill/internal/service"
	"seckill/pkg/response"
)

type Producer interface {
	Send(ctx context.Context, topic, key string, value []byte) error
}


func Setup(db *gorm.DB, redis repository.RedisClient, producer Producer) *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	// 初始化依赖（nil DB 仅用于 ping 测试）
	var sessionHandler *handler.SessionHandler
	var itemHandler *handler.ItemHandler
	var seckillHandler *handler.SeckillHandler

	if db != nil {
		sessionRepo := repository.NewSessionRepo(db)
		sessionSvc := service.NewSessionService(sessionRepo)
		sessionHandler = handler.NewSessionHandler(sessionSvc)

		itemRepo := repository.NewItemRepo(db)
		itemSvc := service.NewItemService(itemRepo, sessionRepo)
		itemHandler = handler.NewItemHandler(itemSvc, redis)

		// seckill 需要 RedisCmdClient
		if cmdRedis, ok := redis.(repository.RedisCmdClient); ok {
			seckillSvc := service.NewSeckillService(cmdRedis, producer)
			seckillHandler = handler.NewSeckillHandler(seckillSvc)
		}
	}

	r.GET("/api/v1/ping", func(c *gin.Context) {
		response.Success(c, "pong")
	})

	v1 := r.Group("/api/v1/seckill")
	if sessionHandler != nil {
		v1.GET("/sessions", sessionHandler.List)
		v1.GET("/sessions/:id", sessionHandler.Get)
		v1.POST("/sessions", sessionHandler.Create)
	}
	if itemHandler != nil {
		v1.GET("/items", itemHandler.List)
		v1.GET("/items/:id", itemHandler.Get)
		v1.POST("/items", itemHandler.Create)
		v1.GET("/items/by-session/:id", itemHandler.BySession)
		v1.POST("/items/:id/preheat", itemHandler.Preheat)
	}
	if seckillHandler != nil {
		buy := v1.Group("/buy")
		buy.Use(middleware.RateLimit(5, time.Second)) // 限流：5次/秒/用户
		{
			buy.POST("", seckillHandler.Buy)
		}
		v1.GET("/result/:item_id", seckillHandler.Result)
	}

	return r
}
