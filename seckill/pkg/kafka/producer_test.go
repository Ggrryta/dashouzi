package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProducer_SendMessage(t *testing.T) {
	p := NewMockProducer()

	err := p.Send(context.Background(), "seckill.orders", "1",
		[]byte(`{"user_id":1,"item_id":1}`))

	assert.NoError(t, err)
	assert.Len(t, p.Messages["seckill.orders"], 1)
	assert.Equal(t, []byte(`{"user_id":1,"item_id":1}`), p.Messages["seckill.orders"][0].Value)
}

func TestProducer_SendWithPartitionKey(t *testing.T) {
	p := NewMockProducer()

	p.Send(context.Background(), "orders", "42", []byte("test"))

	msg := p.Messages["orders"][0]
	assert.Equal(t, "42", msg.Key)
}
