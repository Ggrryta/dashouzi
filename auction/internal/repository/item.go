package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"auction/internal/model"
)

// ItemRepository 商品仓储接口
type ItemRepository interface {
	Create(ctx context.Context, item *model.Item) error
	GetByID(ctx context.Context, id int64) (*model.Item, error)
	ListByRoom(ctx context.Context, roomID int64, status model.ItemStatus, cursor int64, size int) ([]*model.Item, error)
	Delete(ctx context.Context, id int64, sellerID int64) error
	UpdateStatus(ctx context.Context, id int64, from, to model.ItemStatus) error
	UpdateStatusAndWinner(ctx context.Context, id int64, status model.ItemStatus, winnerID int64) error
	UpdateCurrentPrice(ctx context.Context, id int64, bidderID int64, price int64) error
	ListPendingBefore(ctx context.Context, t time.Time) ([]*model.Item, error)
	ListClosingBefore(ctx context.Context, t time.Time) ([]*model.Item, error)
}

// itemRepository 商品仓储实现
type itemRepository struct {
	db *gorm.DB
}

// NewItemRepository 创建商品仓储
func NewItemRepository(db *gorm.DB) ItemRepository {
	return &itemRepository{db: db}
}

func (r *itemRepository) Create(ctx context.Context, item *model.Item) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create item failed: %w", err)
	}
	return nil
}

func (r *itemRepository) GetByID(ctx context.Context, id int64) (*model.Item, error) {
	var item model.Item
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get item failed: %w", err)
	}
	return &item, nil
}

func (r *itemRepository) ListByRoom(ctx context.Context, roomID int64, status model.ItemStatus, cursor int64, size int) ([]*model.Item, error) {
	var items []*model.Item
	db := r.db.WithContext(ctx).Model(&model.Item{}).
		Where("room_id = ?", roomID).
		Order("id DESC").
		Limit(size)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if cursor > 0 {
		db = db.Where("id < ?", cursor)
	}
	if err := db.Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list items failed: %w", err)
	}
	return items, nil
}

func (r *itemRepository) Delete(ctx context.Context, id int64, sellerID int64) error {
	result := r.db.WithContext(ctx).Where(
		"id = ? AND seller_id = ? AND status = ?",
		id, sellerID, model.ItemStatusPending,
	).Delete(&model.Item{})
	if result.Error != nil {
		return fmt.Errorf("delete item failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("item not found or cannot be deleted")
	}
	return nil
}

func (r *itemRepository) UpdateStatus(ctx context.Context, id int64, from, to model.ItemStatus) error {
	result := r.db.WithContext(ctx).Model(&model.Item{}).
		Where("id = ? AND status = ?", id, from).
		Update("status", to)
	if result.Error != nil {
		return fmt.Errorf("update item status failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("item not found or status mismatch")
	}
	return nil
}

func (r *itemRepository) UpdateStatusAndWinner(ctx context.Context, id int64, status model.ItemStatus, winnerID int64) error {
	updates := map[string]any{
		"status": status,
	}
	if winnerID > 0 {
		updates["winner_id"] = winnerID
	}
	result := r.db.WithContext(ctx).Model(&model.Item{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update item status and winner failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("item not found")
	}
	return nil
}

func (r *itemRepository) UpdateCurrentPrice(ctx context.Context, id int64, bidderID int64, price int64) error {
	result := r.db.WithContext(ctx).Model(&model.Item{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"current_price": price,
			"bid_count":     gorm.Expr("bid_count + 1"),
		})
	if result.Error != nil {
		return fmt.Errorf("update current price failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("item not found")
	}
	return nil
}

func (r *itemRepository) ListPendingBefore(ctx context.Context, t time.Time) ([]*model.Item, error) {
	var items []*model.Item
	if err := r.db.WithContext(ctx).Model(&model.Item{}).
		Where("status = ? AND start_time <= ?", model.ItemStatusPending, t).
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list pending items failed: %w", err)
	}
	return items, nil
}

func (r *itemRepository) ListClosingBefore(ctx context.Context, t time.Time) ([]*model.Item, error) {
	var items []*model.Item
	if err := r.db.WithContext(ctx).Model(&model.Item{}).
		Where("status IN (?, ?) AND end_time <= ?", model.ItemStatusLive, model.ItemStatusClosing, t).
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list closing items failed: %w", err)
	}
	return items, nil
}
