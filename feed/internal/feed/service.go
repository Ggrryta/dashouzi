package feed

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Service 帖子服务
type Service struct {
	repo      Repository
	outbox    *Outbox
	diffusion *Diffusion
}

// NewService 创建帖子服务
func NewService(repo Repository, outbox *Outbox) *Service {
	return &Service{repo: repo, outbox: outbox}
}

// SetDiffusion 注入扩散器（可选）
func (s *Service) SetDiffusion(d *Diffusion) {
	s.diffusion = d
}

// CreatePost 创建帖子
func (s *Service) CreatePost(ctx context.Context, userID int64, content string) (*Post, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	post := &Post{
		UserID:    userID,
		Content:   content,
		CreatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, post); err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}

	ts := float64(post.CreatedAt.Unix())

	// 写入发件箱
	if s.outbox != nil {
		if err := s.outbox.Add(ctx, userID, post.ID, ts); err != nil {
			return nil, fmt.Errorf("add to outbox: %w", err)
		}
	}

	// 异步写扩散
	if s.diffusion != nil {
		s.diffusion.SpreadAsync(userID, post.ID, ts)
	}

	return post, nil
}

// GetPost 获取帖子详情
func (s *Service) GetPost(ctx context.Context, id int64) (*Post, error) {
	return s.repo.GetByID(ctx, id)
}
