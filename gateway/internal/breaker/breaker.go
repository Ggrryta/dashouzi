package breaker

import (
	"sync"
	"time"
)

type State int

const (
	StateClosed   State = iota // 正常
	StateOpen                  // 熔断
	StateHalfOpen              // 试探
)

func (s State) String() string {
	switch s {
	case StateClosed:   return "closed"
	case StateOpen:     return "open"
	case StateHalfOpen: return "halfopen"
	default:            return "unknown"
	}
}

type Config struct {
	Threshold int           // 连续错误数阈值
	CoolDown  time.Duration // 冷却时间
}

type CircuitBreaker struct {
	mu           sync.Mutex
	state        State
	failureCount int
	config       Config
	lastFailure  time.Time
}

func New(cfg Config) *CircuitBreaker {
	return &CircuitBreaker{
		state:  StateClosed,
		config: cfg,
	}
}

func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// 冷却到了 → HalfOpen
	if cb.state == StateOpen && time.Since(cb.lastFailure) > cb.config.CoolDown {
		cb.state = StateHalfOpen
	}
	return cb.state
}

func (cb *CircuitBreaker) Failures() int {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.failureCount
}

func (cb *CircuitBreaker) Allow() bool {
	return cb.State() == StateClosed || cb.State() == StateHalfOpen
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == StateHalfOpen {
		cb.state = StateClosed
		cb.failureCount = 0
	}
}

func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = time.Now()
	cb.failureCount++

	if cb.state == StateHalfOpen || (cb.state == StateClosed && cb.failureCount >= cb.config.Threshold) {
		cb.state = StateOpen
	}
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
}
