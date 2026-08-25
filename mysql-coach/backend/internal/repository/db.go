package repository

import (
	"database/sql"
	"fmt"

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

	// 连接池配置
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}
