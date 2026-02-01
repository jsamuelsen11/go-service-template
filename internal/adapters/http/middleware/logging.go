package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

// Logging returns middleware that logs HTTP requests.
// It logs:
//   - Request start: method, path, request_id, correlation_id
//   - Request completion: status, latency, bytes written
//
// Health check paths (starting with /-/) are skipped to avoid log noise.
func Logging(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip logging for health check endpoints
		if strings.HasPrefix(c.Request.URL.Path, "/-/") {
			c.Next()
			return
		}

		start := time.Now()

		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path = path + "?" + c.Request.URL.RawQuery
		}

		// Get context logger (enriched with request_id, correlation_id, trace_id)
		ctxLogger := logging.FromContext(c.Request.Context())

		// Log request start
		ctxLogger.Info("request started",
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.String("client_ip", c.ClientIP()),
			slog.String("user_agent", c.Request.UserAgent()),
		)

		// Process request
		c.Next()

		// Log request completion
		latency := time.Since(start)
		status := c.Writer.Status()
		size := c.Writer.Size()

		// Choose log level based on status code
		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		ctxLogger.Log(c.Request.Context(), level, "request completed",
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("latency", latency),
			slog.Int64("latency_ms", latency.Milliseconds()),
			slog.Int("bytes", size),
		)
	}
}

// LoggingWithSkipPaths returns logging middleware with configurable paths to skip.
func LoggingWithSkipPaths(logger *slog.Logger, skipPaths []string) gin.HandlerFunc {
	skipMap := make(map[string]struct{}, len(skipPaths))
	for _, path := range skipPaths {
		skipMap[path] = struct{}{}
	}

	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Check exact path match first
		if _, skip := skipMap[path]; skip {
			c.Next()
			return
		}

		// Check prefix match for /-/ paths
		if strings.HasPrefix(path, "/-/") {
			c.Next()
			return
		}

		start := time.Now()

		fullPath := path
		if c.Request.URL.RawQuery != "" {
			fullPath = fullPath + "?" + c.Request.URL.RawQuery
		}

		ctxLogger := logging.FromContext(c.Request.Context())

		ctxLogger.Info("request started",
			slog.String("method", c.Request.Method),
			slog.String("path", fullPath),
			slog.String("client_ip", c.ClientIP()),
		)

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		size := c.Writer.Size()

		level := slog.LevelInfo
		if status >= http.StatusInternalServerError {
			level = slog.LevelError
		} else if status >= http.StatusBadRequest {
			level = slog.LevelWarn
		}

		ctxLogger.Log(c.Request.Context(), level, "request completed",
			slog.String("method", c.Request.Method),
			slog.String("path", fullPath),
			slog.Int("status", status),
			slog.Duration("latency", latency),
			slog.Int("bytes", size),
		)
	}
}
