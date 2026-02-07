package domain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSentinelErrors_AreDistinct(t *testing.T) {
	sentinels := []error{
		ErrNotFound,
		ErrConflict,
		ErrValidation,
		ErrForbidden,
		ErrUnavailable,
	}

	for i, a := range sentinels {
		for j, b := range sentinels {
			if i != j {
				assert.NotErrorIs(t, a, b,
					"sentinels should be distinct: %v vs %v", a, b)
			}
		}
	}
}

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		entity      string
		id          string
		expectedMsg string
	}{
		{
			name:        "with entity and ID",
			entity:      "user",
			id:          "123",
			expectedMsg: `user with id "123" not found`,
		},
		{
			name:        "with entity only",
			entity:      "order",
			id:          "",
			expectedMsg: "order not found",
		},
		{
			name:        "empty entity with ID",
			entity:      "",
			id:          "abc",
			expectedMsg: ` with id "abc" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewNotFoundError(tt.entity, tt.id)

			assert.Equal(t, tt.expectedMsg, err.Error())
			require.ErrorIs(t, err, ErrNotFound)

			var notFound *NotFoundError
			require.ErrorAs(t, err, &notFound)
			assert.Equal(t, tt.entity, notFound.Entity)
			assert.Equal(t, tt.id, notFound.ID)
		})
	}
}

func TestNotFoundError_Unwrap(t *testing.T) {
	err := NewNotFoundError("user", "123")

	var notFound *NotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, ErrNotFound, notFound.Unwrap())
}

func TestConflictError(t *testing.T) {
	tests := []struct {
		name        string
		entity      string
		reason      string
		details     string
		useDetails  bool
		expectedMsg string
	}{
		{
			name:        "basic conflict",
			entity:      "order",
			reason:      "already exists",
			expectedMsg: "order conflict: already exists",
		},
		{
			name:        "with details",
			entity:      "user",
			reason:      "email taken",
			details:     "email@test.com",
			useDetails:  true,
			expectedMsg: "user conflict: email taken (email@test.com)",
		},
		{
			name:        "empty details uses basic format",
			entity:      "item",
			reason:      "version mismatch",
			details:     "",
			useDetails:  true,
			expectedMsg: "item conflict: version mismatch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.useDetails {
				err = NewConflictErrorWithDetails(tt.entity, tt.reason, tt.details)
			} else {
				err = NewConflictError(tt.entity, tt.reason)
			}

			assert.Equal(t, tt.expectedMsg, err.Error())
			require.ErrorIs(t, err, ErrConflict)

			var conflict *ConflictError
			require.ErrorAs(t, err, &conflict)
			assert.Equal(t, tt.entity, conflict.Entity)
			assert.Equal(t, tt.reason, conflict.Reason)
		})
	}
}

func TestConflictError_Unwrap(t *testing.T) {
	err := NewConflictError("order", "exists")

	var conflict *ConflictError
	require.ErrorAs(t, err, &conflict)
	assert.Equal(t, ErrConflict, conflict.Unwrap())
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name        string
		field       string
		message     string
		expectedMsg string
	}{
		{
			name:        "with field",
			field:       "email",
			message:     "invalid format",
			expectedMsg: "validation failed for email: invalid format",
		},
		{
			name:        "without field",
			field:       "",
			message:     "general validation error",
			expectedMsg: "validation failed: general validation error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationError(tt.field, tt.message)

			assert.Equal(t, tt.expectedMsg, err.Error())
			require.ErrorIs(t, err, ErrValidation)

			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, tt.field, validation.Field)
			assert.Equal(t, tt.message, validation.Message)
		})
	}
}

func TestValidationError_Unwrap(t *testing.T) {
	err := NewValidationError("field", "message")

	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, ErrValidation, validation.Unwrap())
}

func TestForbiddenError(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		reason      string
		expectedMsg string
	}{
		{
			name:        "with reason",
			operation:   "delete",
			reason:      "insufficient permissions",
			expectedMsg: `operation "delete" forbidden: insufficient permissions`,
		},
		{
			name:        "without reason",
			operation:   "update",
			reason:      "",
			expectedMsg: `operation "update" forbidden`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewForbiddenError(tt.operation, tt.reason)

			assert.Equal(t, tt.expectedMsg, err.Error())
			require.ErrorIs(t, err, ErrForbidden)

			var forbidden *ForbiddenError
			require.ErrorAs(t, err, &forbidden)
			assert.Equal(t, tt.operation, forbidden.Operation)
			assert.Equal(t, tt.reason, forbidden.Reason)
		})
	}
}

func TestForbiddenError_Unwrap(t *testing.T) {
	err := NewForbiddenError("delete", "no access")

	var forbidden *ForbiddenError
	require.ErrorAs(t, err, &forbidden)
	assert.Equal(t, ErrForbidden, forbidden.Unwrap())
}

func TestUnavailableError(t *testing.T) {
	tests := []struct {
		name        string
		service     string
		reason      string
		expectedMsg string
	}{
		{
			name:        "with reason",
			service:     "payment-gateway",
			reason:      "connection timeout",
			expectedMsg: `service "payment-gateway" unavailable: connection timeout`,
		},
		{
			name:        "without reason",
			service:     "cache",
			reason:      "",
			expectedMsg: `service "cache" unavailable`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewUnavailableError(tt.service, tt.reason)

			assert.Equal(t, tt.expectedMsg, err.Error())
			require.ErrorIs(t, err, ErrUnavailable)

			var unavailable *UnavailableError
			require.ErrorAs(t, err, &unavailable)
			assert.Equal(t, tt.service, unavailable.Service)
			assert.Equal(t, tt.reason, unavailable.Reason)
		})
	}
}

func TestUnavailableError_Unwrap(t *testing.T) {
	err := NewUnavailableError("db", "timeout")

	var unavailable *UnavailableError
	require.ErrorAs(t, err, &unavailable)
	assert.Equal(t, ErrUnavailable, unavailable.Unwrap())
}

func TestIsHelpers(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		isFunc   func(error) bool
		expected bool
	}{
		// NotFound
		{"IsNotFound with NotFoundError", NewNotFoundError("user", "123"), IsNotFound, true},
		{"IsNotFound with sentinel", ErrNotFound, IsNotFound, true},
		{"IsNotFound with wrapped", fmt.Errorf("wrapped: %w", ErrNotFound), IsNotFound, true},
		{"IsNotFound with other error", ErrConflict, IsNotFound, false},
		{"IsNotFound with nil", nil, IsNotFound, false},

		// Conflict
		{"IsConflict with ConflictError", NewConflictError("order", "exists"), IsConflict, true},
		{"IsConflict with sentinel", ErrConflict, IsConflict, true},
		{"IsConflict with wrapped", fmt.Errorf("wrapped: %w", ErrConflict), IsConflict, true},
		{"IsConflict with other error", ErrNotFound, IsConflict, false},
		{"IsConflict with nil", nil, IsConflict, false},

		// Validation
		{"IsValidation with ValidationError", NewValidationError("email", "invalid"), IsValidation, true},
		{"IsValidation with sentinel", ErrValidation, IsValidation, true},
		{"IsValidation with wrapped", fmt.Errorf("wrapped: %w", ErrValidation), IsValidation, true},
		{"IsValidation with other error", ErrNotFound, IsValidation, false},
		{"IsValidation with nil", nil, IsValidation, false},

		// Forbidden
		{"IsForbidden with ForbiddenError", NewForbiddenError("delete", "no access"), IsForbidden, true},
		{"IsForbidden with sentinel", ErrForbidden, IsForbidden, true},
		{"IsForbidden with wrapped", fmt.Errorf("wrapped: %w", ErrForbidden), IsForbidden, true},
		{"IsForbidden with other error", ErrNotFound, IsForbidden, false},
		{"IsForbidden with nil", nil, IsForbidden, false},

		// Unavailable
		{"IsUnavailable with UnavailableError", NewUnavailableError("db", "timeout"), IsUnavailable, true},
		{"IsUnavailable with sentinel", ErrUnavailable, IsUnavailable, true},
		{"IsUnavailable with wrapped", fmt.Errorf("wrapped: %w", ErrUnavailable), IsUnavailable, true},
		{"IsUnavailable with other error", ErrNotFound, IsUnavailable, false},
		{"IsUnavailable with nil", nil, IsUnavailable, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.isFunc(tt.err))
		})
	}
}

func TestErrorWrappingChain(t *testing.T) {
	t.Run("deeply wrapped NotFoundError", func(t *testing.T) {
		original := NewNotFoundError("user", "123")
		wrapped1 := fmt.Errorf("layer1: %w", original)
		wrapped2 := fmt.Errorf("layer2: %w", wrapped1)
		wrapped3 := fmt.Errorf("layer3: %w", wrapped2)

		assert.True(t, IsNotFound(wrapped3))

		var notFound *NotFoundError
		require.ErrorAs(t, wrapped3, &notFound)
		assert.Equal(t, "123", notFound.ID)
		assert.Equal(t, "user", notFound.Entity)
	})

	t.Run("deeply wrapped ConflictError", func(t *testing.T) {
		original := NewConflictErrorWithDetails("order", "version", "v2")
		wrapped := fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", original))

		assert.True(t, IsConflict(wrapped))

		var conflict *ConflictError
		require.ErrorAs(t, wrapped, &conflict)
		assert.Equal(t, "v2", conflict.Details)
	})

	t.Run("deeply wrapped ValidationError", func(t *testing.T) {
		original := NewValidationError("email", "invalid")
		wrapped := fmt.Errorf("validation: %w", original)

		assert.True(t, IsValidation(wrapped))

		var validation *ValidationError
		require.ErrorAs(t, wrapped, &validation)
		assert.Equal(t, "email", validation.Field)
	})

	t.Run("deeply wrapped ForbiddenError", func(t *testing.T) {
		original := NewForbiddenError("delete", "admin only")
		wrapped := fmt.Errorf("auth: %w", original)

		assert.True(t, IsForbidden(wrapped))

		var forbidden *ForbiddenError
		require.ErrorAs(t, wrapped, &forbidden)
		assert.Equal(t, "admin only", forbidden.Reason)
	})

	t.Run("deeply wrapped UnavailableError", func(t *testing.T) {
		original := NewUnavailableError("redis", "connection refused")
		wrapped := fmt.Errorf("cache: %w", original)

		assert.True(t, IsUnavailable(wrapped))

		var unavailable *UnavailableError
		require.ErrorAs(t, wrapped, &unavailable)
		assert.Equal(t, "redis", unavailable.Service)
	})
}
