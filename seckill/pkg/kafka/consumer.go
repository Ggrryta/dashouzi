package kafka

import (
	"context"
	"errors"
)

var ErrNoMessage = errors.New("no message")

type Consumer interface {
	Read(ctx context.Context) (*Message, error)
	Commit(ctx context.Context, msg *Message) error
}

// MockConsumer for testing
type MockConsumer struct {
	queue     []*Message
	pos       int
	Committed map[int64]bool
}

func NewMockConsumer() *MockConsumer {
	return &MockConsumer{Committed: make(map[int64]bool)}
}

func (c *MockConsumer) Feed(msgs []*Message) {
	c.queue = append(c.queue, msgs...)
}

func (c *MockConsumer) Read(_ context.Context) (*Message, error) {
	if c.pos >= len(c.queue) {
		return nil, ErrNoMessage
	}
	msg := c.queue[c.pos]
	c.pos++
	return msg, nil
}

func (c *MockConsumer) Commit(_ context.Context, msg *Message) error {
	c.Committed[msg.Offset] = true
	return nil
}
