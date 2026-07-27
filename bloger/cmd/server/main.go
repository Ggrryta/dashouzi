package main

import (
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"bloger/internal/config"
	"bloger/internal/model"
	"bloger/internal/router"
	"bloger/pkg/jwt"
	"bloger/pkg/logger"
)

func main() {
	// 加载配置
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// 初始化日志
	logger.Init(cfg.Log.Level, cfg.Log.Format)
	defer logger.Sync()

	logger.Log.Info("config loaded", zap.Int("port", cfg.Server.Port))

	// 连接数据库
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}

	// 自动迁移
	if err := db.AutoMigrate(
		&model.User{},
		&model.Article{},
		&model.Tag{},
		&model.Comment{},
		&model.Like{},
		&model.Favorite{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	// 注意: tsvector 全文索引需要手动创建
	// 参见 scripts/init.sql

	logger.Log.Info("database migrated")

	// 设置 Gin 模式
	// gin.SetMode(cfg.Server.Mode)

	// 初始化 JWT
	jwtService := jwt.New(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// 启动路由
	r := router.Setup(db, jwtService)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Log.Info("server starting", zap.String("addr", addr))

	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
