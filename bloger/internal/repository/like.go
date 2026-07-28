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
	// Toggle 原子化切换点赞状态:未点赞则创建(返回 true),已点赞则删除(返回 false)。
	// 用事务 + ON CONFLICT DO NOTHING 避免 check-then-act 竞态。
	Toggle(ctx context.Context, userID uint, targetType string, targetID uint) (bool, error)
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

func (r *likeRepo) Toggle(_ context.Context, userID uint, targetType string, targetID uint) (bool, error) {
	var liked bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "target_type"}, {Name: "target_id"}},
			DoNothing: true,
		}).Create(&model.Like{
			UserID:     userID,
			TargetType: targetType,
			TargetID:   targetID,
		})
		if res.Error != nil {
			return res.Error
		}
		// RowsAffected=1 表示新插入(未冲突),即新增点赞
		if res.RowsAffected > 0 {
			liked = true
			return nil
		}
		// 已存在 -> 取消点赞
		if err := tx.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
			Delete(&model.Like{}).Error; err != nil {
			return err
		}
		liked = false
		return nil
	})
	return liked, err
}
