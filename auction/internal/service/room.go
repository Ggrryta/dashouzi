package service

import (
	"context"
	"fmt"

	"auction/internal/model"
	"auction/internal/repository"
	"auction/pkg/errcode"
)

// RoomService 房间服务接口
type RoomService interface {
	Create(ctx context.Context, userID int64, name, description string) (*model.Room, error)
	Get(ctx context.Context, id int64) (*model.Room, error)
	List(ctx context.Context, cursor int64, size int) ([]*model.Room, error)
	Close(ctx context.Context, id int64, userID int64) error
}

// roomService 房间服务实现
type roomService struct {
	roomRepo repository.RoomRepository
}

// NewRoomService 创建房间服务
func NewRoomService(roomRepo repository.RoomRepository) RoomService {
	return &roomService{roomRepo: roomRepo}
}

func (s *roomService) Create(ctx context.Context, userID int64, name, description string) (*model.Room, error) {
	if name == "" {
		return nil, errcode.ErrBadRequest.WithMessage("房间名称不能为空")
	}
	if len(name) > 128 {
		return nil, errcode.ErrBadRequest.WithMessage("房间名称过长")
	}

	room := &model.Room{
		Name:        name,
		Description: description,
		OwnerID:     userID,
		Status:      model.RoomStatusOnline,
	}
	if err := s.roomRepo.Create(ctx, room); err != nil {
		return nil, fmt.Errorf("create room failed: %w", err)
	}
	return room, nil
}

func (s *roomService) Get(ctx context.Context, id int64) (*model.Room, error) {
	room, err := s.roomRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get room failed: %w", err)
	}
	if room == nil {
		return nil, errcode.ErrNotFound.WithMessage("房间不存在")
	}
	return room, nil
}

func (s *roomService) List(ctx context.Context, cursor int64, size int) ([]*model.Room, error) {
	if size <= 0 || size > 100 {
		size = 20
	}
	return s.roomRepo.List(ctx, cursor, size)
}

func (s *roomService) Close(ctx context.Context, id int64, userID int64) error {
	room, err := s.roomRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get room failed: %w", err)
	}
	if room == nil {
		return errcode.ErrNotFound.WithMessage("房间不存在")
	}
	if room.OwnerID != userID {
		return errcode.ErrForbidden.WithMessage("只有房主可以关闭房间")
	}
	if room.Status == model.RoomStatusClosed {
		return errcode.ErrConflict.WithMessage("房间已关闭")
	}
	if err := s.roomRepo.Close(ctx, id); err != nil {
		return fmt.Errorf("close room failed: %w", err)
	}
	return nil
}

// Ensure roomService implements RoomService
var _ RoomService = (*roomService)(nil)
