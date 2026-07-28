package router

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouteTable_Match_Exact(t *testing.T) {
	table := NewTable([]Route{
		{Path: "/blog", Upstream: "http://bloger:8080"},
		{Path: "/seckill", Upstream: "http://seckill:8081"},
	})

	r, _ := table.Match("/blog")
	assert.NotNil(t, r)
	assert.Equal(t, "http://bloger:8080", r.Upstream)
}

func TestRouteTable_Match_Prefix(t *testing.T) {
	table := NewTable([]Route{
		{Path: "/blog", Upstream: "http://bloger:8080"},
	})

	r, remainder := table.Match("/blog/api/v1/ping")
	assert.NotNil(t, r)
	assert.Equal(t, "/api/v1/ping", remainder)
}

func TestRouteTable_Match_NoMatch(t *testing.T) {
	table := NewTable([]Route{
		{Path: "/blog", Upstream: "http://bloger:8080"},
	})

	r, _ := table.Match("/unknown")
	assert.Nil(t, r)
}

func TestRouteTable_Match_LongestPrefix(t *testing.T) {
	table := NewTable([]Route{
		{Path: "/blog", Upstream: "http://bloger:8080"},
		{Path: "/blog/admin", Upstream: "http://admin:9090"},
	})

	r, _ := table.Match("/blog/admin/dashboard")
	assert.NotNil(t, r)
	assert.Equal(t, "http://admin:9090", r.Upstream) // 最长前缀优先
}

func TestRouteTable_Match_KeepPrefix(t *testing.T) {
	table := NewTable([]Route{
		{Path: "/blog", Upstream: "http://bloger:8080", KeepPrefix: true},
	})

	_, remainder := table.Match("/blog/api/v1/ping")
	assert.Equal(t, "/blog/api/v1/ping", remainder) // 保留原路径
}
