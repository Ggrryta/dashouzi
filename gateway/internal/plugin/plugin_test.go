package plugin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakePlugin struct {
	name     string
	priority int
	called   bool
}

func (p *fakePlugin) Name() string     { return p.name }
func (p *fakePlugin) Priority() int    { return p.priority }
func (p *fakePlugin) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.called = true
		next.ServeHTTP(w, r)
	})
}

func TestRegistry_Register(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&fakePlugin{name: "auth", priority: 100})
	assert.Len(t, reg.GetChain(), 1)
}

func TestRegistry_OrderedByPriority(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&fakePlugin{name: "low", priority: 10})
	reg.Register(&fakePlugin{name: "high", priority: 100})
	reg.Register(&fakePlugin{name: "mid", priority: 50})

	chain := reg.GetChain()
	assert.Len(t, chain, 3)
	assert.Equal(t, "high", chain[0].Name()) // 100 first
	assert.Equal(t, "mid", chain[1].Name())  // 50
	assert.Equal(t, "low", chain[2].Name())  // 10 last
}

func TestChain_AllPluginsCalled(t *testing.T) {
	reg := NewRegistry()
	a := &fakePlugin{name: "a", priority: 10}
	b := &fakePlugin{name: "b", priority: 20}
	reg.Register(a)
	reg.Register(b)

	handler := reg.BuildChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, a.called)
	assert.True(t, b.called)
	assert.Equal(t, 200, w.Code)
}

func TestChain_PluginRejectsRequest(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&rejectPlugin{name: "reject"})
	reg.Register(&fakePlugin{name: "never_called", priority: 10})

	handler := reg.BuildChain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be reached")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

type rejectPlugin struct{ name string }

func (p *rejectPlugin) Name() string  { return p.name }
func (p *rejectPlugin) Priority() int { return 100 }
func (p *rejectPlugin) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		// 不调用 next → 中断链
	})
}
