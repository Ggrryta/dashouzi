package service

import (
	"context"
	_ "embed"
	"fmt"

	"seckill/internal/repository"
)

//go:embed seckill.lua
var seckillLuaScript string

var _ = fmt.Sprint

type SeckillResult int

const (
	ResultSuccess       SeckillResult = 1
	ResultSoldOut       SeckillResult = 0
	ResultAlreadyBought SeckillResult = -1
)

type SeckillService struct {
	redis    repository.RedisCmdClient
	producer Producer
}

type Producer interface {
	Send(ctx context.Context, topic, key string, value []byte) error
}

func NewSeckillService(redis repository.RedisCmdClient, producer Producer) *SeckillService {
	return &SeckillService{redis: redis, producer: producer}
}

// Execute 执行秒杀。返回值 1=成功 0=库存不足 -1=已购买。
// 成功时异步投递 Kafka 消息。
func (s *SeckillService) Execute(ctx context.Context, itemID, userID uint) (SeckillResult, error) {
	stockKey := fmt.Sprintf("seckill:stock:%d", itemID)
	boughtKey := fmt.Sprintf("seckill:bought:%d", itemID)

	result, err := s.redis.Eval(ctx, seckillLuaScript,
		[]string{stockKey, boughtKey},
		fmt.Sprintf("%d", userID),
	)
	if err != nil {
		return ResultSoldOut, err
	}

	if result == 1 && s.producer != nil {
		msgBody := fmt.Sprintf(`{"user_id":%d,"item_id":%d}`, userID, itemID)
		_ = s.producer.Send(ctx, "seckill.orders", fmt.Sprintf("%d", userID), []byte(msgBody))
	}

	return SeckillResult(result), nil
}

// IsBought 查询用户是否已购买。
func (s *SeckillService) IsBought(ctx context.Context, itemID, userID uint) (bool, error) {
	key := fmt.Sprintf("seckill:bought:%d", itemID)
	return s.redis.SIsMember(ctx, key, fmt.Sprintf("%d", userID))
}

// GetStock 查询当前剩余库存。
func (s *SeckillService) GetStock(ctx context.Context, itemID uint) (int, error) {
	key := fmt.Sprintf("seckill:stock:%d", itemID)
	v, err := s.redis.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	var stock int
	fmt.Sscanf(v, "%d", &stock)
	return stock, nil
}
