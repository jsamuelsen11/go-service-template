package logging

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

var defaultLogger = slog.Default()

// FromContext extracts the logger from context.
// Returns the default logger if no logger is found or ctx is nil.
func FromContext(ctx context.Context) *slog.Logger {
	if ctx == nil {
		return defaultLogger
	}

	if logger, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return logger
	}

	return defaultLogger
}

// WithContext stores a logger in the context.
func WithContext(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}

// WithRequestID adds a request ID to the logger in context.
// Returns a new context with the enriched logger.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	logger := FromContext(ctx).With(slog.String("request_id", requestID))
	return WithContext(ctx, logger)
}

// WithTraceID adds a trace ID to the logger in context.
// Returns a new context with the enriched logger.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	logger := FromContext(ctx).With(slog.String("trace_id", traceID))
	return WithContext(ctx, logger)
}

// WithCorrelationID adds a correlation ID to the logger in context.
// Returns a new context with the enriched logger.
func WithCorrelationID(ctx context.Context, correlationID string) context.Context {
	logger := FromContext(ctx).With(slog.String("correlation_id", correlationID))
	return WithContext(ctx, logger)
}

// SetDefault sets the default logger used when no logger is in context.
func SetDefault(logger *slog.Logger) {
	defaultLogger = logger
	slog.SetDefault(logger)
}
