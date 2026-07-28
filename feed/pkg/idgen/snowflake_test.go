package idgen

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// T1.1: 全局唯一性
func TestSnowflake_Uniqueness(t *testing.T) {
	gen := NewSnowflake(1)
	seen := make(map[int64]bool)
	for i := 0; i < 1000; i++ {
		id := gen.NextID()
		assert.False(t, seen[id], "duplicate ID: %d", id)
		seen[id] = true
	}
}

// T1.2: 单调递增
func TestSnowflake_Monotonic(t *testing.T) {
	gen := NewSnowflake(1)
	prev := gen.NextID()
	for i := 0; i < 100; i++ {
		curr := gen.NextID()
		assert.True(t, curr > prev, "%d should > %d", curr, prev)
		prev = curr
	}
}

// T1.3: 并发安全
func TestSnowflake_Concurrent(t *testing.T) {
	gen := NewSnowflake(1)
	ids := make(chan int64, 1000)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ids <- gen.NextID()
			}
		}()
	}
	wg.Wait()
	close(ids)
	seen := make(map[int64]bool)
	for id := range ids {
		assert.False(t, seen[id], "concurrent duplicate: %d", id)
		seen[id] = true
	}
	assert.Equal(t, 1000, len(seen))
}
