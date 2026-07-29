package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	"seckill/internal/model"
	pkafka "seckill/pkg/kafka"
	"seckill/pkg/logger"
)

// SeckillMessage Kafka 消息体
type SeckillMessage struct {
	UserID uint `json:"user_id"`
	ItemID uint `json:"item_id"`
}

// ItemLookup 消费时查商品以补全订单的 session 与价格。
type ItemLookup interface {
	FindByID(ctx context.Context, id uint) (*model.SeckillItem, error)
}

// ConsumerWorker 消费 Kafka 秒杀订单消息，写入 MySQL。
type ConsumerWorker struct {
	consumer pkafka.Consumer
	orderSvc *OrderService
	itemRepo ItemLookup
	stopCh   chan struct{}
}

func NewConsumerWorker(consumer pkafka.Consumer, orderSvc *OrderService, repo ItemLookup) *ConsumerWorker {
	return &ConsumerWorker{
		consumer: consumer,
		orderSvc: orderSvc,
		itemRepo: repo,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动消费循环。ctx 取消或 Stop 被调用后退出。
func (w *ConsumerWorker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			default:
			}

			msg, err := w.consumer.Read(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				// 连接抖动等错误：退避后重试，避免 CPU 空转
				select {
				case <-time.After(time.Second):
				case <-ctx.Done():
					return
				}
				continue
			}

			if !w.process(ctx, msg) {
				// 处理失败不提交 offset，消息将被重新投递
				continue
			}
			// 写库成功后才提交 offset
			_ = w.consumer.Commit(ctx, msg)
		}
	}()
}

// process 解析并落库。返回 false 表示不应提交（待重试）。
// 不可恢复的坏消息返回 true 以跳过（避免毒丸消息阻塞队列），并记录告警。
func (w *ConsumerWorker) process(ctx context.Context, msg *pkafka.Message) bool {
	var sm SeckillMessage
	if err := json.Unmarshal(msg.Value, &sm); err != nil {
		logger.Log.Warn("drop malformed message",
			zap.Int64("offset", msg.Offset),
			zap.ByteString("value", msg.Value),
			zap.Error(err))
		return true // 坏消息跳过，避免毒丸阻塞
	}

	// 查商品补全订单的 session 与价格
	item, err := w.itemRepo.FindByID(ctx, sm.ItemID)
	if err != nil || item == nil {
		logger.Log.Error("lookup item failed, will retry",
			zap.Uint("item_id", sm.ItemID), zap.Error(err))
		return false // 商品暂时查不到（如主从延迟），重试
	}

	order := &model.SeckillOrder{
		UserID:    sm.UserID,
		ItemID:    sm.ItemID,
		SessionID: item.SessionID,
		Price:     item.Price,
		Status:    "paid",
	}
	if err := w.orderSvc.Create(ctx, order); err != nil {
		logger.Log.Error("create order failed, will retry",
			zap.Uint("user_id", sm.UserID),
			zap.Uint("item_id", sm.ItemID),
			zap.Error(err))
		return false // UNIQUE 冲突之外的错误也重试
	}
	return true
}

// Stop 优雅停止。
func (w *ConsumerWorker) Stop() {
	select {
	case <-w.stopCh:
	default:
		close(w.stopCh)
	}
}
