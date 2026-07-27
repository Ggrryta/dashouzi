package service

import (
	"context"

	"bloger/internal/model"
	"bloger/internal/repository"
)

type LikeService struct {
	repo repository.LikeRepository
}

func NewLikeService(repo repository.LikeRepository) *LikeService {
	return &LikeService{repo: repo}
}

// Toggle 点赞/取消点赞。返回操作后的最终状态 true=已点赞 false=已取消。
func (s *LikeService) Toggle(ctx context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	exists, err := s.repo.Exists(ctx, userID, targetType, targetID)
	if err != nil {
		return false, err
	}

	if exists {
		return false, s.repo.Delete(ctx, userID, targetType, targetID)
	}

	return true, s.repo.Create(ctx, &model.Like{
		UserID:     userID,
		TargetType: targetType,
		TargetID:   targetID,
	})
}

// IsLiked 查询是否已点赞。
func (s *LikeService) IsLiked(ctx context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	return s.repo.Exists(ctx, userID, targetType, targetID)
}
