package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

const (
	// HeaderCorrelationID is the header name for correlation ID.
	// Unlike request ID (per-request), correlation ID tracks an entire
	// business transaction across multiple services.
	HeaderCorrelationID = "X-Correlation-ID"

	// ContextKeyCorrelationID is the context key for storing the correlation ID.
	ContextKeyCorrelationID = "correlation_id"
)

// CorrelationID returns middleware that handles correlation ID propagation.
// The correlation ID is:
//   - Extracted from X-Correlation-ID header if present (propagated from upstream)
//   - Generated as a new UUID v4 if not present (this is the transaction origin)
//   - Stored in gin.Context for retrieval by handlers and downstream calls
//   - Added to response headers
//   - Added to context logger for structured logging
//
// This enables distributed tracing across service boundaries.
func CorrelationID() gin.HandlerFunc {
	return createIDMiddleware(idMiddlewareConfig{
		headerName:      HeaderCorrelationID,
		contextKey:      ContextKeyCorrelationID,
		contextEnricher: logging.WithCorrelationID,
	})
}

// GetCorrelationID extracts the correlation ID from the gin.Context.
// Returns empty string if not set.
func GetCorrelationID(c *gin.Context) string {
	return getIDFromContext(c, ContextKeyCorrelationID)
}

// MustGetCorrelationID extracts the correlation ID from the gin.Context.
// Returns "unknown" if not set (should not happen if middleware is applied).
func MustGetCorrelationID(c *gin.Context) string {
	if id := GetCorrelationID(c); id != "" {
		return id
	}

	return "unknown"
}
