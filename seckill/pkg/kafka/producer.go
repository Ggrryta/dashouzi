package kafka

import "context"

type Message struct {
	Key    string
	Value  []byte
	Offset int64
}

type Producer interface {
	Send(ctx context.Context, topic, key string, value []byte) error
}

// MockProducer for testing
type MockProducer struct {
	Messages map[string][]*Message
	offset   int64
}

func NewMockProducer() *MockProducer {
	return &MockProducer{Messages: make(map[string][]*Message)}
}

func (p *MockProducer) Send(_ context.Context, topic, key string, value []byte) error {
	p.offset++
	p.Messages[topic] = append(p.Messages[topic], &Message{
		Key: key, Value: value, Offset: p.offset,
	})
	return nil
}
