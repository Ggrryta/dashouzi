package repository

import (
	"context"

	"gorm.io/gorm"

	"bloger/internal/model"
)

type CommentRepository interface {
	Create(ctx context.Context, comment *model.Comment) error
	FindByID(ctx context.Context, id uint) (*model.Comment, error)
	FindByArticle(ctx context.Context, articleID uint) ([]*model.Comment, error)
	FindReplies(ctx context.Context, parentID uint) ([]*model.Comment, error)
	SoftDelete(ctx context.Context, id uint) error
}

type commentRepo struct {
	db *gorm.DB
}

func NewCommentRepo(db *gorm.DB) CommentRepository {
	return &commentRepo{db: db}
}

func (r *commentRepo) Create(_ context.Context, comment *model.Comment) error {
	return r.db.Create(comment).Error
}

func (r *commentRepo) FindByID(_ context.Context, id uint) (*model.Comment, error) {
	var c model.Comment
	err := r.db.Preload("User").First(&c, id).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (r *commentRepo) FindByArticle(_ context.Context, articleID uint) ([]*model.Comment, error) {
	var comments []*model.Comment
	err := r.db.
		Where("article_id = ? AND parent_id IS NULL", articleID).
		Preload("User").
		Preload("Replies", "is_deleted = false").
		Preload("Replies.User").
		Order("created_at DESC").
		Find(&comments).Error
	return comments, err
}

func (r *commentRepo) FindReplies(_ context.Context, parentID uint) ([]*model.Comment, error) {
	var replies []*model.Comment
	err := r.db.
		Where("parent_id = ? AND is_deleted = false", parentID).
		Preload("User").
		Find(&replies).Error
	return replies, err
}

func (r *commentRepo) SoftDelete(_ context.Context, id uint) error {
	return r.db.Model(&model.Comment{}).Where("id = ?", id).
		Update("is_deleted", true).Error
}
