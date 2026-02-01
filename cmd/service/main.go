// Package main is the entry point for the service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/adapters/clients/acl"
	"github.com/jsamuelsen/go-service-template/internal/adapters/http"
	"github.com/jsamuelsen/go-service-template/internal/adapters/http/handlers"
	"github.com/jsamuelsen/go-service-template/internal/app"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
	"github.com/jsamuelsen/go-service-template/internal/platform/telemetry"
	"github.com/jsamuelsen/go-service-template/internal/ports"
)

// Build-time variables, injected via ldflags.
// Example: go build -ldflags "-X main.Version=1.0.0 -X main.Commit=$(git rev-parse HEAD) -X main.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	// Version is the semantic version of the service.
	Version = "dev"

	// Commit is the git commit SHA.
	Commit = "unknown"

	// BuildTime is the timestamp when the binary was built.
	BuildTime = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	// 1. Determine profile from environment
	profile := os.Getenv("APP_ENVIRONMENT")
	if profile == "" {
		profile = "local"
	}

	// 2. Load and validate configuration (fail fast)
	cfg, err := config.Load(profile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 3. Initialize logging
	logger := logging.New(logging.Config{
		Level:   cfg.Log.Level,
		Format:  cfg.Log.Format,
		Service: cfg.App.Name,
		Version: cfg.App.Version,
	})
	slog.SetDefault(logger)

	logger.Info("starting service",
		slog.String("version", Version),
		slog.String("commit", Commit),
		slog.String("environment", cfg.App.Environment),
	)

	// 4. Initialize telemetry (noop if disabled)
	telProvider, err := telemetry.New(ctx, &telemetry.Config{
		Enabled:      cfg.Telemetry.Enabled,
		Endpoint:     cfg.Telemetry.Endpoint,
		ServiceName:  cfg.Telemetry.ServiceName,
		Version:      cfg.App.Version,
		Environment:  cfg.App.Environment,
		SamplingRate: cfg.Telemetry.SamplingRate,
	})
	if err != nil {
		return fmt.Errorf("initializing telemetry: %w", err)
	}

	defer func() {
		if shutdownErr := telProvider.Shutdown(ctx); shutdownErr != nil {
			logger.Error("telemetry shutdown error", slog.Any("error", shutdownErr))
		}
	}()

	// 5. Create health registry
	healthRegistry := ports.NewHealthRegistry()

	// 6. Create HTTP client for downstream services
	httpClient, err := clients.New(&clients.Config{
		BaseURL:     "https://api.quotable.io",
		ServiceName: "quote-service",
		Timeout:     cfg.Client.Timeout,
		Retry:       cfg.Client.Retry,
		Circuit:     cfg.Client.CircuitBreaker,
		Logger:      logger,
	})
	if err != nil {
		return fmt.Errorf("creating HTTP client: %w", err)
	}

	// 7. Create quote client adapter (ACL pattern)
	quoteClient := acl.NewQuoteClient(acl.QuoteClientConfig{
		Client: httpClient,
		Logger: logger,
	})

	// Register quote client as a health checker
	if err := healthRegistry.Register(quoteClient); err != nil {
		return fmt.Errorf("registering quote client health check: %w", err)
	}

	// 8. Create quote service (application layer)
	quoteService := app.NewQuoteService(app.QuoteServiceConfig{
		QuoteClient: quoteClient,
		Logger:      logger,
	})

	// 9. Create handlers
	buildInfo := handlers.NewBuildInfo(Version, Commit, BuildTime)
	healthHandler := handlers.NewHealthHandler(healthRegistry, buildInfo)
	quoteHandler := handlers.NewQuoteHandler(quoteService)

	// 10. Create HTTP server
	server := http.New(&cfg.Server, logger)

	// 11. Setup router with all middleware and routes
	routerCfg := http.RouterConfig{
		Logger:        logger,
		AuthConfig:    &cfg.Auth,
		AppConfig:     &cfg.App,
		HealthHandler: healthHandler,
		QuoteHandler:  quoteHandler,
		Timeout:       http.DefaultRequestTimeout,
	}
	http.SetupRouter(server.Engine(), routerCfg)

	// 12. Start server (non-blocking)
	serverErr := server.Start()

	// 13. Wait for shutdown signal
	return waitForShutdown(ctx, logger, server, serverErr, cfg.Server.ShutdownTimeout)
}

// waitForShutdown blocks until a shutdown signal is received or server error occurs.
// It then performs graceful shutdown of the HTTP server.
func waitForShutdown(
	ctx context.Context,
	logger *slog.Logger,
	server *http.Server,
	serverErr <-chan error,
	shutdownTimeout time.Duration,
) error {
	// Listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		// Server error during startup or runtime
		return fmt.Errorf("server error: %w", err)

	case sig := <-quit:
		logger.Info("received shutdown signal", slog.String("signal", sig.String()))
	}

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()

	// Graceful shutdown sequence
	logger.Info("initiating graceful shutdown",
		slog.Duration("timeout", shutdownTimeout),
	)

	// Stop accepting new requests, drain in-flight
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	logger.Info("shutdown complete")

	return nil
}
