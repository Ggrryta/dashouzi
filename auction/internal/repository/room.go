package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"auction/internal/model"
)

// RoomRepository 房间仓储接口
type RoomRepository interface {
	Create(ctx context.Context, room *model.Room) error
	GetByID(ctx context.Context, id int64) (*model.Room, error)
	List(ctx context.Context, cursor int64, size int) ([]*model.Room, error)
	Close(ctx context.Context, id int64) error
}

// roomRepository 房间仓储实现
type roomRepository struct {
	db *gorm.DB
}

// NewRoomRepository 创建房间仓储
func NewRoomRepository(db *gorm.DB) RoomRepository {
	return &roomRepository{db: db}
}

func (r *roomRepository) Create(ctx context.Context, room *model.Room) error {
	if err := r.db.WithContext(ctx).Create(room).Error; err != nil {
		return fmt.Errorf("create room failed: %w", err)
	}
	return nil
}

func (r *roomRepository) GetByID(ctx context.Context, id int64) (*model.Room, error) {
	var room model.Room
	if err := r.db.WithContext(ctx).First(&room, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get room failed: %w", err)
	}
	return &room, nil
}

func (r *roomRepository) List(ctx context.Context, cursor int64, size int) ([]*model.Room, error) {
	var rooms []*model.Room
	db := r.db.WithContext(ctx).Model(&model.Room{}).Order("id DESC").Limit(size)
	if cursor > 0 {
		db = db.Where("id < ?", cursor)
	}
	if err := db.Find(&rooms).Error; err != nil {
		return nil, fmt.Errorf("list rooms failed: %w", err)
	}
	return rooms, nil
}

func (r *roomRepository) Close(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Model(&model.Room{}).
		Where("id = ? AND status = ?", id, model.RoomStatusOnline).
		Update("status", model.RoomStatusClosed)
	if result.Error != nil {
		return fmt.Errorf("close room failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return errors.New("room not found or already closed")
	}
	return nil
}
