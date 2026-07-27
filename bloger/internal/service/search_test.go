package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"bloger/internal/model"
	"bloger/internal/repository"
)

type mockSearchRepo struct {
	articles []*model.Article
}

func (m *mockSearchRepo) FullTextSearch(_ context.Context, keyword string, page, size int) ([]*model.Article, int64, error) {
	var result []*model.Article
	for _, a := range m.articles {
		// 简单模拟：检查标题或内容包含关键词
		if len(keyword) > 0 && (containsStr(a.Title, keyword) || containsStr(a.Content, keyword)) {
			result = append(result, a)
		}
	}
	total := int64(len(result))
	start := (page - 1) * size
	if start >= len(result) {
		return nil, total, nil
	}
	end := start + size
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], total, nil
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type mockStatsRepo struct {
	articles []*model.Article
}

func (m *mockStatsRepo) Trending(_ context.Context, limit int) ([]*model.Article, error) {
	// 按 ViewCount 降序
	sorted := make([]*model.Article, len(m.articles))
	copy(sorted, m.articles)
	// bubble sort for simplicity
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].ViewCount > sorted[i].ViewCount {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if limit > len(sorted) {
		limit = len(sorted)
	}
	return sorted[:limit], nil
}

func (m *mockStatsRepo) UserRanking(_ context.Context, limit int) ([]repository.UserRank, error) {
	return nil, nil
}

// ====== Search Tests ======

func TestSearch_Success(t *testing.T) {
	repo := &mockSearchRepo{
		articles: []*model.Article{
			{Title: "Go语言入门", Content: "Go是一门好语言", Status: "published"},
			{Title: "Python教程", Content: "Python简单易学", Status: "published"},
		},
	}
	svc := NewSearchService(repo)

	results, total, err := svc.Search(context.Background(), SearchParams{
		Keyword: "Go",
		Page:    1,
		Size:    10,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, results, 1)
	assert.Equal(t, "Go语言入门", results[0].Title)
}

func TestSearch_EmptyKeyword(t *testing.T) {
	svc := NewSearchService(&mockSearchRepo{})

	_, _, err := svc.Search(context.Background(), SearchParams{Keyword: ""})
	assert.Error(t, err)
}

func TestSearch_Pagination(t *testing.T) {
	repo := &mockSearchRepo{}
	for i := 0; i < 5; i++ {
		repo.articles = append(repo.articles, &model.Article{Title: "Go test", Status: "published"})
	}
	svc := NewSearchService(repo)

	results, total, err := svc.Search(context.Background(), SearchParams{Keyword: "Go", Page: 1, Size: 2})
	assert.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, results, 2)
}

func TestSearch_LongKeywordTruncated(t *testing.T) {
	svc := NewSearchService(&mockSearchRepo{})

	longKeyword := ""
	for i := 0; i < 250; i++ {
		longKeyword += "a"
	}
	_, _, err := svc.Search(context.Background(), SearchParams{Keyword: longKeyword})
	assert.NoError(t, err) // 截断后不报错
}

// ====== Stats Tests ======

func TestTrending_Success(t *testing.T) {
	repo := &mockStatsRepo{
		articles: []*model.Article{
			{Title: "A", ViewCount: 10, Status: "published"},
			{Title: "B", ViewCount: 50, Status: "published"},
			{Title: "C", ViewCount: 30, Status: "published"},
		},
	}
	svc := NewStatsService(repo)

	results, err := svc.Trending(context.Background(), 2)
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, int64(50), results[0].ViewCount)
	assert.Equal(t, int64(30), results[1].ViewCount)
}

func TestTrending_DefaultLimit(t *testing.T) {
	repo := &mockStatsRepo{
		articles: []*model.Article{
			{Title: "A", ViewCount: 10, Status: "published"},
		},
	}
	svc := NewStatsService(repo)

	results, err := svc.Trending(context.Background(), 0) // 0 → default 10
	assert.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestUserRanking_DefaultLimit(t *testing.T) {
	svc := NewStatsService(&mockStatsRepo{})

	ranks, err := svc.UserRanking(context.Background(), 0)
	assert.NoError(t, err)
	assert.Nil(t, ranks) // mock 返回 nil
}
