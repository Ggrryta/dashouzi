package limiter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ====== 固定窗口 ======

func TestFixedWindow_UnderLimit(t *testing.T) {
	l := NewFixedWindow(5, time.Second)
	for i := 0; i < 5; i++ {
		assert.True(t, l.Allow("u1"), "req %d", i)
	}
}

func TestFixedWindow_Exceeded(t *testing.T) {
	l := NewFixedWindow(5, time.Second)
	for i := 0; i < 5; i++ {
		l.Allow("u1")
	}
	assert.False(t, l.Allow("u1"), "6th should be denied")
}

func TestFixedWindow_DifferentKeys(t *testing.T) {
	l := NewFixedWindow(1, time.Second)
	l.Allow("u1")
	assert.True(t, l.Allow("u2"), "different key should be allowed")
}

func TestFixedWindow_Expires(t *testing.T) {
	l := NewFixedWindow(1, time.Millisecond*100)
	l.Allow("u1")
	assert.False(t, l.Allow("u1"))
	time.Sleep(150 * time.Millisecond)
	assert.True(t, l.Allow("u1"), "window expired, should allow again")
}

// ====== 滑动窗口 ======

func TestSlidingWindow_UnderLimit(t *testing.T) {
	l := NewSlidingWindow(5, time.Second)
	for i := 0; i < 5; i++ {
		assert.True(t, l.Allow("u1"))
	}
}

func TestSlidingWindow_OldRequestsSlideOut(t *testing.T) {
	l := NewSlidingWindow(5, time.Second)
	old := time.Now().Add(-1100 * time.Millisecond)
	for i := 0; i < 5; i++ {
		l.AllowAt("u1", old)
	}
	// 旧请求已滑出窗口，新请求应全部放行
	for i := 0; i < 5; i++ {
		assert.True(t, l.Allow("u1"), "all new should pass")
	}
}

// ====== 令牌桶 ======

func TestTokenBucket_InitialTokens(t *testing.T) {
	b := NewTokenBucket(10, time.Second)
	for i := 0; i < 10; i++ {
		assert.True(t, b.Allow("k"))
	}
	assert.False(t, b.Allow("k"), "out of tokens")
}

func TestTokenBucket_Refill(t *testing.T) {
	b := NewTokenBucket(10, time.Second) // 1 token per 100ms
	for i := 0; i < 10; i++ {
		b.Allow("k")
	}
	time.Sleep(250 * time.Millisecond) // ~2.5 tokens refilled
	assert.True(t, b.Allow("k"))
	assert.True(t, b.Allow("k"))
	assert.False(t, b.Allow("k"), "only ~2.5 refilled")
}

// ====== 漏桶 ======

func TestLeakyBucket_QueueAndOverflow(t *testing.T) {
	b := NewLeakyBucket(5, time.Millisecond*100) // capacity=5, rate=5/100ms
	for i := 0; i < 5; i++ {
		assert.True(t, b.Allow("k"), "first 5 should be queued")
	}
	assert.False(t, b.Allow("k"), "6th should overflow")
}

func TestLeakyBucket_DrainOverTime(t *testing.T) {
	b := NewLeakyBucket(10, time.Millisecond*50) // rate=10/50ms
	for i := 0; i < 5; i++ {
		b.Allow("k")
	}
	time.Sleep(100 * time.Millisecond) // ~20 drained
	// 应可以再次入队
	assert.True(t, b.Allow("k"))
}
