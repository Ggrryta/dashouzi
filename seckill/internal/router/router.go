package router

import (
	"time"

	"gorm.io/gorm"

	"github.com/gin-gonic/gin"

	"seckill/internal/handler"
	"seckill/internal/middleware"
	"seckill/internal/repository"
	"seckill/internal/service"
	"seckill/pkg/response"
)

// Setup 组装依赖并注册路由。
//   - db 为 nil 时仅注册 ping（供单元测试）
//   - authSecret 为空时 buy/result 路由不挂鉴权（仅测试场景）
//   - orderTopic 注入到秒杀服务用于 Kafka 投递
func Setup(db *gorm.DB, redis repository.RedisClient, producer service.Producer, authSecret, orderTopic string) *gin.Engine {
	r := gin.New()

	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	var sessionHandler *handler.SessionHandler
	var itemHandler *handler.ItemHandler
	var seckillHandler *handler.SeckillHandler
	var cmdRedis repository.RedisCmdClient

	if redis != nil {
		if c, ok := redis.(repository.RedisCmdClient); ok {
			cmdRedis = c
		}
	}

	if db != nil {
		sessionRepo := repository.NewSessionRepo(db)
		sessionSvc := service.NewSessionService(sessionRepo)
		sessionHandler = handler.NewSessionHandler(sessionSvc)

		itemRepo := repository.NewItemRepo(db)
		itemSvc := service.NewItemService(itemRepo, sessionRepo)
		itemHandler = handler.NewItemHandler(itemSvc, redis)

		if cmdRedis != nil {
			validator := service.NewItemSessionValidator(itemRepo, sessionRepo)
			seckillSvc := service.NewSeckillServiceWithValidation(cmdRedis, producer, validator, orderTopic)
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
		auth := middleware.Auth(authSecret)
		buy := v1.Group("/buy")
		buy.Use(auth, middleware.RateLimit(cmdRedis, 5, time.Second)) // 限流：5次/秒/用户
		{
			buy.POST("", seckillHandler.Buy)
		}
		v1.GET("/result/:item_id", auth, seckillHandler.Result)
	}

	return r
}
