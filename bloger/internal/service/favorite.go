package service

import (
	"context"

	"bloger/internal/model"
	"bloger/internal/repository"
)

type FavoriteService struct {
	repo repository.FavoriteRepository
}

func NewFavoriteService(repo repository.FavoriteRepository) *FavoriteService {
	return &FavoriteService{repo: repo}
}

// Toggle 收藏/取消收藏。返回操作后的最终状态。
func (s *FavoriteService) Toggle(ctx context.Context, userID, articleID uint) (bool, error) {
	exists, err := s.repo.Exists(ctx, userID, articleID)
	if err != nil {
		return false, err
	}

	if exists {
		return false, s.repo.Delete(ctx, userID, articleID)
	}

	return true, s.repo.Create(ctx, &model.Favorite{
		UserID:    userID,
		ArticleID: articleID,
	})
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
