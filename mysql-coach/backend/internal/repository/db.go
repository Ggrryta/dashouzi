package repository

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/mysql-coach/backend/internal/config"
)

// New 创建数据库连接
func New(cfg config.DBConfig) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	// 连接池配置（参数来自 DBConfig，避免硬编码）
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetimeMin) * time.Minute)
	db.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxLifetimeMin) * time.Minute)

	return db, nil
}
