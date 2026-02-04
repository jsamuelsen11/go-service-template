//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/adapters/http/middleware"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

// testClientConfig returns a minimal config for integration testing.
func testClientConfig(baseURL string) *clients.Config {
	return &clients.Config{
		ServiceName: "integration-test-service",
		BaseURL:     baseURL,
		Timeout:     5 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   3,
			Timeout:       100 * time.Millisecond,
			HalfOpenLimit: 2,
		},
	}
}

// TestClient_RetryBehavior_TransientFailures verifies that the client
// retries on transient server failures and eventually succeeds.
func TestClient_RetryBehavior_TransientFailures(t *testing.T) {
	var attempts int32

	// Server fails twice, then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := testClientConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts), "expected 3 attempts (2 failures + 1 success)")
}

// TestClient_CircuitBreaker_StateTransitions verifies the circuit breaker
// transitions through all states correctly.
func TestClient_CircuitBreaker_StateTransitions(t *testing.T) {
	var calls int32
	var shouldFail atomic.Bool
	shouldFail.Store(true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if shouldFail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := testClientConfig(server.URL)
	cfg.Retry.MaxAttempts = 1 // No retries for clearer circuit breaker testing
	cfg.Circuit.MaxFailures = 2
	cfg.Circuit.Timeout = 50 * time.Millisecond

	client, err := clients.New(cfg)
	require.NoError(t, err)

	// Phase 1: Closed state - failures accumulate
	assert.Equal(t, clients.StateClosed, client.CircuitState())

	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
	assert.Equal(t, clients.StateClosed, client.CircuitState())

	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)

	// Phase 2: Open state - circuit should be open after 2 failures
	assert.Equal(t, clients.StateOpen, client.CircuitState())

	// Phase 3: Requests should be blocked (circuit open)
	callsBefore := atomic.LoadInt32(&calls)
	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
	assert.ErrorIs(t, err, clients.ErrCircuitOpen)
	assert.Equal(t, callsBefore, atomic.LoadInt32(&calls), "no server call when circuit is open")

	// Phase 4: Wait for timeout, then circuit should transition to half-open
	time.Sleep(60 * time.Millisecond)
	shouldFail.Store(false) // Server now succeeds

	// First success in half-open
	resp, err := client.Get(context.Background(), "/test")
	require.NoError(t, err)
	resp.Body.Close()

	// Second success should close the circuit
	resp, err = client.Get(context.Background(), "/test")
	require.NoError(t, err)
	resp.Body.Close()

	// Phase 5: Circuit should be closed again
	assert.Equal(t, clients.StateClosed, client.CircuitState())
}

// TestClient_Timeout_SlowResponse verifies the client times out
// when the server responds slowly.
func TestClient_Timeout_SlowResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Slower than client timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := testClientConfig(server.URL)
	cfg.Timeout = 50 * time.Millisecond
	cfg.Retry.MaxAttempts = 1

	client, err := clients.New(cfg)
	require.NoError(t, err)

	start := time.Now()
	_, err = client.Get(context.Background(), "/slow")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 200*time.Millisecond, "should timeout quickly")
}

// TestClient_ConcurrentRequests_WithCircuitBreaker verifies the client
// handles concurrent requests correctly with circuit breaker.
func TestClient_ConcurrentRequests_WithCircuitBreaker(t *testing.T) {
	var totalCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&totalCalls, 1)
		time.Sleep(10 * time.Millisecond) // Simulate some work
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := testClientConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	const numGoroutines = 10
	var wg sync.WaitGroup
	var successCount int32
	var errorCount int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(context.Background(), "/concurrent")
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}
			resp.Body.Close()
			atomic.AddInt32(&successCount, 1)
		}()
	}

	wg.Wait()

	// All requests should succeed
	assert.Equal(t, int32(numGoroutines), atomic.LoadInt32(&successCount), "all concurrent requests should succeed")
	assert.Equal(t, int32(0), atomic.LoadInt32(&errorCount), "no errors expected")
	assert.Equal(t, int32(numGoroutines), atomic.LoadInt32(&totalCalls), "server should receive all calls")
}

// TestClient_HeaderPropagation_Integration verifies that request ID
// and correlation ID headers are propagated correctly.
func TestClient_HeaderPropagation_Integration(t *testing.T) {
	var receivedRequestID, receivedCorrelationID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequestID = r.Header.Get(middleware.HeaderRequestID)
		receivedCorrelationID = r.Header.Get(middleware.HeaderCorrelationID)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := testClientConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	// Set up context with request tracking headers
	ctx := context.Background()
	ctx = middleware.ContextWithRequestID(ctx, "req-integration-123")
	ctx = middleware.ContextWithCorrelationID(ctx, "corr-integration-456")

	resp, err := client.Get(ctx, "/headers")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "req-integration-123", receivedRequestID)
	assert.Equal(t, "corr-integration-456", receivedCorrelationID)
}

// TestClient_ContextCancellation_Integration verifies that requests
// are properly cancelled when the context is cancelled.
func TestClient_ContextCancellation_Integration(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCompleted := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done() // Wait for cancellation
		close(requestCompleted)
	}))
	defer server.Close()

	cfg := testClientConfig(server.URL)
	cfg.Timeout = 5 * time.Second // Long timeout to ensure context cancellation triggers first

	client, err := clients.New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		<-requestStarted
		cancel()
	}()

	start := time.Now()
	_, err = client.Get(ctx, "/cancel")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, time.Second, "cancellation should be prompt")

	// Wait for server handler to complete
	select {
	case <-requestCompleted:
		// Good, server saw the cancellation
	case <-time.After(time.Second):
		t.Fatal("server did not receive cancellation")
	}
}
