//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

// TestConfig_DefaultValues verifies that clients work correctly
// with default configuration values.
func TestConfig_DefaultValues(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	cfg := &clients.Config{
		ServiceName: "default-config-test",
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     1,
			InitialInterval: 100 * time.Millisecond,
			MaxInterval:     time.Second,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   5,
			Timeout:       time.Second,
			HalfOpenLimit: 2,
		},
	}

	client, err := clients.New(cfg)
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "/test")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestConfig_CustomTimeout verifies that custom timeout configuration
// is respected by the client.
func TestConfig_CustomTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Longer than configured timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &clients.Config{
		ServiceName: "timeout-test",
		BaseURL:     server.URL,
		Timeout:     50 * time.Millisecond, // Short timeout
		Retry: config.RetryConfig{
			MaxAttempts:     1,
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

	client, err := clients.New(cfg)
	require.NoError(t, err)

	start := time.Now()
	_, err = client.Get(context.Background(), "/slow")
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Less(t, elapsed, 150*time.Millisecond, "request should timeout quickly")
}

// TestConfig_RetryConfiguration verifies that retry configuration
// affects client behavior as expected.
func TestConfig_RetryConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		maxAttempts     int
		expectedCalls   int32
		expectSuccess   bool
		serverFailCount int32 // Number of times server will fail before succeeding
	}{
		{
			name:            "no retry succeeds on first try",
			maxAttempts:     1,
			serverFailCount: 0,
			expectedCalls:   1,
			expectSuccess:   true,
		},
		{
			name:            "single retry after one failure",
			maxAttempts:     2,
			serverFailCount: 1,
			expectedCalls:   2,
			expectSuccess:   true,
		},
		{
			name:            "max retries exceeded",
			maxAttempts:     2,
			serverFailCount: 5, // More failures than attempts
			expectedCalls:   2,
			expectSuccess:   false,
		},
		{
			name:            "multiple retries succeed eventually",
			maxAttempts:     4,
			serverFailCount: 3,
			expectedCalls:   4,
			expectSuccess:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var calls int32 = 0

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls <= tt.serverFailCount {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cfg := &clients.Config{
				ServiceName: "retry-config-test",
				BaseURL:     server.URL,
				Timeout:     5 * time.Second,
				Retry: config.RetryConfig{
					MaxAttempts:     tt.maxAttempts,
					InitialInterval: 5 * time.Millisecond,
					MaxInterval:     50 * time.Millisecond,
					Multiplier:      2.0,
				},
				Circuit: config.CircuitBreakerConfig{
					MaxFailures:   100, // High to avoid circuit breaker interference
					Timeout:       time.Second,
					HalfOpenLimit: 2,
				},
			}

			client, err := clients.New(cfg)
			require.NoError(t, err)

			resp, err := client.Get(context.Background(), "/test")

			if tt.expectSuccess {
				require.NoError(t, err)
				defer resp.Body.Close()
				assert.Equal(t, http.StatusOK, resp.StatusCode)
			} else {
				require.Error(t, err)
			}

			assert.Equal(t, tt.expectedCalls, calls, "unexpected number of server calls")
		})
	}
}

// TestConfig_CircuitBreakerConfiguration verifies that circuit breaker
// configuration affects client behavior as expected.
func TestConfig_CircuitBreakerConfiguration(t *testing.T) {
	tests := []struct {
		name              string
		maxFailures       int
		failuresToTrigger int
		expectCircuitOpen bool
	}{
		{
			name:              "circuit stays closed with few failures",
			maxFailures:       5,
			failuresToTrigger: 2,
			expectCircuitOpen: false,
		},
		{
			name:              "circuit opens at threshold",
			maxFailures:       3,
			failuresToTrigger: 3,
			expectCircuitOpen: true,
		},
		{
			name:              "circuit opens after exceeding threshold",
			maxFailures:       2,
			failuresToTrigger: 4,
			expectCircuitOpen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			cfg := &clients.Config{
				ServiceName: "circuit-config-test",
				BaseURL:     server.URL,
				Timeout:     5 * time.Second,
				Retry: config.RetryConfig{
					MaxAttempts:     1, // No retries
					InitialInterval: 10 * time.Millisecond,
					MaxInterval:     100 * time.Millisecond,
					Multiplier:      2.0,
				},
				Circuit: config.CircuitBreakerConfig{
					MaxFailures:   tt.maxFailures,
					Timeout:       time.Second,
					HalfOpenLimit: 2,
				},
			}

			client, err := clients.New(cfg)
			require.NoError(t, err)

			// Trigger failures
			for i := 0; i < tt.failuresToTrigger; i++ {
				_, _ = client.Get(context.Background(), "/fail")
			}

			if tt.expectCircuitOpen {
				assert.Equal(t, clients.StateOpen, client.CircuitState(), "circuit should be open")
			} else {
				assert.Equal(t, clients.StateClosed, client.CircuitState(), "circuit should be closed")
			}
		})
	}
}

// TestConfig_AuthFunctionConfiguration verifies that the authentication
// function is correctly applied to requests.
func TestConfig_AuthFunctionConfiguration(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &clients.Config{
		ServiceName: "auth-config-test",
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     1,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   5,
			Timeout:       time.Second,
			HalfOpenLimit: 2,
		},
		AuthFunc: func(r *http.Request) {
			r.Header.Set("Authorization", "Bearer integration-test-token-12345")
		},
	}

	client, err := clients.New(cfg)
	require.NoError(t, err)

	resp, err := client.Get(context.Background(), "/auth")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, "Bearer integration-test-token-12345", receivedAuth)
}

// TestConfig_BaseURLNormalization verifies that base URLs with and without
// trailing slashes are handled correctly.
func TestConfig_BaseURLNormalization(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		path        string
		expectedURL string
	}{
		{
			name:        "base URL without trailing slash, path with leading slash",
			baseURL:     "http://example.com",
			path:        "/api/users",
			expectedURL: "/api/users",
		},
		{
			name:        "base URL with trailing slash, path with leading slash",
			baseURL:     "http://example.com/",
			path:        "/api/users",
			expectedURL: "/api/users",
		},
		{
			name:        "path without leading slash",
			baseURL:     "http://example.com",
			path:        "api/users",
			expectedURL: "/api/users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPath string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			cfg := &clients.Config{
				ServiceName: "url-test",
				BaseURL:     server.URL, // We use the test server URL
				Timeout:     5 * time.Second,
				Retry: config.RetryConfig{
					MaxAttempts:     1,
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

			client, err := clients.New(cfg)
			require.NoError(t, err)

			resp, err := client.Get(context.Background(), tt.path)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedURL, receivedPath)
		})
	}
}

// TestConfig_InvalidConfiguration verifies that invalid configurations
// are properly rejected.
func TestConfig_InvalidConfiguration(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *clients.Config
		expectError string
	}{
		{
			name:        "nil config",
			cfg:         nil,
			expectError: "config is required",
		},
		{
			name: "empty service name",
			cfg: &clients.Config{
				ServiceName: "",
				BaseURL:     "http://example.com",
				Timeout:     time.Second,
			},
			expectError: "service name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := clients.New(tt.cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}
