package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
)

// RealConsumer 基于 kafka-go Reader 的真实消费者实现，手动提交 offset。
type RealConsumer struct {
	reader *kafka.Reader
	topic  string
}

// NewRealConsumer 创建真实 Kafka 消费者。brokers 不可达时不会立即失败，
// 真正的连接在首次 Read 时按需建立。
// 使用 FetchMessage + 手动 CommitMessages 模式，确保“写库成功后才提交 offset”。
func NewRealConsumer(brokers []string, topic, groupID string) *RealConsumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &RealConsumer{reader: r, topic: topic}
}

func (c *RealConsumer) Read(ctx context.Context) (*Message, error) {
	m, err := c.reader.FetchMessage(ctx)
	if err != nil {
		return nil, err
	}
	return &Message{
		Key:       string(m.Key),
		Value:     m.Value,
		Offset:    m.Offset,
		Partition: m.Partition,
	}, nil
}

// Commit 提交指定消息的 offset。仅当订单写库成功后调用。
func (c *RealConsumer) Commit(ctx context.Context, msg *Message) error {
	return c.reader.CommitMessages(ctx, kafka.Message{
		Topic:     c.topic,
		Partition: msg.Partition,
		Offset:    msg.Offset,
	})
}

func (c *RealConsumer) Close() error {
	return c.reader.Close()
}
