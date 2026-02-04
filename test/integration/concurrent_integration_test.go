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
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

// testConcurrentConfig returns a config optimized for concurrent testing.
func testConcurrentConfig(baseURL string) *clients.Config {
	return &clients.Config{
		ServiceName: "concurrent-test-service",
		BaseURL:     baseURL,
		Timeout:     10 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     2,
			InitialInterval: 5 * time.Millisecond,
			MaxInterval:     20 * time.Millisecond,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   10, // Higher threshold for concurrent tests
			Timeout:       100 * time.Millisecond,
			HalfOpenLimit: 3,
		},
	}
}

// TestConcurrent_MultipleRequests verifies that multiple concurrent requests
// are handled correctly without race conditions.
func TestConcurrent_MultipleRequests(t *testing.T) {
	var serverCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&serverCalls, 1)
		// Simulate variable response times
		time.Sleep(time.Duration(5+atomic.LoadInt32(&serverCalls)%10) * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := testConcurrentConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	const numGoroutines = 50
	var wg sync.WaitGroup
	var successCount, errorCount int32

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp, err := client.Get(context.Background(), "/concurrent")
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
			}
			resp.Body.Close()
			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int32(numGoroutines), atomic.LoadInt32(&successCount), "all requests should succeed")
	assert.Equal(t, int32(0), atomic.LoadInt32(&errorCount), "no errors expected")
	assert.GreaterOrEqual(t, atomic.LoadInt32(&serverCalls), int32(numGoroutines), "server should handle all requests")
}

// TestConcurrent_ContextCancellation verifies that concurrent requests
// are properly cancelled when their contexts are cancelled.
func TestConcurrent_ContextCancellation(t *testing.T) {
	var startedRequests, completedRequests int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&startedRequests, 1)
		select {
		case <-r.Context().Done():
			// Request was cancelled
		case <-time.After(5 * time.Second):
			atomic.AddInt32(&completedRequests, 1)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cfg := testConcurrentConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	const numGoroutines = 10
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	var cancelledCount int32

	// Start concurrent requests
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Get(ctx, "/slow")
			if err != nil {
				atomic.AddInt32(&cancelledCount, 1)
			}
		}()
	}

	// Wait a bit then cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	wg.Wait()

	// All requests should have been cancelled or encountered errors
	assert.Greater(t, atomic.LoadInt32(&cancelledCount), int32(0), "some requests should be cancelled")
	assert.Equal(t, int32(0), atomic.LoadInt32(&completedRequests), "no requests should complete")
}

// TestConcurrent_CircuitBreakerUnderLoad verifies that the circuit breaker
// behaves correctly under concurrent load with failures.
func TestConcurrent_CircuitBreakerUnderLoad(t *testing.T) {
	var serverCalls int32
	var failCount int32 = 0 // First N requests will fail

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&serverCalls, 1)
		// First 5 calls fail, then succeed
		if call <= 5 {
			atomic.AddInt32(&failCount, 1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := testConcurrentConfig(server.URL)
	cfg.Retry.MaxAttempts = 1
	cfg.Circuit.MaxFailures = 3
	cfg.Circuit.Timeout = 50 * time.Millisecond

	client, err := clients.New(cfg)
	require.NoError(t, err)

	// First wave: trigger failures to open circuit
	var wg sync.WaitGroup
	var circuitOpenErrors int32

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.Get(context.Background(), "/test")
			if err != nil && err == clients.ErrCircuitOpen {
				atomic.AddInt32(&circuitOpenErrors, 1)
			}
		}()
		time.Sleep(5 * time.Millisecond) // Slight delay between requests
	}

	wg.Wait()

	// Some requests should have been blocked by circuit breaker
	assert.Greater(t, atomic.LoadInt32(&circuitOpenErrors), int32(0), "some requests should hit circuit breaker")

	// Wait for circuit to transition to half-open
	time.Sleep(60 * time.Millisecond)

	// Second wave: circuit should recover
	var successCount int32
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := client.Get(context.Background(), "/test")
			if err == nil {
				resp.Body.Close()
				atomic.AddInt32(&successCount, 1)
			}
		}()
		time.Sleep(10 * time.Millisecond)
	}

	wg.Wait()

	// Circuit should have recovered and some requests should succeed
	assert.Greater(t, atomic.LoadInt32(&successCount), int32(0), "circuit should recover")
}

// TestConcurrent_SharedClient verifies that a single client instance
// can be safely shared across multiple goroutines.
func TestConcurrent_SharedClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"test"}`))
	}))
	defer server.Close()

	cfg := testConcurrentConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	// Simulate multiple "services" using the same client
	const numServices = 5
	const requestsPerService = 20

	var wg sync.WaitGroup
	results := make(chan error, numServices*requestsPerService)

	for service := 0; service < numServices; service++ {
		wg.Add(1)
		go func(serviceID int) {
			defer wg.Done()
			for i := 0; i < requestsPerService; i++ {
				resp, err := client.Get(context.Background(), "/service")
				if err != nil {
					results <- err
					continue
				}
				resp.Body.Close()
				results <- nil
			}
		}(service)
	}

	wg.Wait()
	close(results)

	var errors []error
	for err := range results {
		if err != nil {
			errors = append(errors, err)
		}
	}

	assert.Empty(t, errors, "no errors expected when sharing client across goroutines")
}

// TestConcurrent_MixedMethods verifies that concurrent requests using
// different HTTP methods work correctly.
func TestConcurrent_MixedMethods(t *testing.T) {
	var getCalls, postCalls, putCalls, deleteCalls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			atomic.AddInt32(&getCalls, 1)
		case http.MethodPost:
			atomic.AddInt32(&postCalls, 1)
		case http.MethodPut:
			atomic.AddInt32(&putCalls, 1)
		case http.MethodDelete:
			atomic.AddInt32(&deleteCalls, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := testConcurrentConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	var wg sync.WaitGroup
	const iterations = 10

	// Launch concurrent requests with different methods
	for i := 0; i < iterations; i++ {
		wg.Add(4)

		go func() {
			defer wg.Done()
			resp, err := client.Get(context.Background(), "/resource")
			if err == nil {
				resp.Body.Close()
			}
		}()

		go func() {
			defer wg.Done()
			resp, err := client.Post(context.Background(), "/resource", nil)
			if err == nil {
				resp.Body.Close()
			}
		}()

		go func() {
			defer wg.Done()
			resp, err := client.Put(context.Background(), "/resource", nil)
			if err == nil {
				resp.Body.Close()
			}
		}()

		go func() {
			defer wg.Done()
			resp, err := client.Delete(context.Background(), "/resource")
			if err == nil {
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()

	assert.Equal(t, int32(iterations), atomic.LoadInt32(&getCalls), "GET calls mismatch")
	assert.Equal(t, int32(iterations), atomic.LoadInt32(&postCalls), "POST calls mismatch")
	assert.Equal(t, int32(iterations), atomic.LoadInt32(&putCalls), "PUT calls mismatch")
	assert.Equal(t, int32(iterations), atomic.LoadInt32(&deleteCalls), "DELETE calls mismatch")
}
