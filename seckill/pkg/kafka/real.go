package kafka

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
)

type RealProducer struct {
	writer *kafka.Writer
	topic  string
}

func NewRealProducer(brokers []string, topic string) (*RealProducer, error) {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
		// 自动创建 topic
		AllowAutoTopicCreation: true,
	}
	return &RealProducer{writer: writer, topic: topic}, nil
}

func (p *RealProducer) Send(_ context.Context, topic string, key string, value []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(key),
		Value: value,
	})
}

func (p *RealProducer) Close() error {
	return p.writer.Close()
}
