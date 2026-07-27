package repository

import (
	"context"

	"gorm.io/gorm"

	"bloger/internal/model"
)

type SearchRepository interface {
	FullTextSearch(ctx context.Context, keyword string, page, size int) ([]*model.Article, int64, error)
}

type UserRank struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	ArticleCount int64 `json:"article_count"`
	CommentCount int64 `json:"comment_count"`
}

type StatsRepository interface {
	Trending(ctx context.Context, limit int) ([]*model.Article, error)
	UserRanking(ctx context.Context, limit int) ([]UserRank, error)
}

type searchRepo struct {
	db *gorm.DB
}

func NewSearchRepo(db *gorm.DB) SearchRepository {
	return &searchRepo{db: db}
}

func (r *searchRepo) FullTextSearch(_ context.Context, keyword string, page, size int) ([]*model.Article, int64, error) {
	var articles []*model.Article
	var total int64

	likeKeyword := "%" + keyword + "%"
	query := r.db.Model(&model.Article{}).
		Where("status = ?", "published").
		Where("title ILIKE ? OR content ILIKE ?", likeKeyword, likeKeyword)

	query.Count(&total)

	offset := (page - 1) * size
	err := query.
		Order("created_at DESC").
		Offset(offset).Limit(size).
		Preload("Tags").Preload("Author").
		Find(&articles).Error

	return articles, total, err
}

type statsRepo struct {
	db *gorm.DB
}

func NewStatsRepo(db *gorm.DB) StatsRepository {
	return &statsRepo{db: db}
}

func (r *statsRepo) Trending(_ context.Context, limit int) ([]*model.Article, error) {
	var articles []*model.Article
	err := r.db.Where("status = ?", "published").
		Order("view_count DESC").
		Limit(limit).
		Preload("Tags").Preload("Author").
		Find(&articles).Error
	return articles, err
}

func (r *statsRepo) UserRanking(_ context.Context, limit int) ([]UserRank, error) {
	var ranks []UserRank
	err := r.db.Raw(`
		SELECT u.id as user_id, u.username,
			COUNT(DISTINCT a.id) as article_count,
			COUNT(DISTINCT c.id) as comment_count
		FROM users u
		LEFT JOIN articles a ON a.author_id = u.id AND a.status = 'published'
		LEFT JOIN comments c ON c.user_id = u.id AND c.is_deleted = false
		GROUP BY u.id, u.username
		ORDER BY article_count DESC, comment_count DESC
		LIMIT ?
	`, limit).Scan(&ranks).Error
	return ranks, err
}
