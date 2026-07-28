package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger

func Init(level, format string) {
	var cfg zap.Config
	if format == "json" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
	}
	switch level {
	case "debug": cfg.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":  cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	default:      cfg.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}
	cfg.EncoderConfig.TimeKey = "time"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	var err error
	Log, err = cfg.Build()
	if err != nil {
		os.Exit(1)
	}
}

func Sync() { _ = Log.Sync() }
