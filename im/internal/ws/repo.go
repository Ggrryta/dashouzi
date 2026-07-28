package ws

import (
	"context"
	"sync"
)

// MemoryMessageRepo 内存版消息存储，Day 5 替换为 MySQL 版
type MemoryMessageRepo struct {
	mu       sync.RWMutex
	messages map[string]*Message
	statuses map[string]string
}

func (r *MemoryMessageRepo) Save(_ context.Context, msg *Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.messages == nil {
		r.messages = make(map[string]*Message)
		r.statuses = make(map[string]string)
	}
	r.messages[msg.MsgID] = msg
	r.statuses[msg.MsgID] = "pending"
	return nil
}

func (r *MemoryMessageRepo) UpdateStatus(_ context.Context, msgID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.statuses == nil {
		r.statuses = make(map[string]string)
	}
	r.statuses[msgID] = status
	return nil
}

func (r *MemoryMessageRepo) FindByMsgID(_ context.Context, msgID string) (*Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.messages == nil {
		return nil, nil
	}
	return r.messages[msgID], nil
}
