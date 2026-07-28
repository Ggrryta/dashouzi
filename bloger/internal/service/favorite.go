package service

import (
	"context"

	"bloger/internal/model"
	"bloger/internal/repository"
)

type FavoriteService struct {
	repo        repository.FavoriteRepository
	articleRepo repository.ArticleRepository
}

func NewFavoriteService(repo repository.FavoriteRepository, ar repository.ArticleRepository) *FavoriteService {
	return &FavoriteService{repo: repo, articleRepo: ar}
}

// Toggle 收藏/取消收藏。返回操作后的最终状态。
// S5: 先校验文章存在;S3: 调用 repo.Toggle 原子化切换。
func (s *FavoriteService) Toggle(ctx context.Context, userID, articleID uint) (bool, error) {
	a, err := s.articleRepo.FindByID(ctx, articleID)
	if err != nil || a == nil {
		return false, ErrTargetNotFound
	}
	return s.repo.Toggle(ctx, userID, articleID)
}

// List 收藏列表。
func (s *FavoriteService) List(ctx context.Context, userID uint, page, size int) ([]*model.Favorite, int64, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	return s.repo.List(ctx, userID, page, size)
}
