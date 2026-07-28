package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"bloger/internal/model"
)

type FavoriteRepository interface {
	Exists(ctx context.Context, userID, articleID uint) (bool, error)
	Create(ctx context.Context, fav *model.Favorite) error
	Delete(ctx context.Context, userID, articleID uint) error
	List(ctx context.Context, userID uint, page, size int) ([]*model.Favorite, int64, error)
	// Toggle 原子化切换收藏状态,用事务+ON CONFLICT 消除竞态。
	Toggle(ctx context.Context, userID, articleID uint) (bool, error)
}

type favoriteRepo struct {
	db *gorm.DB
}

func NewFavoriteRepo(db *gorm.DB) FavoriteRepository {
	return &favoriteRepo{db: db}
}

func (r *favoriteRepo) Exists(_ context.Context, userID, articleID uint) (bool, error) {
	var count int64
	err := r.db.Model(&model.Favorite{}).
		Where("user_id = ? AND article_id = ?", userID, articleID).
		Count(&count).Error
	return count > 0, err
}

func (r *favoriteRepo) Create(_ context.Context, fav *model.Favorite) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "article_id"}},
		DoNothing: true,
	}).Create(fav).Error
}

func (r *favoriteRepo) Delete(_ context.Context, userID, articleID uint) error {
	return r.db.Where("user_id = ? AND article_id = ?", userID, articleID).
		Delete(&model.Favorite{}).Error
}

func (r *favoriteRepo) List(_ context.Context, userID uint, page, size int) ([]*model.Favorite, int64, error) {
	var total int64
	query := r.db.Model(&model.Favorite{}).Where("user_id = ?", userID)
	query.Count(&total)

	var favs []*model.Favorite
	offset := (page - 1) * size
	err := query.Preload("Article").Offset(offset).Limit(size).Order("created_at DESC").Find(&favs).Error
	return favs, total, err
}

func (r *favoriteRepo) Toggle(_ context.Context, userID, articleID uint) (bool, error) {
	var faved bool
	err := r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "article_id"}},
			DoNothing: true,
		}).Create(&model.Favorite{
			UserID:    userID,
			ArticleID: articleID,
		})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected > 0 {
			faved = true
			return nil
		}
		if err := tx.Where("user_id = ? AND article_id = ?", userID, articleID).
			Delete(&model.Favorite{}).Error; err != nil {
			return err
		}
		faved = false
		return nil
	})
	return faved, err
}
