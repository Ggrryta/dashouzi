package plugin

import (
	"net/http"
	"sort"
)

// Plugin 插件接口
type Plugin interface {
	Name() string
	Priority() int
	Handle(next http.Handler) http.Handler
}

// Registry 插件注册器
type Registry struct {
	plugins []Plugin
}

func NewRegistry() *Registry {
	return &Registry{}
}

func (r *Registry) Register(p Plugin) {
	r.plugins = append(r.plugins, p)
}

// GetChain 按优先级排序返回插件链
func (r *Registry) GetChain() []Plugin {
	sorted := make([]Plugin, len(r.plugins))
	copy(sorted, r.plugins)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority() > sorted[j].Priority()
	})
	return sorted
}

// BuildChain 构建完整的中间件链：plugin1 → plugin2 → ... → handler
func (r *Registry) BuildChain(final http.Handler) http.Handler {
	chain := r.GetChain()
	h := final
	for i := len(chain) - 1; i >= 0; i-- {
		h = chain[i].Handle(h)
	}
	return h
}
