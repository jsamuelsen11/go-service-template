// Package middleware provides HTTP middleware for the Gin framework.
package middleware

import "context"

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	// ctxKeyRequestID is the context key for storing request ID in context.Context.
	ctxKeyRequestID contextKey = "request_id"

	// ctxKeyCorrelationID is the context key for storing correlation ID in context.Context.
	ctxKeyCorrelationID contextKey = "correlation_id"
)

// RequestIDFromContext extracts the request ID from context.Context.
// Returns empty string if not set or if ctx is nil.
// This is used by client adapters to propagate the request ID to downstream services.
func RequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if id, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return id
	}

	return ""
}

// CorrelationIDFromContext extracts the correlation ID from context.Context.
// Returns empty string if not set or if ctx is nil.
// This is used by client adapters to propagate the correlation ID to downstream services.
func CorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	if id, ok := ctx.Value(ctxKeyCorrelationID).(string); ok {
		return id
	}

	return ""
}

// ContextWithRequestID stores a request ID in the context.
// This is typically called by the request ID middleware.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// ContextWithCorrelationID stores a correlation ID in the context.
// This is typically called by the correlation ID middleware.
func ContextWithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyCorrelationID, id)
}
