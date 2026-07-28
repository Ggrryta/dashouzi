package kafka

import (
	"context"
	"errors"
	"sync"
)

var ErrNoMessage = errors.New("no message")

type Consumer interface {
	Read(ctx context.Context, topic string) (*Message, error)
	Commit(ctx context.Context, msg *Message) error
}

type MockConsumer struct {
	mu        sync.Mutex
	queues    map[string][]*Message
	positions map[string]int
	committed map[int64]bool
}

func NewMockConsumer() *MockConsumer {
	return &MockConsumer{
		queues:    make(map[string][]*Message),
		positions: make(map[string]int),
		committed: make(map[int64]bool),
	}
}

func (c *MockConsumer) Feed(topic string, msgs []*Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queues[topic] = append(c.queues[topic], msgs...)
}

func (c *MockConsumer) Read(_ context.Context, topic string) (*Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pos := c.positions[topic]
	queue := c.queues[topic]
	if pos >= len(queue) {
		return nil, ErrNoMessage
	}
	msg := queue[pos]
	c.positions[topic] = pos + 1
	return msg, nil
}

func (c *MockConsumer) Commit(_ context.Context, msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.committed[msg.Offset] = true
	return nil
}
