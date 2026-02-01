package acl

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

// testConfig returns a minimal config for testing.
func testConfig(baseURL string) *clients.Config {
	return &clients.Config{
		ServiceName: "test-service",
		BaseURL:     baseURL,
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
}

// --- Error Mapping Tests ---

func TestMapHTTPError_NotFound(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"NOT_FOUND","message":"user not found"}}`)),
	}

	err := MapHTTPError(resp, nil, "user-service", "get user", "user-123")

	require.Error(t, err)
	assert.True(t, domain.IsNotFound(err), "expected NotFoundError")

	// Verify the entityID is set correctly
	var notFoundErr *domain.NotFoundError
	require.True(t, errors.As(err, &notFoundErr))
	assert.Equal(t, "user-123", notFoundErr.ID)
}

func TestMapHTTPError_Conflict(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusConflict,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"CONFLICT","message":"email already exists"}}`)),
	}

	err := MapHTTPError(resp, nil, "user-service", "create user", "")

	require.Error(t, err)
	assert.True(t, domain.IsConflict(err), "expected ConflictError")
}

func TestMapHTTPError_ValidationWithDetails(t *testing.T) {
	body := `{
		"error": {
			"code": "VALIDATION_ERROR",
			"message": "validation failed",
			"details": {
				"email": "invalid format"
			}
		}
	}`
	resp := &http.Response{
		StatusCode: http.StatusBadRequest,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	err := MapHTTPError(resp, nil, "user-service", "create user", "")

	require.Error(t, err)
	assert.True(t, domain.IsValidation(err), "expected ValidationError")

	var validationErr *domain.ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Equal(t, "email", validationErr.Field)
}

func TestMapHTTPError_Forbidden(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusForbidden,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"insufficient permissions"}}`)),
	}

	err := MapHTTPError(resp, nil, "user-service", "delete user", "user-123")

	require.Error(t, err)
	assert.True(t, domain.IsForbidden(err), "expected ForbiddenError")
}

func TestMapHTTPError_Unauthorized(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
	}

	err := MapHTTPError(resp, nil, "user-service", "get user", "user-123")

	require.Error(t, err)
	assert.True(t, domain.IsForbidden(err), "expected ForbiddenError for 401")
}

func TestMapHTTPError_ServerError(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"internal error"}}`)),
	}

	err := MapHTTPError(resp, nil, "user-service", "get user", "user-123")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err), "expected UnavailableError")
}

func TestMapHTTPError_RateLimited(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
	}

	err := MapHTTPError(resp, nil, "user-service", "get user", "")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err), "expected UnavailableError for rate limit")
	assert.Contains(t, err.Error(), "rate limit")
}

func TestMapHTTPError_CircuitOpen(t *testing.T) {
	err := MapHTTPError(nil, clients.ErrCircuitOpen, "user-service", "get user", "user-123")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err))
	assert.Contains(t, err.Error(), "circuit breaker open")
}

func TestMapHTTPError_MaxRetriesExceeded(t *testing.T) {
	err := MapHTTPError(nil, clients.ErrMaxRetriesExceeded, "user-service", "get user", "user-123")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err))
	assert.Contains(t, err.Error(), "max retries exceeded")
}

