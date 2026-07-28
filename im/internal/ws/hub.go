package ws

import (
	"context"
	"encoding/json"

	"go.uber.org/zap"

	"im/pkg/logger"
)

type MessageRepo interface {
	Save(ctx context.Context, msg *Message) error
	UpdateStatus(ctx context.Context, msgID, status string) error
	FindByMsgID(ctx context.Context, msgID string) (*Message, error)
}

type Hub struct {
	mgr    *ConnManager
	repo   MessageRepo
	pubsub *PubSubManager
}

func NewHub(mgr *ConnManager, repo MessageRepo) *Hub {
	return &Hub{mgr: mgr, repo: repo}
}

func NewHubWithPubSub(mgr *ConnManager, repo MessageRepo, pubsub *PubSubManager) *Hub {
	return &Hub{mgr: mgr, repo: repo, pubsub: pubsub}
}

func (h *Hub) SetPubSub(ps *PubSubManager) { h.pubsub = ps }

// HandleMessage 路由一条消息。msg → ack / 普通消息
func (h *Hub) HandleMessage(ctx context.Context, msg Message) error {
	switch msg.Type {
	case "ack":
		return h.handleACK(ctx, msg)
	default:
		return h.handleChat(ctx, msg)
	}
}

func (h *Hub) handleChat(ctx context.Context, msg Message) error {
	// 1. 先存 DB
	if err := h.repo.Save(ctx, &msg); err != nil {
		return err
	}

	// 2. 推送给本地接收方
	if receiver := h.mgr.Get(msg.To); receiver != nil {
		data, _ := json.Marshal(msg)
		select {
		case receiver.Send <- data:
		default:
		}
		return nil
	}

	// 3. 本地没有 → 跨节点发布
	if h.pubsub != nil {
		targetNode, err := h.pubsub.ResolveNode(ctx, msg.To)
		if err == nil && targetNode != "" {
			return h.pubsub.PublishToNode(ctx, targetNode, msg)
		}
	}

	return nil
}

func (h *Hub) handleACK(ctx context.Context, msg Message) error {
	// 更新 DB 状态
	if err := h.repo.UpdateStatus(ctx, msg.MsgID, "delivered"); err != nil {
		logger.Log.Error("ack update failed", zap.Error(err))
		return err
	}

	// 查原始消息，找到发送方（需要被通知的人）
	original, _ := h.repo.FindByMsgID(ctx, msg.MsgID)
	originalSender := msg.From // fallback
	if original != nil {
		originalSender = original.From
	}

	ackResp := map[string]string{"type": "ack", "msg_id": msg.MsgID, "status": "delivered"}
	data, _ := json.Marshal(ackResp)

	if sender := h.mgr.Get(originalSender); sender != nil {
		select {
		case sender.Send <- data:
		default:
		}
	}

	return nil
}
