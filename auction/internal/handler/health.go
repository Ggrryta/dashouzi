package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"auction/pkg/errcode"
	"auction/pkg/redis"
	"auction/pkg/response"
)

type HealthHandler struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewHealthHandler(db *gorm.DB, redis *redis.Client) *HealthHandler {
	return &HealthHandler{db: db, redis: redis}
}

func (h *HealthHandler) Health(c *gin.Context) {
	response.Success(c, gin.H{"status": "ok"})
}

func (h *HealthHandler) Startup(c *gin.Context) {
	response.Success(c, gin.H{"status": "started"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := h.redis.Ping(ctx); err != nil {
		response.Error(c, errcode.ErrRedisUnavailable)
		return
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		response.Error(c, errcode.ErrDBUnavailable)
		return
	}

	if err := sqlDB.PingContext(ctx); err != nil {
		response.Error(c, errcode.ErrDBUnavailable)
		return
	}

	response.Success(c, gin.H{"status": "ready"})
}
