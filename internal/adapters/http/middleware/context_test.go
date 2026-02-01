package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContextWithRequestID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"stores request ID", "test-request-id-123"},
		{"handles empty string", ""},
		{"handles UUID format", "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ContextWithRequestID(context.Background(), tt.id)
			assert.Equal(t, tt.id, RequestIDFromContext(ctx))
		})
	}
}

func TestContextWithCorrelationID(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"stores correlation ID", "test-correlation-id-456"},
		{"handles empty string", ""},
		{"handles UUID format", "550e8400-e29b-41d4-a716-446655440000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ContextWithCorrelationID(context.Background(), tt.id)
			assert.Equal(t, tt.id, CorrelationIDFromContext(ctx))
		})
	}
}

func TestIDFromContext_NotSet(t *testing.T) {
	ctx := context.Background()

	assert.Empty(t, RequestIDFromContext(ctx))
	assert.Empty(t, CorrelationIDFromContext(ctx))
}

func TestBothIDsCanBeStoredTogether(t *testing.T) {
	ctx := context.Background()
	ctx = ContextWithRequestID(ctx, "request-123")
	ctx = ContextWithCorrelationID(ctx, "correlation-456")

	assert.Equal(t, "request-123", RequestIDFromContext(ctx))
	assert.Equal(t, "correlation-456", CorrelationIDFromContext(ctx))
}
