package service

import (
	"context"
	"fmt"
	"time"

	"auction/internal/model"
	"auction/internal/repository"
	"auction/pkg/errcode"
)

// ItemService 商品服务接口
type ItemService interface {
	Create(ctx context.Context, req *CreateItemRequest) (*model.Item, error)
	Get(ctx context.Context, id int64) (*model.Item, error)
	ListByRoom(ctx context.Context, roomID int64, status model.ItemStatus, cursor int64, size int) ([]*model.Item, error)
	Delete(ctx context.Context, id int64, userID int64) error
	StartPendingItems(ctx context.Context, before time.Time) error
}

// CreateItemRequest 创建商品请求
type CreateItemRequest struct {
	RoomID       int64
	SellerID     int64
	Title        string
	Description  string
	ImageURL     string
	StartPrice   int64
	MinIncrement int64
	StartTime    time.Time
	EndTime      time.Time
}

// itemService 商品服务实现
type itemService struct {
	itemRepo repository.ItemRepository
	roomRepo repository.RoomRepository
}

// NewItemService 创建商品服务
func NewItemService(itemRepo repository.ItemRepository, roomRepo repository.RoomRepository) ItemService {
	return &itemService{
		itemRepo: itemRepo,
		roomRepo: roomRepo,
	}
}

func (s *itemService) Create(ctx context.Context, req *CreateItemRequest) (*model.Item, error) {
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	room, err := s.roomRepo.GetByID(ctx, req.RoomID)
	if err != nil {
		return nil, fmt.Errorf("get room failed: %w", err)
	}
	if room == nil {
		return nil, errcode.ErrNotFound.WithMessage("房间不存在")
	}
	if room.Status != model.RoomStatusOnline {
		return nil, errcode.ErrConflict.WithMessage("房间已关闭，无法创建商品")
	}

	item := &model.Item{
		RoomID:       req.RoomID,
		SellerID:     req.SellerID,
		Title:        req.Title,
		Description:  req.Description,
		ImageURL:     req.ImageURL,
		StartPrice:   req.StartPrice,
		MinIncrement: req.MinIncrement,
		Status:       model.ItemStatusPending,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
	}
	if err := s.itemRepo.Create(ctx, item); err != nil {
		return nil, fmt.Errorf("create item failed: %w", err)
	}
	return item, nil
}

func (s *itemService) validateCreateRequest(req *CreateItemRequest) error {
	if req.Title == "" {
		return errcode.ErrBadRequest.WithMessage("商品标题不能为空")
	}
	if req.StartPrice <= 0 {
		return errcode.ErrBadRequest.WithMessage("起拍价必须大于 0")
	}
	if req.MinIncrement <= 0 {
		return errcode.ErrBadRequest.WithMessage("最小加价幅度必须大于 0")
	}
	if req.StartTime.IsZero() || req.EndTime.IsZero() {
		return errcode.ErrBadRequest.WithMessage("开始时间和结束时间不能为空")
	}
	if !req.EndTime.After(req.StartTime) {
		return errcode.ErrBadRequest.WithMessage("结束时间必须晚于开始时间")
	}
	now := time.Now()
	if req.StartTime.Before(now) {
		return errcode.ErrBadRequest.WithMessage("开始时间不能早于当前时间")
	}
	return nil
}

func (s *itemService) Get(ctx context.Context, id int64) (*model.Item, error) {
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get item failed: %w", err)
	}
	if item == nil {
		return nil, errcode.ErrNotFound.WithMessage("商品不存在")
	}
	return item, nil
}

func (s *itemService) ListByRoom(ctx context.Context, roomID int64, status model.ItemStatus, cursor int64, size int) ([]*model.Item, error) {
	if size <= 0 || size > 100 {
		size = 20
	}
	return s.itemRepo.ListByRoom(ctx, roomID, status, cursor, size)
}

func (s *itemService) Delete(ctx context.Context, id int64, userID int64) error {
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get item failed: %w", err)
	}
	if item == nil {
		return errcode.ErrNotFound.WithMessage("商品不存在")
	}
	if item.SellerID != userID {
		return errcode.ErrForbidden.WithMessage("只有卖家可以删除商品")
	}
	if item.Status != model.ItemStatusPending {
		return errcode.ErrConflict.WithMessage("只有 pending 状态的商品可以删除")
	}
	if err := s.itemRepo.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("delete item failed: %w", err)
	}
	return nil
}

func (s *itemService) StartPendingItems(ctx context.Context, before time.Time) error {
	items, err := s.itemRepo.ListPendingBefore(ctx, before)
	if err != nil {
		return fmt.Errorf("list pending items failed: %w", err)
	}
	for _, item := range items {
		if err := s.itemRepo.UpdateStatus(ctx, item.ID, model.ItemStatusPending, model.ItemStatusLive); err != nil {
			// 记录日志，继续处理其他商品
			fmt.Printf("start item %d failed: %v\n", item.ID, err)
		}
	}
	return nil
}

// Ensure itemService implements ItemService
var _ ItemService = (*itemService)(nil)
