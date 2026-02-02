# Writing Unit Tests

This guide covers unit testing patterns including table-driven tests, mocking, and test helpers.

## Overview

The template uses:

- **testify** for assertions (`assert`, `require`)
- **mockery** for auto-generated mocks
- **httptest** for HTTP handler testing

---

## Table-Driven Tests

The standard Go pattern for testing multiple scenarios:

```go
func TestValidateEmail(t *testing.T) {
    tests := []struct {
        name    string
        email   string
        wantErr bool
    }{
        {
            name:    "valid email",
            email:   "user@example.com",
            wantErr: false,
        },
        {
            name:    "missing @",
            email:   "userexample.com",
            wantErr: true,
        },
        {
            name:    "empty string",
            email:   "",
            wantErr: true,
        },
        {
            name:    "multiple @",
            email:   "user@@example.com",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateEmail(tt.email)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

---

## Using Testify

### Assertions vs Requirements

```go
import (
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
    // assert: Test continues on failure (soft assertion)
    assert.Equal(t, expected, actual)
    assert.NoError(t, err)
    assert.True(t, condition)

    // require: Test stops on failure (hard assertion)
    require.NoError(t, err)  // Use when subsequent code depends on success
    require.NotNil(t, result)

    // Example: require for setup, assert for verification
    result, err := service.GetUser(ctx, "user-123")
    require.NoError(t, err)           // Stop if error
    require.NotNil(t, result)         // Stop if nil

    assert.Equal(t, "user-123", result.ID)  // Continue checking
    assert.Equal(t, "test@example.com", result.Email)
}
```

### Common Assertions

```go
// Equality
assert.Equal(t, expected, actual)
assert.NotEqual(t, expected, actual)

// Nil checks
assert.Nil(t, value)
assert.NotNil(t, value)

// Boolean
assert.True(t, condition)
assert.False(t, condition)

// Error checking
assert.NoError(t, err)
assert.Error(t, err)
assert.ErrorIs(t, err, domain.ErrNotFound)
assert.ErrorContains(t, err, "not found")

// Collections
assert.Len(t, slice, 3)
assert.Empty(t, slice)
assert.Contains(t, slice, element)

// Strings
assert.Contains(t, str, "substring")
assert.HasPrefix(t, str, "prefix")
```

---

## Mocking with Mockery

### Generate Mocks

Mocks are auto-generated from interfaces defined in `.mockery.yaml`:

```yaml
# .mockery.yaml
with-expecter: true
mockname: "Mock{{.InterfaceName}}"
packages:
  github.com/jsamuelsen/go-service-template/internal/ports:
    interfaces:
      HealthChecker:
      QuoteClient:
      FeatureFlags:
```

Run `task generate` to create mocks in `internal/ports/mocks/`.

### Using Mocks

```go
import (
    "testing"

    "github.com/stretchr/testify/mock"

    "github.com/jsamuelsen/go-service-template/internal/app"
    "github.com/jsamuelsen/go-service-template/internal/domain"
    "github.com/jsamuelsen/go-service-template/internal/ports/mocks"
)

func TestQuoteService_GetRandomQuote(t *testing.T) {
    // Create mock
    mockClient := mocks.NewMockQuoteClient(t)

    // Setup expectation using EXPECT() (type-safe)
    expectedQuote := &domain.Quote{
        ID:      "quote-123",
        Content: "Test quote",
        Author:  "Test Author",
    }

    mockClient.EXPECT().
        GetRandomQuote(mock.Anything).  // mock.Anything matches any argument
        Return(expectedQuote, nil).
        Once()  // Expect exactly one call

    // Create service with mock
    service := app.NewQuoteService(app.QuoteServiceConfig{
        QuoteClient: mockClient,
        Logger:      slog.Default(),
    })

    // Execute
    result, err := service.GetRandomQuote(context.Background())

    // Assert
    require.NoError(t, err)
    assert.Equal(t, "quote-123", result.ID)
    assert.Equal(t, "Test quote", result.Content)
}
```

### Mock Expectations

```go
// Match any argument
mockClient.EXPECT().
    GetQuote(mock.Anything, mock.Anything).
    Return(quote, nil)

// Match specific argument
mockClient.EXPECT().
    GetQuote(mock.Anything, "quote-123").
    Return(quote, nil)

// Custom argument matching
mockClient.EXPECT().
    GetQuote(mock.Anything, mock.MatchedBy(func(id string) bool {
        return strings.HasPrefix(id, "quote-")
    })).
    Return(quote, nil)

