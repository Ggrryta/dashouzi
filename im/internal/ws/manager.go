package ws

import "sync"

type ConnManager struct {
	mu    sync.RWMutex
	conns map[uint]*Client
}

func NewConnManager() *ConnManager {
	return &ConnManager{conns: make(map[uint]*Client)}
}

func (m *ConnManager) Add(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 踢掉旧连接
	if old, ok := m.conns[c.UserID]; ok {
		close(old.Send)
	}
	m.conns[c.UserID] = c
}

func (m *ConnManager) Get(userID uint) *Client {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conns[userID]
}

func (m *ConnManager) Remove(userID uint) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.conns, userID)
}
