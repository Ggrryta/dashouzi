package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConsumer_ReadAndCommit(t *testing.T) {
	c := NewMockConsumer()

	// Send some messages via mock producer
	p := NewMockProducer()
	p.Send(context.Background(), "orders", "1", []byte("msg1"))
	p.Send(context.Background(), "orders", "2", []byte("msg2"))

	// Consumer reads
	c.Feed(p.Messages["orders"])
	msg, err := c.Read(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "msg1", string(msg.Value))

	err = c.Commit(context.Background(), msg)
	assert.NoError(t, err)
	assert.True(t, c.Committed[msg.Offset])

	// Second message
	msg2, _ := c.Read(context.Background())
	assert.Equal(t, "msg2", string(msg2.Value))
}
