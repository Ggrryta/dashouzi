package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"bloger/internal/model"
	"bloger/internal/repository"
)

// ====== mock repo ======

type mockArticleRepo struct {
	articles map[uint]*model.Article
	nextID   uint
}

func newMockArticleRepo() *mockArticleRepo {
	return &mockArticleRepo{articles: make(map[uint]*model.Article), nextID: 1}
}

// WithTx 返回自身：mock 不需要真实事务，单元测试通过 runInTx 的 nil db 降级路径执行。
func (m *mockArticleRepo) WithTx(_ *gorm.DB) repository.ArticleRepository {
	return m
}

func (m *mockArticleRepo) Create(_ context.Context, a *model.Article) error {
	a.ID = m.nextID
	m.nextID++
	m.articles[a.ID] = a
	return nil
}

func (m *mockArticleRepo) FindByID(_ context.Context, id uint) (*model.Article, error) {
	a, ok := m.articles[id]
	if !ok {
		return nil, nil
	}
	return a, nil
}

func (m *mockArticleRepo) FindBySlug(_ context.Context, slug string) (*model.Article, error) {
	for _, a := range m.articles {
		if a.Slug == slug {
			return a, nil
		}
	}
	return nil, nil
}

func (m *mockArticleRepo) Update(_ context.Context, a *model.Article) error {
	m.articles[a.ID] = a
	return nil
}

func (m *mockArticleRepo) Delete(_ context.Context, id uint) error {
	delete(m.articles, id)
	return nil
}

func (m *mockArticleRepo) List(_ context.Context, params repository.ArticleListParams) ([]*model.Article, int64, error) {
	var result []*model.Article
	for _, a := range m.articles {
		if (params.Status == "" || a.Status == params.Status) &&
			(params.AuthorID == 0 || a.AuthorID == params.AuthorID) {
			result = append(result, a)
		}
	}
	total := int64(len(result))

	// 分页
	start := (params.Page - 1) * params.Size
	if start >= len(result) {
		return nil, total, nil
	}
	end := start + params.Size
	if end > len(result) {
		end = len(result)
	}

	return result[start:end], total, nil
}

func (m *mockArticleRepo) IncrementViewCount(_ context.Context, id uint) error {
	// 模拟真实 DB:UPDATE 不影响已查询的内存对象,由 service 层手动 ++ 返回值
	return nil
}

type mockTagRepo struct {
	tagsByName map[string]*model.Tag
	nextID     uint
}

func newMockTagRepo() *mockTagRepo {
	return &mockTagRepo{tagsByName: make(map[string]*model.Tag), nextID: 1}
}

// WithTx 返回自身：mock 不需要真实事务。
func (m *mockTagRepo) WithTx(_ *gorm.DB) repository.TagRepository {
	return m
}

func (m *mockTagRepo) FindOrCreate(_ context.Context, name string) (*model.Tag, error) {
	if t, ok := m.tagsByName[name]; ok {
		return t, nil
	}
	t := &model.Tag{ID: m.nextID, Name: name, CreatedAt: time.Now()}
	m.nextID++
	m.tagsByName[name] = t
	return t, nil
}

func (m *mockTagRepo) FindByNames(_ context.Context, names []string) ([]*model.Tag, error) {
	var result []*model.Tag
	for _, name := range names {
		if t, ok := m.tagsByName[name]; ok {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *mockTagRepo) List(_ context.Context) ([]*model.Tag, error) {
	var result []*model.Tag
	for _, t := range m.tagsByName {
		result = append(result, t)
	}
	return result, nil
}

// ====== 状态机测试 ======

func TestValidateStatusTransition_Valid(t *testing.T) {
	tests := []struct {
		from, to string
		wantErr  bool
	}{
		{"draft", "reviewing", false},
		{"draft", "published", false},
		{"reviewing", "published", false},
		{"reviewing", "draft", false},
		{"published", "archived", false},
		{"archived", "draft", false},
	}
	for _, tt := range tests {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			err := validateStatusTransition(tt.from, tt.to)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateStatusTransition_Invalid(t *testing.T) {
	tests := []struct {
		from, to string
	}{
		{"draft", "archived"},
		{"published", "draft"},
		{"published", "reviewing"},
		{"archived", "published"},
		{"reviewing", "archived"},
	}
	for _, tt := range tests {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			err := validateStatusTransition(tt.from, tt.to)
			assert.Error(t, err)
		})
	}
}

func TestValidateStatusTransition_UnknownStatus(t *testing.T) {
	assert.Error(t, validateStatusTransition("unknown", "draft"))
	assert.Error(t, validateStatusTransition("draft", "unknown"))
}

// ====== Create 测试 ======

func TestArticleCreate_Success(t *testing.T) {
	articleRepo := newMockArticleRepo()
	tagRepo := newMockTagRepo()
	svc := NewArticleService(nil, articleRepo, tagRepo)

	article, err := svc.Create(context.Background(), 1, CreateArticleInput{
		Title:    "Test Article",
		Content:  "Hello World",
		TagNames: []string{"Go", "后端"},
	})

	assert.NoError(t, err)
	assert.Equal(t, "draft", article.Status)
	assert.Len(t, article.Tags, 2)
	assert.NotEmpty(t, article.Slug)
	assert.Equal(t, uint(1), article.AuthorID)
}

func TestArticleCreate_EmptyTitle(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())

	_, err := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "", Content: "Hello",
	})
	assert.Error(t, err)
}

