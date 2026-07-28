package ws

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage_MarshalUnmarshal(t *testing.T) {
	msg := Message{
		Type:    "msg",
		From:    1,
		To:      2,
		Content: "hello",
		MsgID:   "uuid-1",
	}
	data, err := json.Marshal(msg)
	assert.NoError(t, err)

	var parsed Message
	assert.NoError(t, json.Unmarshal(data, &parsed))
	assert.Equal(t, "uuid-1", parsed.MsgID)
	assert.Equal(t, "hello", parsed.Content)
	assert.Equal(t, uint(1), parsed.From)
	assert.Equal(t, uint(2), parsed.To)
}

func TestMessage_InvalidJSON(t *testing.T) {
	var msg Message
	err := json.Unmarshal([]byte("bad json"), &msg)
	assert.Error(t, err)
}

func TestMessage_ACK(t *testing.T) {
	ack := Message{Type: "ack", MsgID: "uuid-1"}
	data, _ := json.Marshal(ack)
	var parsed Message
	json.Unmarshal(data, &parsed)
	assert.Equal(t, "ack", parsed.Type)
	assert.Equal(t, "uuid-1", parsed.MsgID)
}
