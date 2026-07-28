package ws

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	pkafka "im/pkg/kafka"
)

func TestOfflineHandler_SendWhenReceiverOffline(t *testing.T) {
	mgr := NewConnManager()
	hub := NewHub(mgr, newMockMessageRepo())
	producer := pkafka.NewMockProducer()
	handler := NewOfflineHandler(hub, producer)

	// B 不在线
	msg := Message{Type: "msg", From: 1, To: 999, Content: "offline test", MsgID: "uuid-1"}
	err := handler.HandleOfflineMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.Len(t, producer.Messages["im:offline:user_999"], 1)
}

func TestOfflineHandler_DeliverOnLogin(t *testing.T) {
	mgr := NewConnManager()
	hub := NewHub(mgr, newMockMessageRepo())

	// Producer 预投离线消息
	producer := pkafka.NewMockProducer()
	producer.Send(context.Background(), "im:offline:user_5", "1", []byte(`{"from":1,"to":5,"content":"offline_hello","msg_id":"uuid-1"}`))

	consumer := pkafka.NewMockConsumer()
	consumer.Feed("im:offline:user_5", producer.Messages["im:offline:user_5"])

	handler := NewOfflineHandler(hub, producer)
	handler.SetConsumer(consumer)

	// B 上线
	receiver := &Client{UserID: 5, Send: make(chan []byte, 10)}
	mgr.Add(receiver)

	// 拉取离线消息
	handler.DeliverOfflineMessages(context.Background(), 5)

	select {
	case data := <-receiver.Send:
		assert.Contains(t, string(data), "offline_hello")
	default:
		t.Fatal("should receive offline message")
	}
}
