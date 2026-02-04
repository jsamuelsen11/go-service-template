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
	"github.com/jsamuelsen/go-service-template/internal/adapters/clients/acl"
	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

// testAdapterConfig returns a config suitable for adapter integration testing.
func testAdapterConfig(baseURL string) *clients.Config {
	return &clients.Config{
		ServiceName: "user-service",
		BaseURL:     baseURL,
		Timeout:     5 * time.Second,
		Retry: config.RetryConfig{
			MaxAttempts:     2,
			InitialInterval: 10 * time.Millisecond,
			MaxInterval:     50 * time.Millisecond,
			Multiplier:      2.0,
		},
		Circuit: config.CircuitBreakerConfig{
			MaxFailures:   3,
			Timeout:       100 * time.Millisecond,
			HalfOpenLimit: 2,
		},
	}
}

// TestUserServiceAdapter_GetByID_Integration verifies the full flow
// of fetching a user by ID through the adapter.
func TestUserServiceAdapter_GetByID_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify correct path format
		assert.Equal(t, "/api/v1/users/user-integration-123", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "user-integration-123",
			"full_name": "Integration Test User",
			"email": "integration@test.com",
			"status": 1,
			"created_at": "2024-06-15T14:30:00Z"
		}`))
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	user, err := adapter.GetByID(context.Background(), "user-integration-123")

	require.NoError(t, err)
	assert.Equal(t, "user-integration-123", user.ID)
	assert.Equal(t, "Integration Test User", user.FullName)
	assert.Equal(t, "integration@test.com", user.Email)
	assert.True(t, user.IsActive)
	assert.False(t, user.CreatedAt.IsZero())
}

// TestUserServiceAdapter_ErrorMapping_NotFound verifies that 404 responses
// are correctly mapped to domain NotFoundError.
func TestUserServiceAdapter_ErrorMapping_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": "NOT_FOUND",
				"message": "user not found"
			}
		}`))
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	_, err = adapter.GetByID(context.Background(), "nonexistent-user")

	require.Error(t, err)
	assert.True(t, domain.IsNotFound(err), "expected NotFoundError")

	// Verify entity ID is preserved
	var notFoundErr *domain.NotFoundError
	require.ErrorAs(t, err, &notFoundErr)
	assert.Equal(t, "nonexistent-user", notFoundErr.ID)
}

// TestUserServiceAdapter_ErrorMapping_Conflict verifies that 409 responses
// are correctly mapped to domain ConflictError.
func TestUserServiceAdapter_ErrorMapping_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": "CONFLICT",
				"message": "email already exists"
			}
		}`))
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	// Use GetByEmail to trigger a request that would return conflict
	_, err = adapter.GetByEmail(context.Background(), "duplicate@test.com")

	require.Error(t, err)
	assert.True(t, domain.IsConflict(err), "expected ConflictError")
}

// TestUserServiceAdapter_ErrorMapping_Validation verifies that 400 responses
// with validation details are correctly mapped to domain ValidationError.
func TestUserServiceAdapter_ErrorMapping_Validation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error": {
				"code": "VALIDATION_ERROR",
				"message": "validation failed",
				"details": {
					"email": "invalid email format"
				}
			}
		}`))
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	_, err = adapter.GetByEmail(context.Background(), "invalid-email")

	require.Error(t, err)
	assert.True(t, domain.IsValidation(err), "expected ValidationError")
}

// TestUserServiceAdapter_ErrorMapping_ServiceUnavailable verifies that 5xx responses
// are correctly mapped to domain UnavailableError.
func TestUserServiceAdapter_ErrorMapping_ServiceUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`internal server error`))
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	cfg.Retry.MaxAttempts = 1 // Fail fast for this test

	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	_, err = adapter.GetByID(context.Background(), "any-user")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err), "expected UnavailableError")
}

