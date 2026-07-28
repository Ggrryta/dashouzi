package feed

import (
	"context"
	"time"
)

// Post 帖子模型
type Post struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Content   string    `json:"content"`
	IsBigV    bool      `json:"is_big_v"`
	CreatedAt time.Time `json:"created_at"`
}

// Repository 帖子持久化接口
type Repository interface {
	Create(ctx context.Context, p *Post) error
	GetByID(ctx context.Context, id int64) (*Post, error)
	GetByUserID(ctx context.Context, userID int64, cursor int64, limit int) ([]*Post, error)
}
