package service

import (
	"context"
	"errors"
	"strings"

	"bloger/internal/model"
	"bloger/internal/repository"
)

type SearchParams struct {
	Keyword string
	Page    int
	Size    int
}

type SearchService struct {
	repo repository.SearchRepository
}

func NewSearchService(repo repository.SearchRepository) *SearchService {
	return &SearchService{repo: repo}
}

func (s *SearchService) Search(ctx context.Context, params SearchParams) ([]*model.Article, int64, error) {
	params.Keyword = strings.TrimSpace(params.Keyword)
	if params.Keyword == "" {
		return nil, 0, errors.New("keyword is required")
	}
	if len(params.Keyword) > 200 {
		params.Keyword = params.Keyword[:200]
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.Size <= 0 || params.Size > 50 {
		params.Size = 10
	}

	return s.repo.FullTextSearch(ctx, params.Keyword, params.Page, params.Size)
}
