// Package middleware provides HTTP middleware components for the Gin server.
package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

const (
	// HeaderRequestID is the header name for request ID.
	HeaderRequestID = "X-Request-ID"

	// ContextKeyRequestID is the context key for storing the request ID.
	ContextKeyRequestID = "request_id"
)

// RequestID returns middleware that extracts or generates a request ID.
// The request ID is:
//   - Extracted from the X-Request-ID header if present
//   - Generated as a new UUID v4 if not present
//   - Stored in the gin.Context for later retrieval
//   - Added to the response headers
//   - Added to the context logger for structured logging
func RequestID() gin.HandlerFunc {
	return createIDMiddleware(idMiddlewareConfig{
		headerName:      HeaderRequestID,
		contextKey:      ContextKeyRequestID,
		contextEnricher: logging.WithRequestID,
	})
}

// GetRequestID extracts the request ID from the gin.Context.
// Returns empty string if not set.
func GetRequestID(c *gin.Context) string {
	return getIDFromContext(c, ContextKeyRequestID)
}

// MustGetRequestID extracts the request ID from the gin.Context.
// Returns "unknown" if not set (should not happen if middleware is applied).
func MustGetRequestID(c *gin.Context) string {
	if id := GetRequestID(c); id != "" {
		return id
	}

	return "unknown"
}
