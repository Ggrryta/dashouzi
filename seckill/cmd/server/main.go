package main

import (
	"fmt"
	"log"
	"os"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"seckill/internal/config"
	"seckill/internal/model"
	"seckill/internal/router"
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
		log.Fatalf("failed to connect database: %v", err)
	}

	if err := db.AutoMigrate(
		&model.SeckillSession{},
		&model.SeckillItem{},
		&model.SeckillOrder{},
	); err != nil {
		log.Fatalf("failed to migrate: %v", err)
	}

	logger.Log.Info("database migrated")

	// Redis
	redisClient := redisclient.NewClient(cfg.Redis.Addr(), cfg.Redis.Password, cfg.Redis.DB)
	logger.Log.Info("redis connected")

	// Kafka Producer（带重试）
	kafkaProducer, err := pkafka.NewRealProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	if err != nil {
		log.Fatalf("failed to connect kafka: %v", err)
	}
	defer kafkaProducer.Close()
	logger.Log.Info("kafka producer ready")

	r := router.Setup(db, redisClient, kafkaProducer)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Log.Info("server starting", zap.String("addr", addr))

	if err := r.Run(addr); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
