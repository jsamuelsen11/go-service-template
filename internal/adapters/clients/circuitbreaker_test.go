package clients

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   5,
		Timeout:       30 * time.Second,
		HalfOpenLimit: 3,
	})

	assert.Equal(t, StateClosed, cb.State())
	assert.True(t, cb.Allow())
}

func TestCircuitBreaker_ClosedToOpen(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   3,
		Timeout:       30 * time.Second,
		HalfOpenLimit: 2,
	})

	// Record failures up to threshold
	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State())

	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State())

	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())

	// Should block requests when open
	assert.False(t, cb.Allow())
}

func TestCircuitBreaker_SuccessResetsFailures(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   3,
		Timeout:       30 * time.Second,
		HalfOpenLimit: 2,
	})

	// Record some failures
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State())

	// Success resets counter
	cb.RecordSuccess()

	// Need full 3 failures again to open
	cb.RecordFailure()
	cb.RecordFailure()
	assert.Equal(t, StateClosed, cb.State())

	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_OpenToHalfOpen(t *testing.T) {
	now := time.Now()

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   1,
		Timeout:       100 * time.Millisecond,
		HalfOpenLimit: 2,
	})
	cb.now = func() time.Time { return now }

	// Trip the circuit
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())
	assert.False(t, cb.Allow())

	// Advance time past timeout
	now = now.Add(150 * time.Millisecond)

	// Should transition to half-open on Allow()
	assert.True(t, cb.Allow())
	assert.Equal(t, StateHalfOpen, cb.State())
}

func TestCircuitBreaker_HalfOpenToClosed(t *testing.T) {
	now := time.Now()

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   1,
		Timeout:       100 * time.Millisecond,
		HalfOpenLimit: 2,
	})
	cb.now = func() time.Time { return now }

	// Trip the circuit and wait for half-open
	cb.RecordFailure()
	now = now.Add(150 * time.Millisecond)
	cb.Allow()
	assert.Equal(t, StateHalfOpen, cb.State())

	// Record successes to close
	cb.RecordSuccess()
	assert.Equal(t, StateHalfOpen, cb.State())

	cb.RecordSuccess()
	assert.Equal(t, StateClosed, cb.State())
}

func TestCircuitBreaker_HalfOpenToOpen(t *testing.T) {
	now := time.Now()

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   1,
		Timeout:       100 * time.Millisecond,
		HalfOpenLimit: 2,
	})
	cb.now = func() time.Time { return now }

	// Trip the circuit and wait for half-open
	cb.RecordFailure()
	now = now.Add(150 * time.Millisecond)
	cb.Allow()
	assert.Equal(t, StateHalfOpen, cb.State())

	// Any failure in half-open immediately reopens
	cb.RecordFailure()
	assert.Equal(t, StateOpen, cb.State())
}

func TestCircuitBreaker_OnStateChange(t *testing.T) {
	var transitions []struct {
		from, to State
	}
	var mu sync.Mutex

	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   1,
		Timeout:       10 * time.Millisecond,
		HalfOpenLimit: 1,
	})

	cb.OnStateChange(func(from, to State) {
		mu.Lock()
		transitions = append(transitions, struct{ from, to State }{from, to})
		mu.Unlock()
	})

	// Trip the circuit
	cb.RecordFailure()

	// Wait for callback to execute (it's async)
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	require.Len(t, transitions, 1)
	assert.Equal(t, StateClosed, transitions[0].from)
	assert.Equal(t, StateOpen, transitions[0].to)
	mu.Unlock()
}

func TestCircuitBreaker_Concurrent(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   100,
		Timeout:       time.Second,
		HalfOpenLimit: 10,
	})

	var wg sync.WaitGroup
	var allows int64
	var denies int64

	// Run many concurrent operations
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if cb.Allow() {
				atomic.AddInt64(&allows, 1)
				if atomic.LoadInt64(&allows)%2 == 0 {
					cb.RecordSuccess()
				} else {
					cb.RecordFailure()
				}
			} else {
				atomic.AddInt64(&denies, 1)
			}
		}()
	}

	wg.Wait()

	// Should not panic or deadlock
	// State should be valid
	state := cb.State()
	assert.Contains(t, []State{StateClosed, StateOpen, StateHalfOpen}, state)
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}
