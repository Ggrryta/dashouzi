package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"im/internal/config"
	"im/internal/ws"
	jwtpkg "im/pkg/jwt"
	"im/pkg/logger"
)

var configPath = flag.String("config", "config/node.yaml", "config file path")

var upgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

func main() {
	flag.Parse()
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	logger.Init(cfg.Log.Level, cfg.Log.Format)
	defer logger.Sync()

	// Redis
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr()})
	logger.Log.Info("redis connected")

	jwtService := jwtpkg.New(cfg.JWT.Secret, 24)
	mgr := ws.NewConnManager()
	hub := ws.NewHubWithPubSub(mgr, &ws.MemoryMessageRepo{}, nil)
	pubsub := ws.NewPubSubManager(rdb, cfg.NodeID, hub)
	hub.SetPubSub(pubsub)

	// 注册节点 + 订阅跨节点消息
	nodeAddr := fmt.Sprintf("node-%s:%d", cfg.NodeID, cfg.Server.Port)
	// 简化：用配置的 host:port
	if cfg.Server.Host == "0.0.0.0" {
		nodeAddr = fmt.Sprintf("%s:%d", cfg.NodeID, cfg.Server.Port)
	} else {
		nodeAddr = fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	}
	pubsub.RegisterNode(context.Background(), nodeAddr)
	pubsub.Subscribe(context.Background())

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		claims, err := jwtService.ParseToken(token)
		if err != nil {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Log.Error("ws upgrade failed", zap.Error(err))
			return
		}

		client := &ws.Client{
			UserID:   claims.UserID,
			Conn:     conn,
			Send:     make(chan []byte, 64),
			LastPing: time.Now(),
		}

		mgr.Add(client)
		pubsub.SetOnline(context.Background(), claims.UserID)

		logger.Log.Info("ws connected",
			zap.Uint("user_id", claims.UserID),
			zap.String("node", cfg.NodeID),
		)

		go client.WritePump()
		go client.ReadPump(mgr, hub)
	})

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	logger.Log.Info("im node starting", zap.String("addr", addr), zap.String("node_id", cfg.NodeID))
	log.Fatal(http.ListenAndServe(addr, nil))
}
