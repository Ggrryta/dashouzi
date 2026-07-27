package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bloger/internal/model"
)

type LikeRepository interface {
	Exists(ctx context.Context, userID uint, targetType string, targetID uint) (bool, error)
	Create(ctx context.Context, like *model.Like) error
	Delete(ctx context.Context, userID uint, targetType string, targetID uint) error
}

type likeRepo struct {
	db *gorm.DB
}

func NewLikeRepo(db *gorm.DB) LikeRepository {
	return &likeRepo{db: db}
}

func (r *likeRepo) Exists(_ context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.Like{}).
		Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Count(&count).Error
	return count > 0, err
}

func (r *likeRepo) Create(_ context.Context, like *model.Like) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "target_type"}, {Name: "target_id"}},
		DoNothing: true,
	}).Create(like).Error
}

func (r *likeRepo) Delete(_ context.Context, userID uint, targetType string, targetID uint) error {
	return r.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Delete(&model.Like{}).Error
}
