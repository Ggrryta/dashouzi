package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode"

	"bloger/internal/model"
	"bloger/internal/repository"
)

var (
	ErrArticleNotFound = errors.New("article not found")
	ErrNotOwner        = errors.New("not the article owner")
	ErrNotPublished    = errors.New("article not published")
)

// isAdmin 判断角色是否为管理员(可跨用户操作)。
func isAdmin(role string) bool {
	return role == "admin"
}

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

// GetByID 通过 ID 查文章(内部使用,不限状态,不计浏览量)。
func (s *ArticleService) GetByID(ctx context.Context, id uint) (*model.Article, error) {
	article, err := s.articleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrArticleNotFound
	}
	if article == nil {
		return nil, ErrArticleNotFound
	}
	return article, nil
}

// GetPublicByID 公开文章详情:仅返回 published 状态,并同步累加浏览量。
// 修复 S2(未发布内容泄露)与 M4(浏览量异步丢失)。
func (s *ArticleService) GetPublicByID(ctx context.Context, id uint) (*model.Article, error) {
	article, err := s.articleRepo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrArticleNotFound
	}
	if article == nil || article.Status != "published" {
		return nil, ErrArticleNotFound
	}

	// 同步累加浏览量:失败不阻断主流程,仅记录。
	if err := s.articleRepo.IncrementViewCount(ctx, id); err == nil {
		article.ViewCount++
	}
	return article, nil
}

// ChangeStatus 变更文章状态。校验状态机 + 所有权(admin 可跨用户)。
func (s *ArticleService) ChangeStatus(ctx context.Context, userID uint, role string, articleID uint, newStatus string) error {
	article, err := s.articleRepo.FindByID(ctx, articleID)
	if err != nil || article == nil {
		return ErrArticleNotFound
	}

	if !isAdmin(role) && article.AuthorID != userID {
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

// Update 更新文章内容。校验所有权(admin 可跨用户)。
func (s *ArticleService) Update(ctx context.Context, userID uint, role string, articleID uint, input UpdateArticleInput) error {
	article, err := s.articleRepo.FindByID(ctx, articleID)
	if err != nil || article == nil {
		return ErrArticleNotFound
	}

	if !isAdmin(role) && article.AuthorID != userID {
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

// Delete 删除文章。校验所有权(admin 可跨用户)。
func (s *ArticleService) Delete(ctx context.Context, userID uint, role string, articleID uint) error {
	article, err := s.articleRepo.FindByID(ctx, articleID)
	if err != nil || article == nil {
		return ErrArticleNotFound
	}

	if !isAdmin(role) && article.AuthorID != userID {
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
	slug := strings.ToLower(strings.TrimSpace(title))
	slug = strings.ReplaceAll(slug, " ", "-")
	// 保留字母/数字(含中文等 Unicode 字母)与 '-',过滤标点
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, slug)
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "article"
	}
	// 用纳秒时间戳 + 随机数防碰撞
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return fmt.Sprintf("%s-%d%d", slug, time.Now().UnixNano(), r.Intn(1000))
}
