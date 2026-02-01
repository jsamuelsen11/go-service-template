package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// idMiddlewareConfig configures the ID middleware behavior.
type idMiddlewareConfig struct {
	headerName      string
	contextKey      string
	contextEnricher func(ctx context.Context, id string) context.Context
	contextStorer   func(ctx context.Context, id string) context.Context // Stores ID in context.Context for downstream propagation
}

// createIDMiddleware creates middleware that extracts or generates an ID.
// This is a shared implementation for request ID and correlation ID middleware.
func createIDMiddleware(cfg idMiddlewareConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(cfg.headerName)

		// Generate new UUID if not provided
		if id == "" {
			id = uuid.New().String()
		}

		// Store in gin context for retrieval by handlers
		c.Set(cfg.contextKey, id)

		// Add to response headers
		c.Header(cfg.headerName, id)

		// Store ID in context.Context for downstream propagation (client adapters)
		ctx := c.Request.Context()
		if cfg.contextStorer != nil {
			ctx = cfg.contextStorer(ctx, id)
		}

		// Enrich context logger with ID
		if cfg.contextEnricher != nil {
			ctx = cfg.contextEnricher(ctx, id)
		}

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// getIDFromContext extracts an ID from the gin context by key.
func getIDFromContext(c *gin.Context, key string) string {
	if id, exists := c.Get(key); exists {
		if s, ok := id.(string); ok {
			return s
		}
	}

	return ""
}
