package ws

import (
	"context"
	"encoding/json"
	"fmt"

	pkafka "im/pkg/kafka"
)

type OfflineHandler struct {
	hub      *Hub
	producer pkafka.Producer
	consumer pkafka.Consumer
}

func NewOfflineHandler(hub *Hub, producer pkafka.Producer) *OfflineHandler {
	return &OfflineHandler{hub: hub, producer: producer}
}

func (h *OfflineHandler) SetConsumer(c pkafka.Consumer) { h.consumer = c }

// HandleOfflineMessage 接收方不在线 → 投递到 Kafka
func (h *OfflineHandler) HandleOfflineMessage(ctx context.Context, msg Message) error {
	topic := fmt.Sprintf("im:offline:user_%d", msg.To)
	data, _ := json.Marshal(msg)
	return h.producer.Send(ctx, topic, fmt.Sprintf("%d", msg.To), data)
}

// DeliverOfflineMessages 用户上线时拉取离线消息
func (h *OfflineHandler) DeliverOfflineMessages(ctx context.Context, userID uint) {
	if h.consumer == nil {
		return
	}

	topic := fmt.Sprintf("im:offline:user_%d", userID)
	for {
		msg, err := h.consumer.Read(ctx, topic)
		if err != nil {
			return
		}

		// 推送给用户
		if client := h.hub.mgr.Get(userID); client != nil {
			select {
			case client.Send <- msg.Value:
			default:
			}
		}

		h.consumer.Commit(ctx, msg)
	}
}
