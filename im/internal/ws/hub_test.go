package ws

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockMessageRepo struct {
	mu       sync.Mutex
	messages map[string]*Message
	statuses map[string]string
}

func newMockMessageRepo() *mockMessageRepo {
	return &mockMessageRepo{
		messages: make(map[string]*Message),
		statuses: make(map[string]string),
	}
}

func (m *mockMessageRepo) Save(ctx context.Context, msg *Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages[msg.MsgID] = msg
	m.statuses[msg.MsgID] = "pending"
	return nil
}

func (m *mockMessageRepo) UpdateStatus(ctx context.Context, msgID, status string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses[msgID] = status
	return nil
}

func (m *mockMessageRepo) FindByMsgID(ctx context.Context, msgID string) (*Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.messages[msgID], nil
}

func TestHub_RouteToLocalOnline(t *testing.T) {
	mgr := NewConnManager()
	receiver := &Client{UserID: 2, Send: make(chan []byte, 5)}
	mgr.Add(receiver)

	hub := NewHub(mgr, newMockMessageRepo())

	msg := Message{Type: "msg", From: 1, To: 2, Content: "hi", MsgID: "uuid-1"}
	err := hub.HandleMessage(context.Background(), msg)

	assert.NoError(t, err)
	// 接收方应收到消息
	select {
	case data := <-receiver.Send:
		assert.Contains(t, string(data), "hi")
	default:
		t.Fatal("receiver should receive message")
	}
}

func TestHub_MessageStoredBeforeSend(t *testing.T) {
	mgr := NewConnManager()
	mgr.Add(&Client{UserID: 2, Send: make(chan []byte, 5)})

	repo := newMockMessageRepo()
	hub := NewHub(mgr, repo)

	msg := Message{Type: "msg", From: 1, To: 2, Content: "hi", MsgID: "uuid-1"}
	hub.HandleMessage(context.Background(), msg)

	// 验证已存储
	saved, err := repo.FindByMsgID(context.Background(), "uuid-1")
	assert.NoError(t, err)
	assert.Equal(t, "hi", saved.Content)
	assert.Equal(t, "pending", repo.statuses["uuid-1"])
}

func TestHub_ACK_UpdatesStatus(t *testing.T) {
	repo := newMockMessageRepo()
	hub := NewHub(NewConnManager(), repo)

	// 先存一条消息
	repo.Save(context.Background(), &Message{MsgID: "uuid-1"})

	// 发 ACK
	ack := Message{Type: "ack", MsgID: "uuid-1", From: 2}
	err := hub.HandleMessage(context.Background(), ack)
	assert.NoError(t, err)

	assert.Equal(t, "delivered", repo.statuses["uuid-1"])
}

func TestHub_ReceiverOffline(t *testing.T) {
	mgr := NewConnManager() // 空的 ConnManager，B 不在线
	repo := newMockMessageRepo()
	hub := NewHub(mgr, repo)

	msg := Message{Type: "msg", From: 1, To: 999, Content: "hi", MsgID: "uuid-1"}
	err := hub.HandleMessage(context.Background(), msg)

	assert.NoError(t, err)
	assert.Equal(t, "pending", repo.statuses["uuid-1"]) // 保持 pending 等离线推送
}

func TestHub_ACK_SendsToOriginalSender(t *testing.T) {
	mgr := NewConnManager()
	sender := &Client{UserID: 1, Send: make(chan []byte, 5)}
	mgr.Add(sender)

	repo := newMockMessageRepo()
	hub := NewHub(mgr, repo)

	// 先存消息
	repo.Save(context.Background(), &Message{MsgID: "uuid-1", From: 1, To: 2})

	// B 发 ACK → 应通知 A
	ack := Message{Type: "ack", MsgID: "uuid-1", From: 2}
	hub.HandleMessage(context.Background(), ack)

	select {
	case data := <-sender.Send:
		assert.Contains(t, string(data), "ack")
		assert.Contains(t, string(data), "uuid-1")
	default:
		t.Fatal("sender should receive ACK notification")
	}
}
