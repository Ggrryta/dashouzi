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
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"seckill/internal/config"
	"seckill/internal/model"
	"seckill/internal/repository"
	"seckill/internal/router"
	"seckill/internal/service"
	"seckill/pkg/logger"
	pkafka "seckill/pkg/kafka"
	redisclient "seckill/pkg/redis"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger.Init(cfg.Log.Level, cfg.Log.Format)
	defer logger.Sync()

	logger.Log.Info("config loaded", zap.Int("port", cfg.Server.Port))

	db, err := gorm.Open(mysql.Open(cfg.DB.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		logger.Log.Fatal("failed to connect database", zap.Error(err))
	}

	if err := db.AutoMigrate(
		&model.SeckillSession{},
		&model.SeckillItem{},
		&model.SeckillOrder{},
	); err != nil {
		logger.Log.Fatal("failed to migrate", zap.Error(err))
	}
	logger.Log.Info("database migrated")

	redisClient := redisclient.NewClient(cfg.Redis.Addr(), cfg.Redis.Password, cfg.Redis.DB)
	logger.Log.Info("redis connected")

	kafkaProducer, err := pkafka.NewRealProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	if err != nil {
		logger.Log.Fatal("failed to connect kafka", zap.Error(err))
	}
	logger.Log.Info("kafka producer ready")

	r := router.Setup(db, redisClient, kafkaProducer, cfg.Auth.Secret, cfg.Kafka.Topic)

	// 启动 Kafka 消费者 worker（异步下单落库）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	worker := startConsumer(ctx, cfg, db)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		logger.Log.Info("server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Log.Fatal("server failed", zap.Error(err))
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Log.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Log.Error("server shutdown error", zap.Error(err))
	}

	worker.Stop()
	cancel()
	if err := kafkaProducer.Close(); err != nil {
		logger.Log.Error("kafka producer close error", zap.Error(err))
	}
	logger.Sync()
	logger.Log.Info("server stopped")
}

// startConsumer 创建并启动 Kafka 消费者 worker，返回 worker 以便优雅停止。
func startConsumer(ctx context.Context, cfg *config.Config, db *gorm.DB) *service.ConsumerWorker {
	consumer := pkafka.NewRealConsumer(cfg.Kafka.Brokers, cfg.Kafka.Topic, cfg.Kafka.ConsumerGroup)
	orderRepo := repository.NewOrderRepo(db)
	itemRepo := repository.NewItemRepo(db)
	orderSvc := service.NewOrderService(orderRepo)
	worker := service.NewConsumerWorker(consumer, orderSvc, itemRepo)
	worker.Start(ctx)
	logger.Log.Info("kafka consumer worker started",
		zap.String("topic", cfg.Kafka.Topic),
		zap.String("group", cfg.Kafka.ConsumerGroup))
	return worker
}
