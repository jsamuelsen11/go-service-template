package ports

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrDuplicateChecker is returned when attempting to register a health checker
// with a name that is already registered.
var ErrDuplicateChecker = errors.New("duplicate health checker")

// HealthChecker is implemented by components that can report their health.
// Adapters register themselves with the HealthRegistry at startup.
//
// Example implementation:
//
//	type DatabaseAdapter struct { ... }
//
//	func (d *DatabaseAdapter) Name() string { return "postgres" }
//
//	func (d *DatabaseAdapter) Check(ctx context.Context) error {
//	    return d.db.PingContext(ctx)
//	}
type HealthChecker interface {
	// Name returns a unique identifier for this health check.
	// Used in health check responses to identify which component failed.
	Name() string

	// Check performs the health check and returns an error if unhealthy.
	// Implementations should respect context cancellation and deadlines.
	// A nil return indicates the component is healthy.
	Check(ctx context.Context) error
}

// HealthRegistry aggregates health checks from multiple components.
// Components register themselves at startup, and the registry
// runs all checks when queried.
type HealthRegistry interface {
	// Register adds a health checker to the registry.
	// Returns an error if a checker with the same name is already registered.
	// Should be called during application startup.
	Register(checker HealthChecker) error

	// CheckAll runs all registered health checks and returns aggregated results.
	// Checks run concurrently with the provided context timeout.
	CheckAll(ctx context.Context) *HealthResult
}

// HealthStatus represents the overall health state.
type HealthStatus string

const (
	// HealthStatusHealthy indicates all checks passed.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusUnhealthy indicates critical checks failed.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// HealthResult contains the aggregated health check results.
type HealthResult struct {
	// Status is the overall health status.
	Status HealthStatus `json:"status"`

	// Checks contains individual check results keyed by checker name.
	Checks map[string]*CheckResult `json:"checks"`

	// Timestamp is when the health check was performed.
	Timestamp time.Time `json:"timestamp"`
}

// CheckResult contains the result of a single health check.
type CheckResult struct {
	// Status is the health status of this component.
	Status HealthStatus `json:"status"`

	// Message provides additional context, especially on failure.
	Message string `json:"message,omitempty"`

	// Duration is how long the check took.
	Duration time.Duration `json:"duration"`
}

// DefaultHealthRegistry is a thread-safe implementation of HealthRegistry.
type DefaultHealthRegistry struct {
	mu       sync.RWMutex
	checkers []HealthChecker
}

// NewHealthRegistry creates a new health registry.
func NewHealthRegistry() *DefaultHealthRegistry {
	return &DefaultHealthRegistry{
		checkers: make([]HealthChecker, 0),
	}
}

// Register adds a health checker to the registry.
// Returns an error if a checker with the same name is already registered.
func (r *DefaultHealthRegistry) Register(checker HealthChecker) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := checker.Name()
	for _, c := range r.checkers {
		if c.Name() == name {
			return fmt.Errorf("%w: %s", ErrDuplicateChecker, name)
		}
	}

	r.checkers = append(r.checkers, checker)

	return nil
}

// CheckAll runs all registered health checks concurrently.
func (r *DefaultHealthRegistry) CheckAll(ctx context.Context) *HealthResult {
	r.mu.RLock()
	checkers := make([]HealthChecker, len(r.checkers))
	copy(checkers, r.checkers)
	r.mu.RUnlock()

	result := &HealthResult{
		Status:    HealthStatusHealthy,
		Checks:    make(map[string]*CheckResult),
		Timestamp: time.Now(),
	}

	if len(checkers) == 0 {
		return result
	}

	// Run checks concurrently.
	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	for _, checker := range checkers {
		wg.Add(1)

		go func(c HealthChecker) {
			defer wg.Done()

			start := time.Now()
			err := c.Check(ctx)
			duration := time.Since(start)

			checkResult := &CheckResult{
				Status:   HealthStatusHealthy,
				Duration: duration,
			}

			if err != nil {
				checkResult.Status = HealthStatusUnhealthy
				checkResult.Message = err.Error()
			}

			mu.Lock()

			result.Checks[c.Name()] = checkResult
			if checkResult.Status == HealthStatusUnhealthy {
				result.Status = HealthStatusUnhealthy
			}

			mu.Unlock()
		}(checker)
	}

	wg.Wait()

	return result
}
