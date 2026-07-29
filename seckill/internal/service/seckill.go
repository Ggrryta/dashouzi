package service

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"

	"seckill/internal/repository"
)

//go:embed seckill.lua
var seckillLuaScript string

//go:embed rollback.lua
var rollbackLuaScript string

const defaultOrderTopic = "seckill.orders"

type SeckillResult int

const (
	ResultSuccess       SeckillResult = 1
	ResultSoldOut       SeckillResult = 0
	ResultAlreadyBought SeckillResult = -1
	ResultSessionClosed SeckillResult = -2
)

// SessionValidator 校验秒杀场次是否处于有效时间窗口内。
// 生产环境建议基于 Redis 缓存实现以避免每次请求都查 DB。
type SessionValidator interface {
	IsActive(ctx context.Context, itemID uint) (bool, error)
}

type Producer interface {
	Send(ctx context.Context, topic, key string, value []byte) error
}

type SeckillService struct {
	redis            repository.RedisCmdClient
	producer         Producer
	sessionValidator SessionValidator
	orderTopic       string
}

func NewSeckillService(redis repository.RedisCmdClient, producer Producer) *SeckillService {
	return &SeckillService{redis: redis, producer: producer, orderTopic: defaultOrderTopic}
}

// NewSeckillServiceWithValidation 构造带会话校验和可配置 topic 的秒杀服务。
func NewSeckillServiceWithValidation(redis repository.RedisCmdClient, producer Producer, validator SessionValidator, orderTopic string) *SeckillService {
	s := NewSeckillService(redis, producer)
	s.sessionValidator = validator
	if orderTopic != "" {
		s.orderTopic = orderTopic
	}
	return s
}

// Execute 执行秒杀。返回值 1=成功 0=库存不足 -1=已购买 -2=场次未开放。
// 成功时异步投递 Kafka 消息；投递失败则回滚 Redis，避免“扣了库存却无订单”。
func (s *SeckillService) Execute(ctx context.Context, itemID, userID uint) (SeckillResult, error) {
	if s.sessionValidator != nil {
		active, err := s.sessionValidator.IsActive(ctx, itemID)
		if err != nil {
			return ResultSoldOut, err
		}
		if !active {
			return ResultSessionClosed, nil
		}
	}

	stockKey := fmt.Sprintf("seckill:stock:%d", itemID)
	boughtKey := fmt.Sprintf("seckill:bought:%d", itemID)
	userArg := fmt.Sprintf("%d", userID)

	result, err := s.redis.Eval(ctx, seckillLuaScript,
		[]string{stockKey, boughtKey}, userArg)
	if err != nil {
		return ResultSoldOut, err
	}
	if result != 1 {
		return SeckillResult(result), nil
	}

	if s.producer == nil {
		return ResultSuccess, nil
	}

	msgBody := fmt.Sprintf(`{"user_id":%d,"item_id":%d}`, userID, itemID)
	if err := s.producer.Send(ctx, s.orderTopic, userArg, []byte(msgBody)); err != nil {
		// 投递失败：回滚 Redis 扣减，保证库存与订单最终一致
		if _, rerr := s.redis.Eval(ctx, rollbackLuaScript, []string{stockKey, boughtKey}, userArg); rerr != nil {
			return ResultSoldOut, fmt.Errorf("send failed: %w; rollback failed: %v", err, rerr)
		}
		return ResultSoldOut, fmt.Errorf("send failed, stock rolled back: %w", err)
	}

	return ResultSuccess, nil
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
	stock, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("invalid stock value %q: %w", v, err)
	}
	return stock, nil
}

// ItemSessionValidator 基于 itemRepo + sessionRepo 的会话校验实现。
// 注意：每次请求都会查两次 DB，高 QPS 场景应替换为 Redis 缓存版本。
type ItemSessionValidator struct {
	items    repository.ItemRepository
	sessions repository.SessionRepository
}

func NewItemSessionValidator(items repository.ItemRepository, sessions repository.SessionRepository) *ItemSessionValidator {
	return &ItemSessionValidator{items: items, sessions: sessions}
}

func (v *ItemSessionValidator) IsActive(ctx context.Context, itemID uint) (bool, error) {
	item, err := v.items.FindByID(ctx, itemID)
	if err != nil {
		return false, err
	}
	if item == nil {
		return false, nil
	}
	session, err := v.sessions.FindByID(ctx, item.SessionID)
	if err != nil {
		return false, err
	}
	if session == nil {
		return false, nil
	}
	now := nowFunc()
	return (session.StartTime.Before(now) || session.StartTime.Equal(now)) && session.EndTime.After(now), nil
}
