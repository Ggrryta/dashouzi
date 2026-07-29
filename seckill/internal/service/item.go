package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"seckill/internal/model"
	"seckill/internal/repository"
)

type CreateItemInput struct {
	SessionID   uint
	Title       string
	Price       float64
	OriginPrice float64
	TotalStock  int
	ImageURL    string
}

type ItemService struct {
	repo        repository.ItemRepository
	sessionRepo repository.SessionRepository
}

func NewItemService(repo repository.ItemRepository, sessionRepo repository.SessionRepository) *ItemService {
	return &ItemService{repo: repo, sessionRepo: sessionRepo}
}

func (s *ItemService) Create(ctx context.Context, input CreateItemInput) (*model.SeckillItem, error) {
	input.Title = strings.TrimSpace(input.Title)
	if input.Title == "" {
		return nil, errors.New("title is required")
	}
	if input.Price > input.OriginPrice {
		return nil, errors.New("price must not exceed origin price")
	}
	if input.TotalStock <= 0 {
		return nil, errors.New("stock must be positive")
	}

	item := &model.SeckillItem{
		SessionID:   input.SessionID,
		Title:       input.Title,
		Price:       input.Price,
		OriginPrice: input.OriginPrice,
		TotalStock:  input.TotalStock,
		ImageURL:    input.ImageURL,
	}

	if err := s.repo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *ItemService) GetByID(ctx context.Context, id uint) (*model.SeckillItem, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *ItemService) FindBySession(ctx context.Context, sessionID uint) ([]*model.SeckillItem, error) {
	return s.repo.FindBySession(ctx, sessionID)
}

func (s *ItemService) List(ctx context.Context) ([]*model.SeckillItem, error) {
	return s.repo.List(ctx)
}

// PreheatStock 将 MySQL 中的库存预热到 Redis
func PreheatStock(ctx context.Context, redis repository.RedisClient, item *model.SeckillItem) error {
	key := fmt.Sprintf("seckill:stock:%d", item.ID)

	// 检查是否已预热
	existing, _ := redis.Get(ctx, key)
	if existing != "" {
		return nil // 已预热，不重复
	}

	return redis.Set(ctx, key, item.TotalStock, 0) // 不设过期，对账用
}
