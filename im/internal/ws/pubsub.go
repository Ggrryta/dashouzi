package ws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"im/pkg/logger"
)

// PubSubManager 跨节点消息路由
type PubSubManager struct {
	rdb    *redis.Client
	nodeID string
	hub    *Hub
}

func NewPubSubManager(rdb *redis.Client, nodeID string, hub *Hub) *PubSubManager {
	return &PubSubManager{rdb: rdb, nodeID: nodeID, hub: hub}
}

// PublishToNode 发布消息到目标节点
func (p *PubSubManager) PublishToNode(ctx context.Context, targetNode string, msg Message) error {
	data, _ := json.Marshal(msg)
	return p.rdb.Publish(ctx, "im:node:"+targetNode, data).Err()
}

// Subscribe 订阅本节点频道，接收其他节点转发的消息
func (p *PubSubManager) Subscribe(ctx context.Context) {
	channel := "im:node:" + p.nodeID
	pubsub := p.rdb.Subscribe(ctx, channel)

	logger.Log.Info("pubsub subscribed", zap.String("channel", channel))

	go func() {
		defer pubsub.Close()
		ch := pubsub.Channel()

		for msg := range ch {
			var wsMsg Message
			if err := json.Unmarshal([]byte(msg.Payload), &wsMsg); err != nil {
				continue
			}
			// 路由到本地 Hub
			p.hub.HandleMessage(ctx, wsMsg)
		}
	}()
}

// RegisterNode 注册节点到 Redis
func (p *PubSubManager) RegisterNode(ctx context.Context, addr string) {
	p.rdb.HSet(ctx, "im:nodes", p.nodeID, addr)
}

// ResolveNode 查询用户所在节点
func (p *PubSubManager) ResolveNode(ctx context.Context, userID uint) (string, error) {
	return p.rdb.HGet(ctx, "im:online", strconvUser(userID)).Result()
}

// SetOnline 设置用户在线状态
func (p *PubSubManager) SetOnline(ctx context.Context, userID uint) {
	p.rdb.HSet(ctx, "im:online", strconvUser(userID), p.nodeID)
}

// SetOffline 清除用户在线状态
func (p *PubSubManager) SetOffline(ctx context.Context, userID uint) {
	p.rdb.HDel(ctx, "im:online", strconvUser(userID))
}

func strconvUser(id uint) string {
	return fmt.Sprintf("%d", id)
}
