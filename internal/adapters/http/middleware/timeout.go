package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

// Timeout returns middleware that enforces a request timeout.
// If the request exceeds the timeout, it:
//   - Cancels the context (handlers should respect this)
//   - Returns 503 Service Unavailable with error envelope
//   - Logs the timeout as a warning
//
// Note: This middleware sets a context deadline but cannot forcibly stop
// handlers that don't respect context cancellation.
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		// Replace request context with timeout context
		c.Request = c.Request.WithContext(ctx)

		// Channel to detect if handler completed
		done := make(chan struct{})

		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			// Handler completed normally
			return
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				handleTimeout(c, timeout)
			}
		}
	}
}

// TimeoutWithSkipPaths returns timeout middleware that skips certain paths.
// Useful for long-running endpoints like file uploads or streaming.
func TimeoutWithSkipPaths(timeout time.Duration, skipPaths []string) gin.HandlerFunc {
	skipMap := make(map[string]struct{}, len(skipPaths))
	for _, path := range skipPaths {
		skipMap[path] = struct{}{}
	}

	return func(c *gin.Context) {
		// Skip timeout for specified paths
		if _, skip := skipMap[c.Request.URL.Path]; skip {
			c.Next()
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		done := make(chan struct{})

		go func() {
			c.Next()
			close(done)
		}()

		select {
		case <-done:
			return
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				handleTimeout(c, timeout)
			}
		}
	}
}

// handleTimeout handles a timeout by logging and responding with an error.
func handleTimeout(c *gin.Context, timeout time.Duration) {
	ctxLogger := logging.FromContext(c.Request.Context())

	// Get trace ID for response
	var traceID string
	if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
		traceID = span.SpanContext().TraceID().String()
	}

	ctxLogger.Warn("request timeout",
		slog.String("path", c.Request.URL.Path),
		slog.String("method", c.Request.Method),
		slog.Duration("timeout", timeout),
		slog.String("trace_id", traceID),
	)

	errResp := dto.NewErrorResponse(
		dto.ErrorCodeTimeout,
		"request timeout exceeded",
	)
	if traceID != "" {
		errResp.TraceID = traceID
	}

	if !c.Writer.Written() {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, errResp)
	} else {
		c.Abort()
	}
}

// SimpleTimeout returns a simpler timeout middleware that only sets
// the context deadline without attempting to abort on timeout.
// Handlers must check ctx.Done() and handle timeout themselves.
// This is more reliable for handlers that do context-aware work.
func SimpleTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
