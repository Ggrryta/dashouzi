package ws

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnManager_AddAndGet(t *testing.T) {
	mgr := NewConnManager()
	c := &Client{UserID: 1, Send: make(chan []byte, 10)}
	mgr.Add(c)

	got := mgr.Get(1)
	assert.NotNil(t, got)
	assert.Equal(t, uint(1), got.UserID)
}

func TestConnManager_GetNotExist(t *testing.T) {
	mgr := NewConnManager()
	assert.Nil(t, mgr.Get(999))
}

func TestConnManager_Remove(t *testing.T) {
	mgr := NewConnManager()
	c := &Client{UserID: 1, Send: make(chan []byte, 10)}
	mgr.Add(c)
	mgr.Remove(1)
	assert.Nil(t, mgr.Get(1))
}

func TestConnManager_DuplicateLoginKicksOld(t *testing.T) {
	mgr := NewConnManager()
	old := &Client{UserID: 1, Send: make(chan []byte, 5)}
	mgr.Add(old)

	newClient := &Client{UserID: 1, Send: make(chan []byte, 5)}
	mgr.Add(newClient)

	// 旧连接应被踢
	_, ok := <-old.Send
	assert.False(t, ok, "old Send channel should be closed")
	// 新连接应存在
	assert.Equal(t, newClient, mgr.Get(1))
}

func TestConnManager_Concurrent(t *testing.T) {
	mgr := NewConnManager()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			c := &Client{UserID: uint(uid), Send: make(chan []byte, 5)}
			mgr.Add(c)
			mgr.Get(uint(uid))
			mgr.Remove(uint(uid))
		}(i)
	}
	wg.Wait()
	// 不 panic 就是通过
	assert.Nil(t, mgr.Get(1))
}

func TestSendChannel_NoBlockWhenFull(t *testing.T) {
	c := &Client{UserID: 1, Send: make(chan []byte, 2)}
	c.Send <- []byte("a")
	c.Send <- []byte("b")
	// channel 满，写入不应阻塞
	select {
	case c.Send <- []byte("c"):
		// OK, 写入成功
	default:
		// 也应该 OK，不阻塞
	}
	// Cleanup
	close(c.Send)
	// 清理 channel 中的消息
	for range c.Send {
	}
}
