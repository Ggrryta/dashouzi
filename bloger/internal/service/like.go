package service

import (
	"context"
	"errors"

	"bloger/internal/repository"
)

var ErrTargetNotFound = errors.New("target resource not found")

type LikeService struct {
	repo        repository.LikeRepository
	articleRepo repository.ArticleRepository
	commentRepo repository.CommentRepository
}

func NewLikeService(repo repository.LikeRepository, ar repository.ArticleRepository, cr repository.CommentRepository) *LikeService {
	return &LikeService{repo: repo, articleRepo: ar, commentRepo: cr}
}

// Toggle 点赞/取消点赞。返回操作后的最终状态 true=已点赞 false=已取消。
// S5: 先校验 target 存在;S3: 调用 repo.Toggle 原子化切换(事务+ON CONFLICT)。
func (s *LikeService) Toggle(ctx context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	if err := s.ensureTargetExists(ctx, targetType, targetID); err != nil {
		return false, err
	}
	return s.repo.Toggle(ctx, userID, targetType, targetID)
}

// IsLiked 查询是否已点赞。
func (s *LikeService) IsLiked(ctx context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	return s.repo.Exists(ctx, userID, targetType, targetID)
}

// ensureTargetExists 校验点赞目标存在,避免产生孤儿 like 记录。
func (s *LikeService) ensureTargetExists(ctx context.Context, targetType string, targetID uint) error {
	switch targetType {
	case "article":
		a, err := s.articleRepo.FindByID(ctx, targetID)
		if err != nil || a == nil {
			return ErrTargetNotFound
		}
	case "comment":
		c, err := s.commentRepo.FindByID(ctx, targetID)
		if err != nil || c == nil {
			return ErrTargetNotFound
		}
	default:
		return ErrTargetNotFound
	}
	return nil
}