func TestMapHTTPError_SuccessReturnsNil(t *testing.T) {
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{}`)),
	}

	err := MapHTTPError(resp, nil, "user-service", "get user", "user-123")

	assert.NoError(t, err)
}

func TestMapHTTPError_NilResponse(t *testing.T) {
	err := MapHTTPError(nil, nil, "user-service", "get user", "user-123")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err))
	assert.Contains(t, err.Error(), "no response received")
}

// --- MapExternalCode Tests ---

func TestMapExternalCode(t *testing.T) {
	tests := []struct {
		code     string
		expected func(error) bool
	}{
		{ExternalCodeNotFound, domain.IsNotFound},
		{ExternalCodeConflict, domain.IsConflict},
		{ExternalCodeValidation, domain.IsValidation},
		{ExternalCodeForbidden, domain.IsForbidden},
		{ExternalCodeUnauthorized, domain.IsForbidden},
		{"UNKNOWN_CODE", domain.IsUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			err := MapExternalCode(tt.code, "test message", "test-service", "test op", "entity-123")
			require.Error(t, err)
			assert.True(t, tt.expected(err), "unexpected error type for code %s", tt.code)
		})
	}
}

func TestMapExternalCode_NotFoundWithEntityID(t *testing.T) {
	err := MapExternalCode(ExternalCodeNotFound, "user not found", "user-service", "get user", "user-456")

	require.Error(t, err)
	assert.True(t, domain.IsNotFound(err))

	var notFoundErr *domain.NotFoundError
	require.True(t, errors.As(err, &notFoundErr))
	assert.Equal(t, "user-456", notFoundErr.ID)
}

// --- Translation Tests ---

func TestDecodeResponse_Success(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`{"id":"123","name":"test"}`))

	type testStruct struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	result, err := DecodeResponse[testStruct](body)

	require.NoError(t, err)
	assert.Equal(t, "123", result.ID)
	assert.Equal(t, "test", result.Name)
}

func TestDecodeResponse_InvalidJSON(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`invalid json`))

	type testStruct struct{}

	_, err := DecodeResponse[testStruct](body)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decoding response")
}

func TestDecodeResponse_NilBody(t *testing.T) {
	type testStruct struct{}

	_, err := DecodeResponse[testStruct](nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil")
}

func TestDecodeResponseForService_Success(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`{"id":"123"}`))

	type testStruct struct {
		ID string `json:"id"`
	}

	result, err := DecodeResponseForService[testStruct](body, "test-service")

	require.NoError(t, err)
	assert.Equal(t, "123", result.ID)
}

func TestDecodeResponseForService_Error(t *testing.T) {
	body := io.NopCloser(strings.NewReader(`invalid json`))

	type testStruct struct{}

	_, err := DecodeResponseForService[testStruct](body, "test-service")

	require.Error(t, err)
	assert.True(t, domain.IsUnavailable(err), "expected UnavailableError for decode failure")
}

func TestTranslateSlice_Success(t *testing.T) {
	type External struct{ Value int }
	type Domain struct{ DoubledValue int }

	items := []External{{Value: 1}, {Value: 2}, {Value: 3}}

	translator := func(ext *External) (*Domain, error) {
		return &Domain{DoubledValue: ext.Value * 2}, nil
	}

	result, err := TranslateSlice(items, translator)

	require.NoError(t, err)
	require.Len(t, result, 3)
	assert.Equal(t, 2, result[0].DoubledValue)
	assert.Equal(t, 4, result[1].DoubledValue)
	assert.Equal(t, 6, result[2].DoubledValue)
}

func TestTranslateSlice_Error(t *testing.T) {
	type External struct{ Value int }
	type Domain struct{ Value int }

	items := []External{{Value: 1}, {Value: -1}, {Value: 3}}

	translator := func(ext *External) (*Domain, error) {
		if ext.Value < 0 {
			return nil, domain.NewValidationError("value", "must be positive")
		}

		return &Domain{Value: ext.Value}, nil
	}

	_, err := TranslateSlice(items, translator)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "item 1")
}

func TestTranslateSlice_EmptySlice(t *testing.T) {
	type External struct{ Value int }
	type Domain struct{ Value int }

	items := []External{}

	translator := func(ext *External) (*Domain, error) {
		return &Domain{Value: ext.Value}, nil
	}

	result, err := TranslateSlice(items, translator)

	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestTranslateMap_Success(t *testing.T) {
	type External struct{ Value int }
	type Domain struct{ Value int }

	items := map[string]External{
		"a": {Value: 1},
		"b": {Value: 2},
	}

	translator := func(ext *External) (*Domain, error) {
		return &Domain{Value: ext.Value * 10}, nil
	}

	result, err := TranslateMap(items, translator)

	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, 10, result["a"].Value)
	assert.Equal(t, 20, result["b"].Value)
}

func TestValidateRequired(t *testing.T) {
	err := ValidateRequired("", "name")
	require.Error(t, err)
	assert.True(t, domain.IsValidation(err))

	err = ValidateRequired("value", "name")
	assert.NoError(t, err)
}

func TestValidatePositive(t *testing.T) {
	tests := []struct {
		value    int
		hasError bool
	}{
		{0, true},
		{-1, true},
		{1, false},
		{100, false},
	}

	for _, tt := range tests {
		err := ValidatePositive(tt.value, "count")
		if tt.hasError {
			require.Error(t, err)
			assert.True(t, domain.IsValidation(err))
		} else {
			assert.NoError(t, err)
		}
	}
}

// --- Integration Tests with UserServiceAdapter ---

func TestUserServiceAdapter_GetByID_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users/user-123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "user-123",
			"full_name": "John Doe",
			"email": "john@example.com",
			"status": 1,
			"created_at": "2024-01-15T10:30:00Z"
		}`))
	}))
	defer server.Close()

	cfg := testConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := NewUserServiceAdapter(client)

	user, err := adapter.GetByID(context.Background(), "user-123")

	require.NoError(t, err)
	assert.Equal(t, "user-123", user.ID)
	assert.Equal(t, "John Doe", user.FullName)
	assert.Equal(t, "john@example.com", user.Email)
	assert.True(t, user.IsActive)
}

