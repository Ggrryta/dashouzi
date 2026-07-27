package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"bloger/internal/model"
	"bloger/internal/repository"
)

var (
	ErrArticleNotFound = errors.New("article not found")
	ErrNotOwner        = errors.New("not the article owner")
)

type CreateArticleInput struct {
	Title    string
	Content  string
	Summary  string
	CoverURL string
	TagNames []string
}

type UpdateArticleInput struct {
	Title    string
	Content  string
	Summary  string
	CoverURL string
	TagNames []string
}

type ListArticleParams struct {
	Status   string
	AuthorID uint
	TagName  string
	Page     int
	Size     int
}

// 状态机：合法流转映射
var validTransitions = map[string]map[string]bool{
	"draft":     {"reviewing": true, "published": true},
	"reviewing": {"published": true, "draft": true},
	"published": {"archived": true},
	"archived":  {"draft": true},
}

func validateStatusTransition(from, to string) error {
	if from == to {
		return nil
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown status: %s", from)
	}
	if !allowed[to] {
		return fmt.Errorf("invalid transition: %s → %s", from, to)
	}
	return nil
}

type ArticleService struct {
	articleRepo repository.ArticleRepository
	tagRepo     repository.TagRepository
}

func NewArticleService(ar repository.ArticleRepository, tr repository.TagRepository) *ArticleService {
	return &ArticleService{articleRepo: ar, tagRepo: tr}
}

// Create 创建文章，默认状态 draft。
func (s *ArticleService) Create(ctx context.Context, authorID uint, input CreateArticleInput) (*model.Article, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return nil, errors.New("title is required")
	}

	article := &model.Article{
		AuthorID: authorID,
		Title:    input.Title,
		Slug:     generateSlug(input.Title),
		Content:  input.Content,
		Summary:  input.Summary,
		CoverURL: input.CoverURL,
		Status:   "draft",
	}

	// 处理标签
	if len(input.TagNames) > 0 {
		for _, name := range input.TagNames {
			tag, err := s.tagRepo.FindOrCreate(ctx, name)
			if err != nil {
				return nil, err
			}
			article.Tags = append(article.Tags, *tag)
		}
	}

	if err := s.articleRepo.Create(ctx, article); err != nil {
		return nil, err
	}

	return article, nil
}

// GetByID 通过 ID 查文章，自动增加阅读量。
func (s *ArticleService) GetByID(ctx context.Context, id uint) (*model.Article, error) {
	article, err := s.articleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrArticleNotFound
	}
	if article == nil {
		return nil, ErrArticleNotFound
	}

	// 增加阅读量（异步，不影响响应）
	go func() {
		_ = s.articleRepo.IncrementViewCount(context.Background(), id)
	}()

	// 同步更新内存中的值以便测试
	article.ViewCount++

	return article, nil
}

// GetBySlug 通过 Slug 查文章。
func (s *ArticleService) GetBySlug(ctx context.Context, slug string) (*model.Article, error) {
	article, err := s.articleRepo.FindBySlug(ctx, slug)
	if err != nil || article == nil {
		return nil, ErrArticleNotFound
	}
	article.ViewCount++
	return article, nil
}

// ChangeStatus 变更文章状态。校验状态机 + 所有权。
func (s *ArticleService) ChangeStatus(ctx context.Context, userID, articleID uint, newStatus string) error {
	article, err := s.articleRepo.FindByID(ctx, articleID)
	if err != nil || article == nil {
		return ErrArticleNotFound
	}

	// 所有权校验（只有作者和管理员能改）
	// 简化处理：管理员逻辑后续在 handler 层判断
	if article.AuthorID != userID {
		return ErrNotOwner
	}

	if err := validateStatusTransition(article.Status, newStatus); err != nil {
		return err
	}

	article.Status = newStatus
	if newStatus == "published" {
		now := time.Now()
		article.PublishedAt = &now
	}

	return s.articleRepo.Update(ctx, article)
}

// Update 更新文章内容。校验所有权。
func (s *ArticleService) Update(ctx context.Context, userID, articleID uint, input UpdateArticleInput) error {
	article, err := s.articleRepo.FindByID(ctx, articleID)
	if err != nil || article == nil {
		return ErrArticleNotFound
	}

	if article.AuthorID != userID {
		return ErrNotOwner
	}

	if input.Title != "" {
		article.Title = strings.TrimSpace(input.Title)
	}
	if input.Content != "" {
		article.Content = input.Content
	}
	if input.Summary != "" {
		article.Summary = input.Summary
	}
	if input.CoverURL != "" {
		article.CoverURL = input.CoverURL
	}

	// 更新标签
	if input.TagNames != nil {
		article.Tags = nil
		for _, name := range input.TagNames {
			tag, err := s.tagRepo.FindOrCreate(ctx, name)
			if err != nil {
				return err
			}
			article.Tags = append(article.Tags, *tag)
		}
	}

	return s.articleRepo.Update(ctx, article)
}

// Delete 删除文章。校验所有权。
func (s *ArticleService) Delete(ctx context.Context, userID, articleID uint) error {
	article, err := s.articleRepo.FindByID(ctx, articleID)
	if err != nil || article == nil {
		return ErrArticleNotFound
	}

	if article.AuthorID != userID {
		return ErrNotOwner
	}

	return s.articleRepo.Delete(ctx, articleID)
}

// List 文章列表分页查询。
func (s *ArticleService) List(ctx context.Context, params ListArticleParams) ([]*model.Article, int64, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.Size <= 0 || params.Size > 50 {
		params.Size = 10
	}

	return s.articleRepo.List(ctx, repository.ArticleListParams{
		Status:   params.Status,
		AuthorID: params.AuthorID,
		TagName:  params.TagName,
		Page:     params.Page,
		Size:     params.Size,
	})
}

func generateSlug(title string) string {
	slug := strings.ToLower(title)
	slug = strings.ReplaceAll(slug, " ", "-")
	// 简单去除特殊字符
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, slug)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%s-%d", slug, r.Intn(10000))
}
