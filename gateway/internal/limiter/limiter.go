package limiter

import "time"

// Limiter 限流器接口
type Limiter interface {
	Allow(key string) bool
}

// ====== 固定窗口 ======

type FixedWindow struct {
	limit  int
	window time.Duration
	store  map[string]*fixedEntry
}

type fixedEntry struct {
	count  int
	window time.Time
}

func NewFixedWindow(limit int, window time.Duration) *FixedWindow {
	return &FixedWindow{limit: limit, window: window, store: make(map[string]*fixedEntry)}
}

func (l *FixedWindow) Allow(key string) bool {
	entry, ok := l.store[key]
	now := time.Now()
	if !ok || now.Sub(entry.window) > l.window {
		l.store[key] = &fixedEntry{count: 1, window: now}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	return true
}

// ====== 滑动窗口 ======

type SlidingWindow struct {
	limit  int
	window time.Duration
	store  map[string][]time.Time
}

func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{limit: limit, window: window, store: make(map[string][]time.Time)}
}

func (l *SlidingWindow) Allow(key string) bool {
	return l.AllowAt(key, time.Now())
}

func (l *SlidingWindow) AllowAt(key string, t time.Time) bool {
	times := l.store[key]
	cutoff := t.Add(-l.window)

	// 滑出旧请求
	var keep []time.Time
	for _, tt := range times {
		if tt.After(cutoff) {
			keep = append(keep, tt)
		}
	}

	if len(keep) >= l.limit {
		l.store[key] = keep
		return false
	}

	keep = append(keep, t)
	l.store[key] = keep
	return true
}

// ====== 令牌桶 ======

type TokenBucket struct {
	rate     float64 // tokens per second
	capacity float64
	tokens   float64
	last     time.Time
	store    map[string]*tokenEntry
}

type tokenEntry struct {
	tokens float64
	last   time.Time
}

func NewTokenBucket(capacity int, fillDuration time.Duration) *TokenBucket {
	rate := float64(capacity) / fillDuration.Seconds()
	return &TokenBucket{
		rate:     rate,
		capacity: float64(capacity),
		store:    make(map[string]*tokenEntry),
		last:     time.Now(),
	}
}

func (b *TokenBucket) Allow(key string) bool {
	e, ok := b.store[key]
	now := time.Now()
	if !ok {
		b.store[key] = &tokenEntry{tokens: b.capacity - 1, last: now}
		return true
	}

	elapsed := now.Sub(e.last).Seconds()
	e.tokens += elapsed * b.rate
	if e.tokens > b.capacity {
		e.tokens = b.capacity
	}
	e.last = now

	if e.tokens < 1 {
		return false
	}
	e.tokens--
	return true
}

// ====== 漏桶 ======

type LeakyBucket struct {
	rate     float64 // requests per second
	capacity int
	store    map[string]*leakyEntry
}

type leakyEntry struct {
	water  float64
	last   time.Time
}

func NewLeakyBucket(capacity int, drainDuration time.Duration) *LeakyBucket {
	rate := float64(capacity) / drainDuration.Seconds()
	return &LeakyBucket{
		rate:     rate,
		capacity: capacity,
		store:    make(map[string]*leakyEntry),
	}
}

func (b *LeakyBucket) Allow(key string) bool {
	e, ok := b.store[key]
	now := time.Now()
	if !ok {
		b.store[key] = &leakyEntry{water: 1, last: now}
		return true
	}

	elapsed := now.Sub(e.last).Seconds()
	e.water -= elapsed * b.rate
	if e.water < 0 {
		e.water = 0
	}
	e.last = now

	if e.water >= float64(b.capacity) {
		return false
	}
	e.water++
	return true
}
