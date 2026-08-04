package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"auction/internal/config"
	"auction/internal/handler"
	"auction/internal/router"
	"auction/pkg/db"
	"auction/pkg/logger"
	redisclient "auction/pkg/redis"
)

func main() {
	configPath := os.Getenv("AUCTION_CONFIG")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger.Init(cfg.Log.Level, cfg.Log.Format)
	defer logger.Sync()

	logger.Log.Info("config loaded",
		zap.String("app", cfg.App.Name),
		zap.String("env", cfg.App.Env),
		zap.Int("port", cfg.App.Port),
	)

	dbConn, err := initDB(cfg.MySQL)
	if err != nil {
		logger.Log.Fatal("failed to init database", zap.Error(err))
	}

	redisCfg := redisclient.Config{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
	}
	redisClient := redisclient.NewClient(redisCfg)
	if err := redisClient.Ping(context.Background()); err != nil {
		logger.Log.Fatal("failed to connect redis", zap.Error(err))
	}
	logger.Log.Info("redis connected")

	healthHandler := handler.NewHealthHandler(dbConn.DB(), redisClient)

	r := router.NewRouter(healthHandler)

	addr := fmt.Sprintf(":%d", cfg.App.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  cfg.App.ReadTimeout,
		WriteTimeout: cfg.App.WriteTimeout,
	}

	go func() {
		logger.Log.Info("server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Log.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("server shutdown error", zap.Error(err))
	}

	if err := redisClient.Close(); err != nil {
		logger.Log.Error("redis close error", zap.Error(err))
	}

	logger.Sync()
	logger.Log.Info("server stopped")
}

func initDB(cfg config.MySQLConfig) (*db.Client, error) {
	dbCfg := db.Config{
		Host:            cfg.Host,
		Port:            cfg.Port,
		User:            cfg.User,
		Password:        cfg.Password,
		Database:        cfg.Database,
		Charset:         cfg.Charset,
		MaxOpenConns:    cfg.MaxOpenConns,
		MaxIdleConns:    cfg.MaxIdleConns,
		ConnMaxLifetime: cfg.ConnMaxLifetime,
	}
	return db.New(dbCfg)
}
