package feed

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MySQLRepo 帖子 MySQL 仓库
type MySQLRepo struct {
	db *sql.DB
}

func NewMySQLRepo(db *sql.DB) *MySQLRepo {
	return &MySQLRepo{db: db}
}

func (r *MySQLRepo) Create(ctx context.Context, p *Post) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO posts (user_id, content, is_big_v, created_at) VALUES (?, ?, ?, ?)`,
		p.UserID, p.Content, p.IsBigV, p.CreatedAt,
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	p.ID = id
	return nil
}

func (r *MySQLRepo) GetByID(ctx context.Context, id int64) (*Post, error) {
	p := &Post{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, content, is_big_v, created_at FROM posts WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.UserID, &p.Content, &p.IsBigV, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *MySQLRepo) GetByUserID(ctx context.Context, userID int64, cursor int64, limit int) ([]*Post, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, content, is_big_v, created_at 
		 FROM posts 
		 WHERE user_id = ? AND created_at < FROM_UNIXTIME(?)
		 ORDER BY created_at DESC 
		 LIMIT ?`,
		userID, cursor, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []*Post
	for rows.Next() {
		p := &Post{}
		if err := rows.Scan(&p.ID, &p.UserID, &p.Content, &p.IsBigV, &p.CreatedAt); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (r *MySQLRepo) InitSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS posts (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		user_id BIGINT NOT NULL,
		content TEXT NOT NULL,
		is_big_v TINYINT DEFAULT 0,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_user_created (user_id, created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("init posts table: %w", err)
	}
	return nil
}

// timeProvider 用于测试时注入时间
var timeProvider = time.Now
