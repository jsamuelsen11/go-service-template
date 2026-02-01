// Package clients provides HTTP client adapters for downstream services.
package clients

import "errors"

// Client errors represent failures in the HTTP client layer.
// These are distinct from domain errors - they represent infrastructure failures
// that should be translated to domain errors by the calling code.
var (
	// ErrCircuitOpen is returned when the circuit breaker is open.
	// This indicates the downstream service is unhealthy and requests are being blocked.
	ErrCircuitOpen = errors.New("circuit breaker open")

	// ErrMaxRetriesExceeded is returned after all retry attempts have been exhausted.
	// The original error is wrapped for context.
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)
