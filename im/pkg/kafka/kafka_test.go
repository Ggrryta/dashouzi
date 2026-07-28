package kafka

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMockProducer_Send(t *testing.T) {
	p := NewMockProducer()
	err := p.Send(context.Background(), "im:offline:user_1", "1", []byte(`{"user_id":1,"item_id":1}`))
	assert.NoError(t, err)
	assert.Len(t, p.Messages["im:offline:user_1"], 1)
}

func TestMockConsumer_ReadAndCommit(t *testing.T) {
	p := NewMockProducer()
	p.Send(context.Background(), "im:offline:user_1", "1", []byte("msg1"))
	p.Send(context.Background(), "im:offline:user_1", "2", []byte("msg2"))

	c := NewMockConsumer()
	c.Feed("im:offline:user_1", p.Messages["im:offline:user_1"])

	msg, err := c.Read(context.Background(), "im:offline:user_1")
	assert.NoError(t, err)
	assert.Equal(t, "msg1", string(msg.Value))

	c.Commit(context.Background(), msg)
	assert.True(t, c.committed[msg.Offset])
}