// Return error
mockClient.EXPECT().
    GetQuote(mock.Anything, "invalid").
    Return(nil, domain.ErrNotFound)

// Call count
mockClient.EXPECT().Method(...).Once()       // Exactly once
mockClient.EXPECT().Method(...).Times(3)     // Exactly 3 times
mockClient.EXPECT().Method(...).Maybe()      // Zero or more times
```

---

## Testing HTTP Handlers

### Basic Handler Test

```go
import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestQuoteHandler_GetRandomQuote(t *testing.T) {
    // Setup
    gin.SetMode(gin.TestMode)

    mockService := mocks.NewMockQuoteService(t)
    mockService.EXPECT().
        GetRandomQuote(mock.Anything).
        Return(&domain.Quote{ID: "123", Content: "Test"}, nil)

    handler := handlers.NewQuoteHandler(mockService)

    // Create test request
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/random", nil)

    // Create Gin context
    c, _ := gin.CreateTestContext(w)
    c.Request = req

    // Execute
    handler.GetRandomQuote(c)

    // Assert
    assert.Equal(t, http.StatusOK, w.Code)
    assert.Contains(t, w.Body.String(), `"id":"123"`)
}
```

### Handler Test with Router

For tests that need full routing:

```go
func TestQuoteHandler_Integration(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // Setup mock
    mockService := mocks.NewMockQuoteService(t)
    mockService.EXPECT().
        GetQuoteByID(mock.Anything, "quote-123").
        Return(&domain.Quote{ID: "quote-123"}, nil)

    // Setup router
    router := gin.New()
    handler := handlers.NewQuoteHandler(mockService)
    handler.RegisterQuoteRoutes(router.Group("/api/v1"))

    // Create request
    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/quote-123", nil)

    // Execute
    router.ServeHTTP(w, req)

    // Assert
    assert.Equal(t, http.StatusOK, w.Code)
}
```

### Testing Error Responses

```go
func TestQuoteHandler_NotFound(t *testing.T) {
    gin.SetMode(gin.TestMode)

    mockService := mocks.NewMockQuoteService(t)
    mockService.EXPECT().
        GetQuoteByID(mock.Anything, "invalid").
        Return(nil, domain.ErrNotFound)

    handler := handlers.NewQuoteHandler(mockService)

    w := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/api/v1/quotes/invalid", nil)

    c, _ := gin.CreateTestContext(w)
    c.Request = req
    c.Params = gin.Params{{Key: "id", Value: "invalid"}}

    handler.GetQuoteByID(c)

    assert.Equal(t, http.StatusNotFound, w.Code)
    assert.Contains(t, w.Body.String(), "NOT_FOUND")
}
```

---

## Test Helpers

### Configuration Fixtures

```go
// From internal/adapters/clients/client_test.go
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
```

### HTTP Test Server

```go
func TestClient_Success(t *testing.T) {
    // Create test server
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Assert request
        assert.Equal(t, "/api/v1/users/123", r.URL.Path)
        assert.Equal(t, "Bearer token", r.Header.Get("Authorization"))

        // Return mock response
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        w.Write([]byte(`{"id":"123","name":"Test"}`))
    }))
    defer server.Close()

    // Create client pointing to test server
    client := NewClient(server.URL)

    // Execute and assert
    result, err := client.GetUser(context.Background(), "123")
    require.NoError(t, err)
    assert.Equal(t, "123", result.ID)
}
```

### Helper Functions

```go
// Close response body safely
func closeBody(t *testing.T, resp *http.Response) {
    t.Helper()
    if err := resp.Body.Close(); err != nil {
        t.Errorf("failed to close response body: %v", err)
    }
}

// Create context with timeout
func testContext(t *testing.T) context.Context {
    t.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    t.Cleanup(cancel)
    return ctx
}
```

---

## Running Tests

```bash
# Run all unit tests
task test

# Run specific package
go test ./internal/app/...

# Run specific test
go test ./internal/app/... -run TestQuoteService

# Verbose output
go test -v ./...

# With coverage
task coverage
```

---

## Checklist

- [ ] Use table-driven tests for multiple scenarios
- [ ] Use `require` for setup, `assert` for verification
- [ ] Generate mocks with `task generate`
- [ ] Use `mock.Anything` for context arguments
- [ ] Set `gin.SetMode(gin.TestMode)` in handler tests
- [ ] Clean up resources with `t.Cleanup()` or `defer`
- [ ] Test both success and error paths

---

## Related Documentation

- [Writing Integration Tests](./writing-integration-tests.md)
- [Writing Benchmark Tests](./writing-benchmark-tests.md)
