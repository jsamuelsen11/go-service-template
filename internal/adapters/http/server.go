// Package http provides the HTTP adapter layer using Gin.
package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

// Server wraps http.Server with Gin and provides graceful shutdown.
type Server struct {
	engine     *gin.Engine
	httpServer *http.Server
	config     *config.ServerConfig
	logger     *slog.Logger
}

// New creates a new HTTP server with the provided configuration.
func New(cfg *config.ServerConfig, logger *slog.Logger) *Server {
	// Set Gin mode based on configuration
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()

	// Apply max request body size middleware
	engine.Use(maxBodySize(cfg.MaxRequestSize))

	httpServer := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler:      engine,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return &Server{
		engine:     engine,
		httpServer: httpServer,
		config:     cfg,
		logger:     logger,
	}
}

// Engine returns the underlying Gin engine for route registration.
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Config returns the server configuration.
func (s *Server) Config() *config.ServerConfig {
	return s.config
}

// Start begins listening and serving HTTP requests.
// Returns an error channel that will receive any ListenAndServe errors.
// This method is non-blocking.
func (s *Server) Start() <-chan error {
	errCh := make(chan error, 1)

	go func() {
		s.logger.Info("starting HTTP server",
			slog.String("addr", s.httpServer.Addr),
			slog.Duration("read_timeout", s.config.ReadTimeout),
			slog.Duration("write_timeout", s.config.WriteTimeout),
		)

		err := s.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server error: %w", err)
		}

		close(errCh)
	}()

	return errCh
}

// Shutdown gracefully stops the server, waiting for active connections to finish.
// The provided context controls the shutdown timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")

	err := s.httpServer.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("http server shutdown: %w", err)
	}

	s.logger.Info("HTTP server stopped")

	return nil
}

// Addr returns the server's listening address.
func (s *Server) Addr() string {
	return s.httpServer.Addr
}

// maxBodySize returns middleware that limits the request body size.
func maxBodySize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
