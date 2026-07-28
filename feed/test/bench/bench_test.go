//go:build bench
// +build bench

// 性能基准测试
// 运行: go test -tags=bench -bench=. -benchtime=10s ./test/bench/...
package bench

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

var baseURL = "http://localhost:8080"

func BenchmarkCreatePost(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			body := map[string]interface{}{
				"user_id": int64(1),
				"content": "benchmark post content",
			}
			data, _ := json.Marshal(body)
			resp, err := http.Post(baseURL+"/api/v1/posts", "application/json", bytes.NewReader(data))
			if err == nil {
				resp.Body.Close()
			}
		}
	})
}

func BenchmarkGetTimeline(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			resp, err := http.Get(baseURL + "/api/v1/timeline?user_id=1&limit=20")
			if err == nil {
				resp.Body.Close()
			}
		}
	})
}

func BenchmarkFollow(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			i++
			body := map[string]interface{}{
				"follower_id": i % 1000,
				"followee_id": (i % 1000) + 1,
			}
			data, _ := json.Marshal(body)
			resp, err := http.Post(baseURL+"/api/v1/follow", "application/json", bytes.NewReader(data))
			if err == nil {
				resp.Body.Close()
			}
		}
	})
}
