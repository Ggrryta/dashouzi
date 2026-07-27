package service

import (
	"context"

	"bloger/internal/model"
	"bloger/internal/repository"
)

type StatsService struct {
	repo repository.StatsRepository
}

func NewStatsService(repo repository.StatsRepository) *StatsService {
	return &StatsService{repo: repo}
}

func (s *StatsService) Trending(ctx context.Context, limit int) ([]*model.Article, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.repo.Trending(ctx, limit)
}

func (s *StatsService) UserRanking(ctx context.Context, limit int) ([]repository.UserRank, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.repo.UserRanking(ctx, limit)
}
