package main

import (
	"flag"
	"fmt"
	"log"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"im/internal/config"
	"im/internal/model"
	"im/pkg/hash"
	jwtpkg "im/pkg/jwt"
	"im/pkg/logger"
	"im/pkg/response"
	"im/pkg/errcode"

	"github.com/gin-gonic/gin"
)

var configPath = flag.String("config", "config/gateway.yaml", "config file path")

func main() {
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger.Init(cfg.Log.Level, cfg.Log.Format)
	defer logger.Sync()

	db, err := gorm.Open(mysql.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	db.AutoMigrate(&model.User{})

	jwtService := jwtpkg.New(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// 一致性哈希分配节点
	ring := hash.New(150)
	for _, node := range cfg.Nodes {
		ring.Add(node.Addr)
	}

	r := gin.Default()
	r.GET("/api/v1/ping", func(c *gin.Context) {
		response.Success(c, "pong")
	})

	r.POST("/api/v1/register", func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required,min=4"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, errcode.ErrBadRequest)
			return
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		user := &model.User{Username: req.Username, Password: string(hash)}
		if err := db.Create(user).Error; err != nil {
			response.Error(c, errcode.ErrUserExists)
			return
		}
		response.Success(c, gin.H{"id": user.ID, "username": user.Username})
	})

	r.POST("/api/v1/login", func(c *gin.Context) {
		var req struct {
			Username string `json:"username" binding:"required"`
			Password string `json:"password" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.Error(c, errcode.ErrBadRequest)
			return
		}
		var user model.User
		if db.Where("username = ?", req.Username).First(&user).Error != nil ||
			bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
			response.Error(c, errcode.ErrInvalidLogin)
			return
		}
		token, err := jwtService.GenerateToken(user.ID, user.Username)
		if err != nil {
			response.Error(c, errcode.ErrInternal)
			return
		}
		// 一致性哈希分配节点
		node := ring.Get(fmt.Sprintf("%d", user.ID))
		response.Success(c, gin.H{
			"token": token,
			"node":  fmt.Sprintf("ws://%s/ws", node),
		})
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Log.Info("gateway starting", zap.String("addr", addr))
	log.Fatal(r.Run(addr))
}
