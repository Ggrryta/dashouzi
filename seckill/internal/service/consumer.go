package service

import (
	"context"
	"encoding/json"
	"log"

	"seckill/internal/model"
	pkafka "seckill/pkg/kafka"
)

// SeckillMessage Kafka 消息体
type SeckillMessage struct {
	UserID uint `json:"user_id"`
	ItemID uint `json:"item_id"`
}

// ConsumerWorker 消费 Kafka 秒杀订单消息，写入 MySQL
type ConsumerWorker struct {
	consumer   pkafka.Consumer
	orderSvc   *OrderService
	itemRepo   OrderItemLookup
	stopCh     chan struct{}
}

type OrderItemLookup interface {
	GetItem(ctx context.Context, itemID uint) (item *model.SeckillItem, err error)
}

func NewConsumerWorker(consumer pkafka.Consumer, orderSvc *OrderService, repo OrderItemLookup) *ConsumerWorker {
	return &ConsumerWorker{
		consumer: consumer,
		orderSvc: orderSvc,
		itemRepo: repo,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动消费循环
func (w *ConsumerWorker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-w.stopCh:
				return
			default:
			}

			msg, err := w.consumer.Read(ctx)
			if err != nil {
				continue
			}

			var sm SeckillMessage
			if json.Unmarshal(msg.Value, &sm) != nil {
				continue
			}

			// 写入 MySQL（UNIQUE 约束兜底幂等）
			order := &model.SeckillOrder{
				UserID: sm.UserID,
				ItemID: sm.ItemID,
				Status: "paid",
			}
			if err := w.orderSvc.Create(ctx, order); err != nil {
				log.Printf("create order failed: %v", err)
				continue
			}

			// 写成功后才 commit offset
			w.consumer.Commit(ctx, msg)
		}
	}()
}

// Stop 优雅停止
func (w *ConsumerWorker) Stop() {
	close(w.stopCh)
}
