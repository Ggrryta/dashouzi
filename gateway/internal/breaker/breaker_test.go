package breaker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker_InitialStateClosed(t *testing.T) {
	cb := New(Config{Threshold: 5, CoolDown: time.Second})
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_RecordFailureIncrements(t *testing.T) {
	cb := New(Config{Threshold: 5, CoolDown: time.Second})
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, 2, cb.Failures())
}

func TestCircuitBreaker_ThresholdTripsToOpen(t *testing.T) {
	cb := New(Config{Threshold: 3, CoolDown: time.Second})
	cb.RecordFailure() // 1
	cb.RecordFailure() // 2
	assert.Equal(t, StateClosed, cb.State())
	cb.RecordFailure() // 3 → Open!
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_OpenRejectsAll(t *testing.T) {
	cb := New(Config{Threshold: 1, CoolDown: time.Second})
	cb.RecordFailure() // → Open
	assert.False(t, cb.Allow())
	assert.False(t, cb.Allow())
}

func TestCircuitBreaker_CoolDownGoesHalfOpen(t *testing.T) {
	cb := New(Config{Threshold: 1, CoolDown: 50 * time.Millisecond})
	cb.RecordFailure() // → Open
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreaker_HalfOpenSuccess_GoesClosed(t *testing.T) {
	cb := New(Config{Threshold: 1, CoolDown: 50 * time.Millisecond})
	cb.RecordFailure() // → Open
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())

	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.Failures()) // 重置
}

func TestCircuitBreaker_HalfOpenFailure_BackToOpen(t *testing.T) {
	cb := New(Config{Threshold: 1, CoolDown: 50 * time.Millisecond})
	cb.RecordFailure() // → Open
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, StateHalfOpen, cb.State())

	cb.RecordFailure() // 试探失败
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := New(Config{Threshold: 3, CoolDown: time.Second})
	cb.RecordFailure()
	cb.RecordFailure()
	cb.Reset()
	assert.Equal(t, StateClosed, cb.State())
	assert.Equal(t, 0, cb.Failures())
}
