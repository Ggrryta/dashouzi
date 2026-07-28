package repository

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"seckill/internal/model"
)

type OrderRepository interface {
	Create(ctx context.Context, order *model.SeckillOrder) error
	FindByUserItem(ctx context.Context, userID, itemID uint) (*model.SeckillOrder, error)
}

type orderRepo struct {
	db *gorm.DB
}

func NewOrderRepo(db *gorm.DB) OrderRepository {
	return &orderRepo{db: db}
}

// Create 创建订单。UNIQUE(user_id, item_id) 兜底幂等。
func (r *orderRepo) Create(_ context.Context, order *model.SeckillOrder) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "item_id"}},
		DoNothing: true,
	}).Create(order).Error
}

func (r *orderRepo) FindByUserItem(_ context.Context, userID, itemID uint) (*model.SeckillOrder, error) {
	var order model.SeckillOrder
	err := r.db.Where("user_id = ? AND item_id = ?", userID, itemID).First(&order).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &order, nil
}