func TestUserServiceAdapter_GetByID_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"user not found"}}`))
	}))
	defer server.Close()

	cfg := testConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := NewUserServiceAdapter(client)

	_, err = adapter.GetByID(context.Background(), "nonexistent")

	require.Error(t, err)
	assert.True(t, domain.IsNotFound(err))

	// Verify the entityID is set correctly in the error
	var notFoundErr *domain.NotFoundError
	require.True(t, errors.As(err, &notFoundErr))
	assert.Equal(t, "nonexistent", notFoundErr.ID)
}

func TestUserServiceAdapter_GetByID_ValidationError(t *testing.T) {
	cfg := testConfig("http://example.com")
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := NewUserServiceAdapter(client)

	_, err = adapter.GetByID(context.Background(), "")

	require.Error(t, err)
	assert.True(t, domain.IsValidation(err))
}

func TestUserServiceAdapter_GetByEmail_URLEscaping(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the path contains the email correctly decoded.
		// Go's HTTP library normalizes the URL, so we check the decoded Path.
		// The url.PathEscape in the adapter ensures special chars like + and @
		// are transmitted safely even when they have meaning in URLs.
		assert.Equal(t, "/api/v1/users/by-email/test+user@example.com", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id": "user-123",
			"full_name": "Test User",
			"email": "test+user@example.com",
			"status": 1,
			"created_at": "2024-01-15T10:30:00Z"
		}`))
	}))
	defer server.Close()

	cfg := testConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := NewUserServiceAdapter(client)

	user, err := adapter.GetByEmail(context.Background(), "test+user@example.com")

	require.NoError(t, err)
	assert.Equal(t, "test+user@example.com", user.Email)
}

