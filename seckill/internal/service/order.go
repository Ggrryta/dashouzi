package service

import (
	"context"

	"seckill/internal/model"
	"seckill/internal/repository"
)

type OrderService struct {
	repo repository.OrderRepository
}

func NewOrderService(repo repository.OrderRepository) *OrderService {
	return &OrderService{repo: repo}
}

func (s *OrderService) Create(ctx context.Context, order *model.SeckillOrder) error {
	return s.repo.Create(ctx, order)
}
