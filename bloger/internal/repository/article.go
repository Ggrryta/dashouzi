package repository

import (
	"context"

	"gorm.io/gorm"

	"bloger/internal/model"
)

type ArticleRepository interface {
	WithTx(tx *gorm.DB) ArticleRepository
	Create(ctx context.Context, article *model.Article) error
	FindByID(ctx context.Context, id uint) (*model.Article, error)
	FindBySlug(ctx context.Context, slug string) (*model.Article, error)
	Update(ctx context.Context, article *model.Article) error
	Delete(ctx context.Context, id uint) error
	List(ctx context.Context, params ArticleListParams) ([]*model.Article, int64, error)
	IncrementViewCount(ctx context.Context, id uint) error
}

type ArticleListParams struct {
	Status   string
	AuthorID uint
	TagName  string
	Page     int
	Size     int
}

type TagRepository interface {
	WithTx(tx *gorm.DB) TagRepository
	FindOrCreate(ctx context.Context, name string) (*model.Tag, error)
	FindByNames(ctx context.Context, names []string) ([]*model.Tag, error)
	List(ctx context.Context) ([]*model.Tag, error)
}

type articleRepo struct {
	db *gorm.DB
}

func NewArticleRepo(db *gorm.DB) ArticleRepository {
	return &articleRepo{db: db}
}

func (r *articleRepo) WithTx(tx *gorm.DB) ArticleRepository {
	return &articleRepo{db: tx}
}

func (r *articleRepo) Create(_ context.Context, article *model.Article) error {
	return r.db.Create(article).Error
}

func (r *articleRepo) FindByID(_ context.Context, id uint) (*model.Article, error) {
	var article model.Article
	err := r.db.Preload("Tags").Preload("Author").First(&article, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &article, nil
}

func (r *articleRepo) FindBySlug(_ context.Context, slug string) (*model.Article, error) {
	var article model.Article
	err := r.db.Preload("Tags").Preload("Author").Where("slug = ?", slug).First(&article).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &article, nil
}

func (r *articleRepo) Update(_ context.Context, article *model.Article) error {
	return r.db.Save(article).Error
}

func (r *articleRepo) Delete(_ context.Context, id uint) error {
	return r.db.Delete(&model.Article{}, id).Error
}

func (r *articleRepo) List(_ context.Context, params ArticleListParams) ([]*model.Article, int64, error) {
	var articles []*model.Article
	var total int64

	query := r.db.Model(&model.Article{}).Preload("Tags").Preload("Author")

	if params.Status != "" {
		query = query.Where("status = ?", params.Status)
	}
	if params.AuthorID != 0 {
		query = query.Where("author_id = ?", params.AuthorID)
	}
	if params.TagName != "" {
		query = query.Joins("JOIN article_tags ON article_tags.article_id = articles.id").
			Joins("JOIN tags ON tags.id = article_tags.tag_id").
			Where("tags.name = ?", params.TagName)
	}

	query.Count(&total)

	if params.Page > 0 && params.Size > 0 {
		offset := (params.Page - 1) * params.Size
		query = query.Offset(offset).Limit(params.Size)
	}

	query = query.Order("is_top DESC, created_at DESC")

	err := query.Find(&articles).Error
	return articles, total, err
}

func (r *articleRepo) IncrementViewCount(_ context.Context, id uint) error {
	return r.db.Model(&model.Article{}).Where("id = ?", id).
		UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error
}

// ====== Tag Repo ======

type tagRepo struct {
	db *gorm.DB
}

func NewTagRepo(db *gorm.DB) TagRepository {
	return &tagRepo{db: db}
}

func (r *tagRepo) WithTx(tx *gorm.DB) TagRepository {
	return &tagRepo{db: tx}
}

func (r *tagRepo) FindOrCreate(_ context.Context, name string) (*model.Tag, error) {
	var tag model.Tag
	err := r.db.Where("name = ?", name).First(&tag).Error
	if err == nil {
		return &tag, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	tag = model.Tag{Name: name}
	if err := r.db.Create(&tag).Error; err != nil {
		return nil, err
	}
	return &tag, nil
}

func (r *tagRepo) FindByNames(_ context.Context, names []string) ([]*model.Tag, error) {
	var tags []*model.Tag
	err := r.db.Where("name IN ?", names).Find(&tags).Error
	return tags, err
}

func (r *tagRepo) List(_ context.Context) ([]*model.Tag, error) {
	var tags []*model.Tag
	err := r.db.Find(&tags).Error
	return tags, err
}
