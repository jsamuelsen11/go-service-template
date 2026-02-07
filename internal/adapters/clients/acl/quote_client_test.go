package acl

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

// setupQuoteClient creates a QuoteClient with a test HTTP server.
func setupQuoteClient(t *testing.T, handler http.HandlerFunc) *QuoteClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client, err := clients.New(&clients.Config{
		ServiceName: "test-quote",
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     1,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   10,
			Timeout:       30 * time.Second,
			HalfOpenLimit: 3,
		},
		Transport: config.TransportConfig{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	})
	require.NoError(t, err)

	return NewQuoteClient(QuoteClientConfig{
		Client: client,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// TestNewQuoteClient_PanicsWithoutClient verifies that NewQuoteClient panics when Client is nil.
func TestNewQuoteClient_PanicsWithoutClient(t *testing.T) {
	assert.Panics(t, func() {
		NewQuoteClient(QuoteClientConfig{
			Client: nil,
			Logger: slog.Default(),
		})
	})
}

// TestNewQuoteClient_DefaultsLogger verifies that nil logger uses default logger.
func TestNewQuoteClient_DefaultsLogger(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	client, err := clients.New(&clients.Config{
		ServiceName: "test-quote",
		BaseURL:     server.URL,
		Timeout:     5 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     1,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     100 * time.Millisecond,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   10,
			Timeout:       30 * time.Second,
			HalfOpenLimit: 3,
		},
		Transport: config.TransportConfig{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	})
	require.NoError(t, err)

	quoteClient := NewQuoteClient(QuoteClientConfig{
		Client: client,
		Logger: nil,
	})

	require.NotNil(t, quoteClient)
	assert.NotNil(t, quoteClient.logger)
}

// TestQuoteClient_Name verifies that Name returns the expected service name.
func TestQuoteClient_Name(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	client := setupQuoteClient(t, handler)

	name := client.Name()

	assert.Equal(t, "quote-service", name)
}

// TestGetRandomQuote_Success verifies that a random quote can be fetched successfully.
func TestGetRandomQuote_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/random", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]any{
			"_id":     "abc123",
			"content": "Be the change you wish to see in the world",
			"author":  "Mahatma Gandhi",
			"tags":    []string{"inspirational", "change"},
		})
		if !assert.NoError(t, err) {
			return
		}
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	quote, err := client.GetRandomQuote(ctx)

	require.NoError(t, err)
	require.NotNil(t, quote)
	assert.Equal(t, "abc123", quote.ID)
	assert.Equal(t, "Be the change you wish to see in the world", quote.Content)
	assert.Equal(t, "Mahatma Gandhi", quote.Author)
	assert.Len(t, quote.Tags, 2)
	assert.Contains(t, quote.Tags, "inspirational")
	assert.Contains(t, quote.Tags, "change")
}

// TestGetRandomQuote_ServerError verifies that 500 error returns UnavailableError.
func TestGetRandomQuote_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	quote, err := client.GetRandomQuote(ctx)

	require.Error(t, err)
	assert.Nil(t, quote)
	assert.True(t, domain.IsUnavailable(err))
	assert.Contains(t, err.Error(), "quote-service")
}

// TestGetRandomQuote_InvalidJSON verifies that invalid JSON returns an error.
func TestGetRandomQuote_InvalidJSON(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("invalid json {"))
		if !assert.NoError(t, err) {
			return
		}
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	quote, err := client.GetRandomQuote(ctx)

	require.Error(t, err)
	assert.Nil(t, quote)
	assert.Contains(t, err.Error(), "decoding quote response")
}

// TestGetQuoteByID_Success verifies that a specific quote can be fetched by ID.
func TestGetQuoteByID_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/quotes/xyz789", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]any{
			"_id":     "xyz789",
			"content": "The only way to do great work is to love what you do",
			"author":  "Steve Jobs",
			"tags":    []string{"work", "passion"},
		})
		if !assert.NoError(t, err) {
			return
		}
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	quote, err := client.GetQuoteByID(ctx, "xyz789")

	require.NoError(t, err)
	require.NotNil(t, quote)
	assert.Equal(t, "xyz789", quote.ID)
	assert.Equal(t, "The only way to do great work is to love what you do", quote.Content)
	assert.Equal(t, "Steve Jobs", quote.Author)
	assert.Len(t, quote.Tags, 2)
}

// TestGetQuoteByID_NotFound verifies that 404 returns NotFoundError.
func TestGetQuoteByID_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	quote, err := client.GetQuoteByID(ctx, "nonexistent")

	require.Error(t, err)
	assert.Nil(t, quote)
	assert.True(t, domain.IsNotFound(err))
	assert.Contains(t, err.Error(), "quote")
	assert.Contains(t, err.Error(), "nonexistent")
}

// TestGetQuoteByID_ServerError verifies that 503 returns UnavailableError.
func TestGetQuoteByID_ServerError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	quote, err := client.GetQuoteByID(ctx, "test-id")

	require.Error(t, err)
	assert.Nil(t, quote)
	assert.True(t, domain.IsUnavailable(err))
	assert.Contains(t, err.Error(), "quote-service")
}

// TestQuoteClient_Check_Success verifies that health check passes on successful request.
func TestQuoteClient_Check_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/random", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(map[string]any{
			"_id":     "health-check",
			"content": "Health check quote",
			"author":  "System",
			"tags":    []string{"health"},
		})
		if !assert.NoError(t, err) {
			return
		}
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	err := client.Check(ctx)

	assert.NoError(t, err)
}

// TestQuoteClient_Check_Failure verifies that health check fails on non-200 response.
func TestQuoteClient_Check_Failure(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	err := client.Check(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "503")
}

// TestGetRandomQuote_ServiceUnavailable verifies that 503 error returns UnavailableError.
func TestGetRandomQuote_ServiceUnavailable(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	client := setupQuoteClient(t, handler)
	ctx := context.Background()

	quote, err := client.GetRandomQuote(ctx)

	require.Error(t, err)
	assert.Nil(t, quote)
	assert.True(t, domain.IsUnavailable(err))
	assert.Contains(t, err.Error(), "quote-service")
	assert.Contains(t, err.Error(), "503")
}
