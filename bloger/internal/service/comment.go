package service

import (
	"context"
	"errors"
	"strings"

	"bloger/internal/model"
	"bloger/internal/repository"
	"bloger/pkg/sensitive"
)

var (
	ErrCommentNotFound  = errors.New("comment not found")
	ErrSensitiveContent = errors.New("content contains sensitive words")
	ErrNotCommentOwner  = errors.New("not the comment owner")
	ErrInvalidParent    = errors.New("invalid parent comment")
)

type CreateCommentInput struct {
	Content  string
	ParentID *uint
}

type CommentService struct {
	repo   repository.CommentRepository
	filter sensitive.Filter
}

func NewCommentService(repo repository.CommentRepository, filter sensitive.Filter) *CommentService {
	return &CommentService{repo: repo, filter: filter}
}

func (s *CommentService) Create(ctx context.Context, userID, articleID uint, input CreateCommentInput) (*model.Comment, error) {
	input.Content = strings.TrimSpace(input.Content)
	if input.Content == "" {
		return nil, errors.New("content is required")
	}

	// 敏感词过滤
	if matched := s.filter.Match(input.Content); len(matched) > 0 {
		return nil, ErrSensitiveContent
	}

	// S4: parent_id 合法性校验 —— 必须存在且属于同一篇文章
	if input.ParentID != nil {
		parent, err := s.repo.FindByID(ctx, *input.ParentID)
		if err != nil || parent == nil {
			return nil, ErrInvalidParent
		}
		if parent.ArticleID != articleID {
			return nil, ErrInvalidParent // 跨文章回复,拒绝
		}
	}

	comment := &model.Comment{
		ArticleID: articleID,
		UserID:    userID,
		ParentID:  input.ParentID,
		Content:   input.Content,
	}

	if err := s.repo.Create(ctx, comment); err != nil {
		return nil, err
	}

	return comment, nil
}

func (s *CommentService) GetByArticle(ctx context.Context, articleID uint) ([]*model.Comment, error) {
	return s.repo.FindByArticle(ctx, articleID)
}

func (s *CommentService) Delete(ctx context.Context, userID uint, role string, commentID uint) error {
	c, err := s.repo.FindByID(ctx, commentID)
	if err != nil || c == nil {
		return ErrCommentNotFound
	}

	// S6: admin 可删除任意评论
	if !isAdmin(role) && c.UserID != userID {
		return ErrNotCommentOwner
	}

	return s.repo.SoftDelete(ctx, commentID)
}
