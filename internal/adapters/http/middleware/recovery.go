package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

// Recovery returns middleware that recovers from panics.
// On panic, it:
//   - Logs the error with full stack trace at ERROR level
//   - Returns a 500 Internal Server Error with standard error envelope
//   - Includes trace_id in the response for debugging
//
// This middleware should be applied first in the chain to catch panics
// from all subsequent handlers and middleware.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Get stack trace
				stack := debug.Stack()

				// Get context logger (has request_id, correlation_id)
				ctxLogger := logging.FromContext(c.Request.Context())

				// Extract trace ID for response
				var traceID string
				if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
					traceID = span.SpanContext().TraceID().String()
				}

				// Log the panic with full context
				ctxLogger.Error("panic recovered",
					slog.Any("error", r),
					slog.String("stack", string(stack)),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("trace_id", traceID),
				)

				// Build error response
				errResp := dto.NewErrorResponse(
					dto.ErrorCodeInternal,
					"an internal error occurred",
				)
				if traceID != "" {
					errResp.TraceID = traceID
				}

				// Ensure headers haven't been sent yet
				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, errResp)
				} else {
					c.Abort()
				}
			}
		}()

		c.Next()
	}
}

// RecoveryWithWriter returns recovery middleware that writes to a custom logger.
// Use this if you need the stack trace written to a specific destination.
func RecoveryWithWriter(logger *slog.Logger, stackHandler func(err any, stack []byte)) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()

				// Call custom handler if provided
				if stackHandler != nil {
					stackHandler(r, stack)
				}

				ctxLogger := logging.FromContext(c.Request.Context())

				var traceID string
				if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
					traceID = span.SpanContext().TraceID().String()
				}

				ctxLogger.Error("panic recovered",
					slog.Any("error", r),
					slog.String("stack", string(stack)),
					slog.String("trace_id", traceID),
				)

				errResp := dto.NewErrorResponse(
					dto.ErrorCodeInternal,
					"an internal error occurred",
				)
				if traceID != "" {
					errResp.TraceID = traceID
				}

				if !c.Writer.Written() {
					c.AbortWithStatusJSON(http.StatusInternalServerError, errResp)
				} else {
					c.Abort()
				}
			}
		}()

		c.Next()
	}
}