func TestArticleCreate_DefaultDraftStatus(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())

	a, err := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})
	assert.NoError(t, err)
	assert.Equal(t, "draft", a.Status)
}

// ====== GetByID 测试 ======

func TestArticleGetPublicByID_Success(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())

	created, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})
	created.Status = "published" // 模拟发布

	found, err := svc.GetPublicByID(context.Background(), created.ID)
	assert.NoError(t, err)
	assert.Equal(t, "Test", found.Title)
	assert.Equal(t, int64(1), found.ViewCount) // 浏览量同步+1
}

func TestArticleGetPublicByID_NotPublished_ReturnsNotFound(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())

	created, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Draft", Content: "Test",
	}) // 默认 draft

	_, err := svc.GetPublicByID(context.Background(), created.ID)
	assert.ErrorIs(t, err, ErrArticleNotFound) // S2: 未发布不可见
}

func TestArticleGetByID_NotFound(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())

	_, err := svc.GetByID(context.Background(), 999)
	assert.Error(t, err)
}

// ====== ChangeStatus 测试 ======

func TestArticleChangeStatus_ValidTransition(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})

	err := svc.ChangeStatus(context.Background(), 1, "author", a.ID, "reviewing")
	assert.NoError(t, err)

	updated, _ := svc.GetByID(context.Background(), a.ID)
	assert.Equal(t, "reviewing", updated.Status)
}

func TestArticleChangeStatus_InvalidTransition(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})

	err := svc.ChangeStatus(context.Background(), 1, "author", a.ID, "archived") // draft→archived 非法
	assert.Error(t, err)
}

func TestArticleChangeStatus_NotOwner(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})

	err := svc.ChangeStatus(context.Background(), 2, "reader", a.ID, "reviewing") // user 2 不是作者
	assert.Error(t, err)
}

func TestArticleChangeStatus_AdminCanOverride(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})

	// S1: admin 可跨用户变更状态
	err := svc.ChangeStatus(context.Background(), 999, "admin", a.ID, "reviewing")
	assert.NoError(t, err)
}

// ====== Update 测试 ======

func TestArticleUpdate_Success(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Old", Content: "Old", TagNames: []string{"Go"},
	})

	err := svc.Update(context.Background(), 1, "author", a.ID, UpdateArticleInput{
		Title:    "New",
		Content:  "New",
		TagNames: []string{"微服务"},
	})
	assert.NoError(t, err)

	updated, _ := svc.GetByID(context.Background(), a.ID)
	assert.Equal(t, "New", updated.Title)
	assert.Len(t, updated.Tags, 1)
	assert.Equal(t, "微服务", updated.Tags[0].Name)
}

func TestArticleUpdate_NotOwner(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})

	err := svc.Update(context.Background(), 2, "reader", a.ID, UpdateArticleInput{Title: "Hacked"})
	assert.Error(t, err)
}

// ====== Delete 测试 ======

func TestArticleDelete_Success(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})

	err := svc.Delete(context.Background(), 1, "author", a.ID)
	assert.NoError(t, err)

	_, err = svc.GetByID(context.Background(), a.ID)
	assert.Error(t, err)
}

func TestArticleDelete_NotOwner(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
		Title: "Test", Content: "Test",
	})

	err := svc.Delete(context.Background(), 2, "reader", a.ID)
	assert.Error(t, err)
}

// ====== List 测试 ======

func TestArticleList_Pagination(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())
	// 创建 3 篇已发布文章
	for i := 0; i < 3; i++ {
		a, _ := svc.Create(context.Background(), 1, CreateArticleInput{
			Title: "Title", Content: "Content",
		})
		a.Status = "published"
	}

	articles, total, err := svc.List(context.Background(), ListArticleParams{
		Status: "published",
		Page:   1,
		Size:   2,
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, articles, 2)
}

func TestArticleList_OnlyPublished(t *testing.T) {
	svc := NewArticleService(nil, newMockArticleRepo(), newMockTagRepo())

	a1, _ := svc.Create(context.Background(), 1, CreateArticleInput{Title: "Pub", Content: "C"})
	a1.Status = "published"
	a2, _ := svc.Create(context.Background(), 1, CreateArticleInput{Title: "Draft", Content: "C"})
	a2.Status = "draft"

	articles, _, _ := svc.List(context.Background(), ListArticleParams{
		Status: "published",
	})
	assert.Len(t, articles, 1)
}