// TestUserServiceAdapter_ErrorMapping_CircuitOpen verifies that circuit breaker
// open state is correctly mapped to domain UnavailableError.
func TestUserServiceAdapter_ErrorMapping_CircuitOpen(t *testing.T) {
	var calls int32 = 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	cfg.Retry.MaxAttempts = 1
	cfg.Circuit.MaxFailures = 2

	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	// Trip the circuit breaker
	_, _ = adapter.GetByID(context.Background(), "user1")
	_, _ = adapter.GetByID(context.Background(), "user2")

	// This call should fail fast with circuit open
	callsBefore := calls
	_, err = adapter.GetByID(context.Background(), "user3")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err), "expected UnavailableError")
	assert.Contains(t, err.Error(), "circuit breaker open")
	assert.Equal(t, callsBefore, calls, "no server call when circuit is open")
}

// TestUserServiceAdapter_List_Integration verifies the full flow
// of listing users with pagination through the adapter.
func TestUserServiceAdapter_List_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify pagination parameters
		assert.Equal(t, "/api/v1/users", r.URL.Path)
		assert.Equal(t, "2", r.URL.Query().Get("page"))
		assert.Equal(t, "25", r.URL.Query().Get("page_size"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"users": [
				{"id": "user-1", "full_name": "User One", "email": "one@test.com", "status": 1, "created_at": "2024-01-01T00:00:00Z"},
				{"id": "user-2", "full_name": "User Two", "email": "two@test.com", "status": 2, "created_at": "2024-01-02T00:00:00Z"},
				{"id": "user-3", "full_name": "User Three", "email": "three@test.com", "status": 1, "created_at": "2024-01-03T00:00:00Z"}
			],
			"total_count": 100,
			"page": 2,
			"page_size": 25
		}`))
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	users, total, err := adapter.List(context.Background(), 2, 25)

	require.NoError(t, err)
	assert.Len(t, users, 3)
	assert.Equal(t, 100, total)

	// Verify status mapping
	assert.True(t, users[0].IsActive)  // status 1 = active
	assert.False(t, users[1].IsActive) // status 2 = inactive
	assert.True(t, users[2].IsActive)  // status 1 = active
}

// TestUserServiceAdapter_InputValidation verifies that invalid inputs
// are rejected before making network calls.
func TestUserServiceAdapter_InputValidation(t *testing.T) {
	// Server that fails if called - we shouldn't reach it
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for invalid input")
	}))
	defer server.Close()

	cfg := testAdapterConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := acl.NewUserServiceAdapter(client)

	tests := []struct {
		name   string
		action func() error
	}{
		{
			name: "GetByID with empty ID",
			action: func() error {
				_, err := adapter.GetByID(context.Background(), "")
				return err
			},
		},
		{
			name: "GetByEmail with empty email",
			action: func() error {
				_, err := adapter.GetByEmail(context.Background(), "")
				return err
			},
		},
		{
			name: "List with invalid page",
			action: func() error {
				_, _, err := adapter.List(context.Background(), 0, 10)
				return err
			},
		},
		{
			name: "List with invalid page size",
			action: func() error {
				_, _, err := adapter.List(context.Background(), 1, 0)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action()
			require.Error(t, err)
			assert.True(t, domain.IsValidation(err), "expected ValidationError")
		})
	}
}

// TestUserServiceAdapter_TranslationAccuracy verifies that external API
// response fields are accurately translated to domain model fields.
func TestUserServiceAdapter_TranslationAccuracy(t *testing.T) {
	// Test various status values
	testCases := []struct {
		name           string
		status         int
		expectedActive bool
	}{
		{"active user", 1, true},
		{"inactive user", 2, false},
		{"suspended user", 3, false},
		{"unknown status", 99, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{
					"id": "user-status-test",
					"full_name": "Status Test User",
					"email": "status@test.com",
					"status": ` + string(rune('0'+tc.status)) + `,
					"created_at": "2024-01-15T10:30:00Z"
				}`))
			}))
			defer server.Close()

			cfg := testAdapterConfig(server.URL)
			client, err := clients.New(cfg)
			require.NoError(t, err)

			adapter := acl.NewUserServiceAdapter(client)
			user, err := adapter.GetByID(context.Background(), "user-status-test")

			require.NoError(t, err)
			assert.Equal(t, tc.expectedActive, user.IsActive, "IsActive mismatch for status %d", tc.status)
		})
	}
}
