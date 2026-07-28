package ws

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"im/pkg/logger"
)

type Client struct {
	UserID   uint
	Conn     *websocket.Conn
	Send     chan []byte
	LastPing time.Time
}

// WritePump 从 Send channel 取消息写入 WebSocket
func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ReadPump 从 WebSocket 读消息，通过 Hub 路由
func (c *Client) ReadPump(mgr *ConnManager, hub *Hub) {
	defer func() {
		mgr.Remove(c.UserID)
		c.Conn.Close()
		close(c.Send)
		logger.Log.Info("ws disconnected", zap.Uint("user_id", c.UserID))
	}()

	c.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.LastPing = time.Now()
		c.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			return
		}
		c.LastPing = time.Now()

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		msg.From = c.UserID
		hub.HandleMessage(context.Background(), msg)
	}
}
