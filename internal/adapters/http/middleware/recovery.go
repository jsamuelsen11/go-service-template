package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

// extractTraceID extracts the OpenTelemetry trace ID from the context.
// Returns an empty string if no trace ID is available.
func extractTraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().HasTraceID() {
		return span.SpanContext().TraceID().String()
	}
	return ""
}

// sendPanicResponse sends an internal server error response for a panic.
// If headers have already been written, it only aborts the request.
func sendPanicResponse(c *gin.Context, traceID string) {
	errResp := dto.NewErrorResponse(
		dto.ErrorCodeInternal,
		"an internal error occurred",
	)
	if traceID != "" {
		errResp.TraceID = traceID
	}

	if c.Writer.Written() {
		c.Abort()
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, errResp)
}

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
				stack := debug.Stack()
				ctxLogger := logging.FromContext(c.Request.Context())
				traceID := extractTraceID(c.Request.Context())

				ctxLogger.Error("panic recovered",
					slog.Any("error", r),
					slog.String("stack", string(stack)),
					slog.String("path", c.Request.URL.Path),
					slog.String("method", c.Request.Method),
					slog.String("trace_id", traceID),
				)

				sendPanicResponse(c, traceID)
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

				if stackHandler != nil {
					stackHandler(r, stack)
				}

				ctxLogger := logging.FromContext(c.Request.Context())
				traceID := extractTraceID(c.Request.Context())

				ctxLogger.Error("panic recovered",
					slog.Any("error", r),
					slog.String("stack", string(stack)),
					slog.String("trace_id", traceID),
				)

				sendPanicResponse(c, traceID)
			}
		}()

		c.Next()
	}
}