func TestUserServiceAdapter_GetByEmail_SpecialCharsInPath(t *testing.T) {
	// Test with characters that must be escaped for valid URL paths
	testCases := []struct {
		name          string
		email         string
		expectedEmail string
	}{
		{"percent", "user%test@example.com", "user%test@example.com"},
		{"slash", "user/test@example.com", "user/test@example.com"},
		{"space", "user test@example.com", "user test@example.com"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var receivedPath string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{
					"id": "user-123",
					"full_name": "Test User",
					"email": "` + tc.expectedEmail + `",
					"status": 1,
					"created_at": "2024-01-15T10:30:00Z"
				}`))
			}))
			defer server.Close()

			cfg := testConfig(server.URL)
			client, err := clients.New(cfg)
			require.NoError(t, err)

			adapter := NewUserServiceAdapter(client)
			user, err := adapter.GetByEmail(context.Background(), tc.email)

			require.NoError(t, err)
			assert.Equal(t, tc.expectedEmail, user.Email)
			// Verify the path correctly transmitted the email
			assert.Contains(t, receivedPath, "/api/v1/users/by-email/")
		})
	}
}

func TestUserServiceAdapter_TranslateUser_StatusMapping(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		expectedActive bool
	}{
		{"active", externalStatusActive, true},
		{"inactive", externalStatusInactive, false},
		{"suspended", externalStatusSuspended, false},
		{"unknown", 99, false},
	}

	adapter := &UserServiceAdapter{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ext := &externalUserResponse{
				ID:        "test",
				FullName:  "Test User",
				Status:    tt.status,
				CreatedAt: "2024-01-01T00:00:00Z",
			}

			user, err := adapter.translateUser(ext)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedActive, user.IsActive)
		})
	}
}

func TestUserServiceAdapter_TranslateUser_MissingRequiredFields(t *testing.T) {
	adapter := &UserServiceAdapter{}

	// Missing ID
	_, err := adapter.translateUser(&externalUserResponse{
		FullName: "Test User",
	})
	require.Error(t, err)
	assert.True(t, domain.IsValidation(err))

	// Missing FullName
	_, err = adapter.translateUser(&externalUserResponse{
		ID: "123",
	})
	require.Error(t, err)
	assert.True(t, domain.IsValidation(err))
}

func TestUserServiceAdapter_TranslateUser_InvalidTimestamp(t *testing.T) {
	adapter := &UserServiceAdapter{}

	ext := &externalUserResponse{
		ID:        "test",
		FullName:  "Test User",
		Status:    1,
		CreatedAt: "invalid-timestamp",
	}

	user, err := adapter.translateUser(ext)

	// Should not error, just use zero time
	require.NoError(t, err)
	assert.True(t, user.CreatedAt.IsZero())
}

func TestUserServiceAdapter_List_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/users", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("page"))
		assert.Equal(t, "10", r.URL.Query().Get("page_size"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"users": [
				{"id": "1", "full_name": "User 1", "email": "u1@test.com", "status": 1, "created_at": "2024-01-01T00:00:00Z"},
				{"id": "2", "full_name": "User 2", "email": "u2@test.com", "status": 2, "created_at": "2024-01-01T00:00:00Z"}
			],
			"total_count": 50,
			"page": 1,
			"page_size": 10
		}`))
	}))
	defer server.Close()

	cfg := testConfig(server.URL)
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := NewUserServiceAdapter(client)

	users, total, err := adapter.List(context.Background(), 1, 10)

	require.NoError(t, err)
	assert.Len(t, users, 2)
	assert.Equal(t, 50, total)
	assert.True(t, users[0].IsActive)
	assert.False(t, users[1].IsActive)
}

func TestUserServiceAdapter_List_ValidationError(t *testing.T) {
	cfg := testConfig("http://example.com")
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := NewUserServiceAdapter(client)

	// Invalid page
	_, _, err = adapter.List(context.Background(), 0, 10)
	require.Error(t, err)
	assert.True(t, domain.IsValidation(err))

	// Invalid page size
	_, _, err = adapter.List(context.Background(), 1, 0)
	require.Error(t, err)
	assert.True(t, domain.IsValidation(err))
}

// --- ParseErrorResponse Tests ---

func TestParseErrorResponse_NestedFormat(t *testing.T) {
	body := strings.NewReader(`{"error":{"code":"NOT_FOUND","message":"not found"}}`)

	resp := ParseErrorResponse(body)

	require.NotNil(t, resp)
	assert.Equal(t, "NOT_FOUND", resp.GetCode())
	assert.Equal(t, "not found", resp.GetMessage())
}

func TestParseErrorResponse_TopLevelFormat(t *testing.T) {
	body := strings.NewReader(`{"code":"CONFLICT","message":"already exists"}`)

	resp := ParseErrorResponse(body)

	require.NotNil(t, resp)
	assert.Equal(t, "CONFLICT", resp.GetCode())
	assert.Equal(t, "already exists", resp.GetMessage())
}

func TestParseErrorResponse_InvalidJSON(t *testing.T) {
	body := strings.NewReader(`not json`)

	resp := ParseErrorResponse(body)

	assert.Nil(t, resp)
}

func TestParseErrorResponse_EmptyBody(t *testing.T) {
	body := strings.NewReader(`{}`)

	resp := ParseErrorResponse(body)

	assert.Nil(t, resp) // No meaningful data
}

func TestParseErrorResponse_NilBody(t *testing.T) {
	resp := ParseErrorResponse(nil)

	assert.Nil(t, resp)
}

// --- BaseAdapter Tests ---

func TestBaseAdapter_ServiceName(t *testing.T) {
	cfg := testConfig("http://example.com")
	client, err := clients.New(cfg)
	require.NoError(t, err)

	adapter := NewBaseAdapter(client, "my-service")

	assert.Equal(t, "my-service", adapter.ServiceName())
	assert.NotNil(t, adapter.Client())
}
