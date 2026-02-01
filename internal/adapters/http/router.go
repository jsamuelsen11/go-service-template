package http

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/handlers"
	"github.com/jsamuelsen/go-service-template/internal/adapters/http/middleware"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
	"github.com/jsamuelsen/go-service-template/internal/platform/telemetry"
)

// DefaultRequestTimeout is the default timeout for API requests.
const DefaultRequestTimeout = 30 * time.Second

// RouterConfig contains configuration for setting up the router.
type RouterConfig struct {
	// Logger is the structured logger for request logging.
	Logger *slog.Logger

	// AuthConfig contains authentication header configuration.
	AuthConfig *config.AuthConfig

	// AppConfig contains application configuration.
	AppConfig *config.AppConfig

	// HealthHandler handles health check endpoints.
	HealthHandler *handlers.HealthHandler

	// Timeout is the default request timeout.
	Timeout time.Duration
}

// SetupRouter configures all routes and middleware on the Gin engine.
// Middleware is applied in the following order (first to last):
//  1. Recovery - catch panics first
//  2. Request ID - generate/extract request ID
//  3. Correlation ID - handle distributed tracing correlation
//  4. OpenTelemetry - tracing and metrics
//  5. Logging - request logging (skips health endpoints)
//  6. Timeout - request deadline (applied per-route or globally)
//
// Route groups:
//   - /-/ (internal): Health endpoints, no auth required
//   - /api/v1/ (public API): Business endpoints, auth as needed
func SetupRouter(engine *gin.Engine, cfg RouterConfig) {
	// Apply global middleware in order
	engine.Use(
		middleware.Recovery(cfg.Logger),
		middleware.RequestID(),
		middleware.CorrelationID(),
		telemetry.Middleware(cfg.AppConfig.Name),
		middleware.Logging(cfg.Logger),
	)

	// Register health endpoints (no auth, no timeout for probes)
	if cfg.HealthHandler != nil {
		cfg.HealthHandler.RegisterHealthRoutesOnEngine(engine)
	}

	// Setup API v1 routes with timeout
	apiV1 := engine.Group("/api/v1")
	if cfg.Timeout > 0 {
		apiV1.Use(middleware.SimpleTimeout(cfg.Timeout))
	}

	// Register API routes
	setupAPIRoutes(apiV1, cfg)
}

// setupAPIRoutes registers business API routes.
// This is where you add your application endpoints.
func setupAPIRoutes(rg *gin.RouterGroup, cfg RouterConfig) {
	// Example: Add your routes here
	//
	// Public routes (no auth):
	// rg.GET("/public/resource", handlers.GetPublicResource)
	//
	// Protected routes (require authentication):
	// protected := rg.Group("")
	// protected.Use(middleware.RequireAuth(cfg.AuthConfig))
	// protected.GET("/users/me", handlers.GetCurrentUser)
	//
	// Admin routes (require admin role):
	// admin := rg.Group("/admin")
	// admin.Use(middleware.RequireAuth(cfg.AuthConfig))
	// admin.Use(middleware.RequireRole(cfg.AuthConfig, "admin"))
	// admin.GET("/users", handlers.ListUsers)
}

// SetupMinimalRouter sets up a minimal router with just health endpoints.
// Useful for testing or lightweight deployments.
func SetupMinimalRouter(engine *gin.Engine, logger *slog.Logger, healthHandler *handlers.HealthHandler) {
	engine.Use(
		middleware.Recovery(logger),
		middleware.RequestID(),
	)

	if healthHandler != nil {
		healthHandler.RegisterHealthRoutesOnEngine(engine)
	}
}

// NewDefaultRouterConfig creates a RouterConfig with sensible defaults.
func NewDefaultRouterConfig(
	logger *slog.Logger,
	appCfg *config.AppConfig,
	authCfg *config.AuthConfig,
	healthHandler *handlers.HealthHandler,
) RouterConfig {
	return RouterConfig{
		Logger:        logger,
		AuthConfig:    authCfg,
		AppConfig:     appCfg,
		HealthHandler: healthHandler,
		Timeout:       DefaultRequestTimeout,
	}
}
