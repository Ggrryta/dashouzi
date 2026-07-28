package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"gateway/internal/config"
	"gateway/internal/plugin"
	"gateway/internal/proxy"
	"gateway/internal/router"
	"gateway/pkg/jwt"
	"gateway/pkg/logger"
)

var configPath = flag.String("config", "config/gateway.yaml", "config file path")

func main() {
	flag.Parse()

	logger.Init("debug", "json")
	defer logger.Sync()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 构建路由表
	routes := make([]router.Route, len(cfg.Routes))
	for i, rc := range cfg.Routes {
		routes[i] = router.Route{
			Path:     rc.Path,
			Upstream: rc.Upstream,
		}
	}
	table := router.NewTable(routes)

	// 代理处理器（最终 handler）
	proxyHandler := proxy.NewHandler(table)

	// 插件注册
	reg := plugin.NewRegistry()

	// JWT 认证插件（白名单：health + admin 免认证）
	jwtService := jwt.New("gateway-secret", 24)
	authPlugin := NewAuthPlugin(jwtService, []string{"/health", "/admin"})
	reg.Register(authPlugin)

	// 构建中间件链
	finalHandler := reg.BuildChain(proxyHandler)

	// Admin API
	http.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"routes":` + fmt.Sprintf("%d", len(routes)) + `}`))
	})
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// 所有请求走插件链 → 代理
	http.HandleFunc("/", finalHandler.ServeHTTP)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("gateway starting on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// AuthPlugin 认证插件
type AuthPlugin struct {
	jwt      *jwt.JWT
	whitelist []string
}

func NewAuthPlugin(j *jwt.JWT, whitelist []string) *AuthPlugin {
	return &AuthPlugin{jwt: j, whitelist: whitelist}
}

func (p *AuthPlugin) Name() string     { return "auth" }
func (p *AuthPlugin) Priority() int    { return 100 }

func (p *AuthPlugin) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 白名单免认证
		for _, prefix := range p.whitelist {
			if len(r.URL.Path) >= len(prefix) && r.URL.Path[:len(prefix)] == prefix {
				next.ServeHTTP(w, r)
				return
			}
		}

		token := r.Header.Get("Authorization")
		if token == "" || len(token) < 8 {
			http.Error(w, `{"code":401,"message":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		claims, err := p.jwt.ParseToken(token[7:]) // skip "Bearer "
		if err != nil {
			http.Error(w, `{"code":401,"message":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		r.Header.Set("X-User-Id", fmt.Sprintf("%d", claims.UserID))
		next.ServeHTTP(w, r)
	})
}

// 占位
var _ = time.Now
