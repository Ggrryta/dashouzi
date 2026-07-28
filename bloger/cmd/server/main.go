package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
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
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化日志
	logger.Init(cfg.Log.Level, cfg.Log.Format)
	defer logger.Sync()

	logger.Log.Info("config loaded", zap.Int("port", cfg.Server.Port))

	// 设置 Gin 模式
	gin.SetMode(cfg.Server.Mode)

	// 连接数据库
	db, err := gorm.Open(postgres.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		logger.Log.Fatal("failed to connect database", zap.Error(err))
	}

	// 连接池配置
	sqlDB, err := db.DB()
	if err != nil {
		logger.Log.Fatal("failed to get sql.DB", zap.Error(err))
	}
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移
	if err := db.AutoMigrate(
		&model.User{},
		&model.Article{},
		&model.Tag{},
		&model.Comment{},
		&model.Like{},
		&model.Favorite{},
	); err != nil {
		logger.Log.Fatal("failed to migrate", zap.Error(err))
	}

	logger.Log.Info("database migrated")

	// 初始化 JWT
	jwtService := jwt.New(cfg.JWT.Secret, cfg.JWT.ExpireHours)

	// 启动路由
	r := router.Setup(db, jwtService, cfg.Sensitive.Words)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: r,
	}

	// 优雅关机
	go func() {
		logger.Log.Info("server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("failed to start server", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Log.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Log.Error("server forced to shutdown", zap.Error(err))
	}
	sqlDB.Close()
	logger.Log.Info("server exited")
}
