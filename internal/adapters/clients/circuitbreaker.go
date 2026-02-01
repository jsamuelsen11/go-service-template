package clients

import (
	"sync"
	"time"
)

// State represents the current state of the circuit breaker.
type State int

const (
	// StateClosed is the normal operating state. Requests are allowed through.
	StateClosed State = iota

	// StateOpen is the failing state. Requests are blocked to prevent cascading failures.
	StateOpen

	// StateHalfOpen is the recovery testing state. Limited requests are allowed to probe recovery.
	StateHalfOpen
)

// String returns a human-readable name for the state.
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig configures the circuit breaker behavior.
type CircuitBreakerConfig struct {
	// MaxFailures is the number of consecutive failures before the circuit opens.
	MaxFailures int

	// Timeout is how long to wait in open state before transitioning to half-open.
	Timeout time.Duration

	// HalfOpenLimit is the number of consecutive successes in half-open state
	// required to close the circuit.
	HalfOpenLimit int
}

// CircuitBreaker implements the circuit breaker pattern.
// It protects downstream services by preventing requests when the service is unhealthy.
//
// State transitions:
//   - Closed → Open: After MaxFailures consecutive failures
//   - Open → HalfOpen: After Timeout duration has passed
//   - HalfOpen → Closed: After HalfOpenLimit consecutive successes
//   - HalfOpen → Open: On any failure
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            State
	failures         int       // consecutive failures in closed state
	successes        int       // consecutive successes in half-open state
	halfOpenRequests int       // current requests in flight during half-open state
	lastFailure      time.Time // time of last failure (for timeout calculation)
	cfg              CircuitBreakerConfig

	// onStateChange is called when the state changes. Can be used for logging/metrics.
	onStateChange func(from, to State)

	// now is a function that returns current time. Overridable for testing.
	now func() time.Time
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state: StateClosed,
		cfg:   cfg,
		now:   time.Now,
	}
}

// OnStateChange sets a callback that is invoked when the circuit state changes.
// This can be used for logging or updating metrics.
func (cb *CircuitBreaker) OnStateChange(fn func(from, to State)) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.onStateChange = fn
}

// Allow checks if a request should be allowed through.
// Returns true if the request should proceed, false if it should be blocked.
//
// This method may trigger a state transition from Open to HalfOpen if the timeout has passed.
// In half-open state, only HalfOpenLimit concurrent requests are allowed to probe recovery.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout has passed
		if cb.now().Sub(cb.lastFailure) >= cb.cfg.Timeout {
			cb.transitionTo(StateHalfOpen)
			cb.halfOpenRequests = 1
			return true // Allow one request through to probe
		}
		return false

	case StateHalfOpen:
		// Limit concurrent requests in half-open state
		if cb.halfOpenRequests >= cb.cfg.HalfOpenLimit {
			return false
		}
		cb.halfOpenRequests++
		return true

	default:
		return false
	}
}

// RecordSuccess records a successful request.
// In half-open state, this may transition to closed after enough successes.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		// Reset failure count on success
		cb.failures = 0

	case StateHalfOpen:
		cb.halfOpenRequests--
		cb.successes++
		if cb.successes >= cb.cfg.HalfOpenLimit {
			cb.transitionTo(StateClosed)
		}
	}
}

// RecordFailure records a failed request.
// In closed state, this may trigger transition to open after enough failures.
// In half-open state, any failure immediately reopens the circuit.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.lastFailure = cb.now()

	switch cb.state {
	case StateClosed:
		cb.failures++
		if cb.failures >= cb.cfg.MaxFailures {
			cb.transitionTo(StateOpen)
		}

	case StateHalfOpen:
		cb.halfOpenRequests--
		// Any failure in half-open immediately reopens
		cb.transitionTo(StateOpen)
	}
}

// State returns the current state of the circuit breaker.
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// transitionTo changes the circuit breaker state.
// Must be called with lock held.
func (cb *CircuitBreaker) transitionTo(newState State) {
	if cb.state == newState {
		return
	}

	oldState := cb.state
	cb.state = newState

	// Reset counters on state change
	cb.failures = 0
	cb.successes = 0

	// Notify callback if set
	if cb.onStateChange != nil {
		// Call in goroutine to avoid blocking while holding lock
		go cb.onStateChange(oldState, newState)
	}
}
