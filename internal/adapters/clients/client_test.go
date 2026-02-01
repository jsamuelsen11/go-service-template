package clients

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/middleware"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

func defaultConfig() *Config {
	return &Config{
		ServiceName: "test-service",
		Timeout:     5 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     3,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   5,
			Timeout:       time.Second,
			HalfOpenLimit: 2,
		},
	}
}

// closeBody is a test helper that closes the response body and fails the test on error.
func closeBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if err := resp.Body.Close(); err != nil {
		t.Errorf("failed to close response body: %v", err)
	}
}

func TestNew_RequiresConfig(t *testing.T) {
	_, err := New(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config is required")
}

func TestNew_RequiresServiceName(t *testing.T) {
	cfg := defaultConfig()
	cfg.ServiceName = ""

	_, err := New(cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service name is required")
}

func TestNew_Success(t *testing.T) {
	cfg := defaultConfig()
	cfg.BaseURL = "https://api.example.com"

	client, err := New(cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "https://api.example.com", client.baseURL)
}

func TestClient_HeaderPropagation(t *testing.T) {
	var receivedRequestID string
	var receivedCorrelationID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedRequestID = r.Header.Get(middleware.HeaderRequestID)
		receivedCorrelationID = r.Header.Get(middleware.HeaderCorrelationID)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL

	client, err := New(cfg)
	require.NoError(t, err)

	ctx := context.Background()
	ctx = middleware.ContextWithRequestID(ctx, "test-request-123")
	ctx = middleware.ContextWithCorrelationID(ctx, "test-correlation-456")

	resp, err := client.Get(ctx, "/test")
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, "test-request-123", receivedRequestID)
	assert.Equal(t, "test-correlation-456", receivedCorrelationID)
}

func TestClient_RetryOnServerError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL
	cfg.Retry.MaxAttempts = 3

	client, err := New(cfg)
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "/test")
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestClient_NoRetryOnClientError(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL

	client, err := New(cfg)
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "/test")
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, int32(1), atomic.LoadInt32(&attempts))
}

func TestClient_MaxRetriesExceeded(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL
	cfg.Retry.MaxAttempts = 3

	client, err := New(cfg)
	require.NoError(t, err)

	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxRetriesExceeded)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestClient_CircuitBreakerIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL
	cfg.Retry.MaxAttempts = 1
	cfg.Circuit.MaxFailures = 2

	client, err := New(cfg)
	require.NoError(t, err)

	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
	assert.Equal(t, StateClosed, client.CircuitState())

	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
	assert.Equal(t, StateOpen, client.CircuitState())

	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCircuitOpen)
}

func TestClient_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL
	cfg.Timeout = 50 * time.Millisecond
	cfg.Retry.MaxAttempts = 1

	client, err := New(cfg)
	require.NoError(t, err)

	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
}

func TestClient_AuthFunc(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL
	cfg.AuthFunc = func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer test-token")
	}

	client, err := New(cfg)
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "/test")
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, "Bearer test-token", receivedAuth)
}

func TestClient_Post(t *testing.T) {
	var receivedBody string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL

	client, err := New(cfg)
	require.NoError(t, err)

	body := strings.NewReader(`{"name": "test"}`)
	resp, err := client.Post(context.Background(), "/test", body)
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "application/json", receivedContentType)
	assert.Equal(t, `{"name": "test"}`, receivedBody)
}

func TestClient_Put(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL

	client, err := New(cfg)
	require.NoError(t, err)

	body := strings.NewReader(`{"name": "updated"}`)
	resp, err := client.Put(context.Background(), "/test/123", body)
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, http.MethodPut, receivedMethod)
}

func TestClient_Delete(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL

	client, err := New(cfg)
	require.NoError(t, err)

	resp, err := client.Delete(context.Background(), "/test/123")
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, http.MethodDelete, receivedMethod)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestClient_BuildURL(t *testing.T) {
	cfg := defaultConfig()
	cfg.BaseURL = "https://api.example.com"

	client, err := New(cfg)
	require.NoError(t, err)

	assert.Equal(t, "https://api.example.com/users", client.buildURL("/users"))
	assert.Equal(t, "https://api.example.com/users", client.buildURL("users"))

	cfg.BaseURL = "https://api.example.com/"
	client, err = New(cfg)
	require.NoError(t, err)
	assert.Equal(t, "https://api.example.com/users", client.buildURL("/users"))
}

func TestClient_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL

	client, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = client.Get(ctx, "/test")
	require.Error(t, err)
}

func TestCalculateBackoff(t *testing.T) {
	cfg := defaultConfig()
	cfg.Retry.InitialInterval = 100 * time.Millisecond
	cfg.Retry.Multiplier = 2.0
	cfg.Retry.MaxInterval = 1 * time.Second

	client, err := New(cfg)
	require.NoError(t, err)

	backoff0 := client.calculateBackoff(0)
	backoff1 := client.calculateBackoff(1)
	backoff2 := client.calculateBackoff(2)

	assert.InDelta(t, 100*time.Millisecond, backoff0, float64(50*time.Millisecond))
	assert.InDelta(t, 200*time.Millisecond, backoff1, float64(100*time.Millisecond))
	assert.InDelta(t, 400*time.Millisecond, backoff2, float64(200*time.Millisecond))

	backoff10 := client.calculateBackoff(10)
	assert.LessOrEqual(t, backoff10, cfg.Retry.MaxInterval+cfg.Retry.MaxInterval/4)
}

// testNetError is a mock net.Error for testing.
type testNetError struct {
	timeout bool
}

func (e testNetError) Error() string   { return "test net error" }
func (e testNetError) Timeout() bool   { return e.timeout }
func (e testNetError) Temporary() bool { return true }

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{"nil error", nil, false},
		{"context canceled", context.Canceled, false},
		{"context deadline exceeded", context.DeadlineExceeded, false},
		{"net error with timeout", testNetError{timeout: true}, true},
		{"net error without timeout", testNetError{timeout: false}, false},
		{"net op error connection refused", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			assert.Equal(t, tt.retryable, result)
		})
	}
}

func TestClient_CircuitBreakerShortCircuitsWhenOpen(t *testing.T) {
	var calls int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL
	cfg.Retry.MaxAttempts = 1
	cfg.Circuit.MaxFailures = 2

	client, err := New(cfg)
	require.NoError(t, err)

	// Trigger failures to open the circuit
	_, _ = client.Get(context.Background(), "/test")
	_, _ = client.Get(context.Background(), "/test")
	assert.Equal(t, StateOpen, client.CircuitState())

	callsBefore := atomic.LoadInt32(&calls)

	// This request should be short-circuited without hitting the server
	_, err = client.Get(context.Background(), "/test")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrCircuitOpen)
	assert.Equal(t, callsBefore, atomic.LoadInt32(&calls), "request should be short-circuited when circuit is open")
}

func TestClient_AuthFuncCalledOnRetry(t *testing.T) {
	var authCallCount int32
	var requestCount int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := defaultConfig()
	cfg.BaseURL = server.URL
	cfg.Retry.MaxAttempts = 2
	cfg.Retry.InitialInterval = 1 * time.Millisecond
	cfg.AuthFunc = func(r *http.Request) {
		atomic.AddInt32(&authCallCount, 1)
		r.Header.Set("Authorization", "Bearer test-token")
	}

	client, err := New(cfg)
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "/test")
	require.NoError(t, err)
	defer closeBody(t, resp)

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// AuthFunc should be called: once initially + once on retry
	assert.Equal(t, int32(2), atomic.LoadInt32(&authCallCount))
}
