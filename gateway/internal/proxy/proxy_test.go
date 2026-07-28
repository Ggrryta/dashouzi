package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gateway/internal/router"
)

func TestReverseProxy_ForwardsToUpstream(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":"pong"}`))
	}))
	defer upstream.Close()

	table := router.NewTable([]router.Route{
		{Path: "/blog", Upstream: upstream.URL},
	})
	handler := NewHandler(table)

	req := httptest.NewRequest("GET", "/blog/api/v1/ping", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body, _ := io.ReadAll(w.Body)
	assert.Contains(t, string(body), "pong")
}

func TestReverseProxy_UpstreamReturnsActualPath(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(200)
	}))
	defer upstream.Close()

	table := router.NewTable([]router.Route{
		{Path: "/blog", Upstream: upstream.URL},
	})
	handler := NewHandler(table)

	req := httptest.NewRequest("GET", "/blog/api/v1/users", nil)
	handler(httptest.NewRecorder(), req)

	assert.Equal(t, "/api/v1/users", receivedPath) // 前缀已剥离
}

func TestReverseProxy_NoRoute_404(t *testing.T) {
	table := router.NewTable([]router.Route{
		{Path: "/blog", Upstream: "http://bloger:8080"},
	})
	handler := NewHandler(table)

	req := httptest.NewRequest("GET", "/unknown/path", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestReverseProxy_UpstreamDown_Returns502(t *testing.T) {
	table := router.NewTable([]router.Route{
		{Path: "/blog", Upstream: "http://127.0.0.1:19999"},
	})
	handler := NewHandler(table)

	req := httptest.NewRequest("GET", "/blog/ping", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	assert.Equal(t, http.StatusBadGateway, w.Code)
}
