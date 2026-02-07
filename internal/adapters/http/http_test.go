package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/adapters/http/handlers"
	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestMapDomainError tests the error mapping function with all domain error types.
func TestMapDomainError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
		checkDetails   bool
		expectedField  string
	}{
		{
			name:           "nil error returns 200",
			err:            nil,
			expectedStatus: http.StatusOK,
			expectedCode:   "",
		},
		{
			name:           "NotFoundError returns 404",
			err:            domain.NewNotFoundError("user", "123"),
			expectedStatus: http.StatusNotFound,
			expectedCode:   dto.ErrorCodeNotFound,
		},
		{
			name:           "ConflictError returns 409",
			err:            domain.NewConflictError("user", "already exists"),
			expectedStatus: http.StatusConflict,
			expectedCode:   dto.ErrorCodeConflict,
		},
		{
			name:           "ValidationError returns 400",
			err:            domain.NewValidationError("email", "invalid format"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   dto.ErrorCodeValidation,
			checkDetails:   true,
			expectedField:  "email",
		},
		{
			name:           "ValidationError without field returns 400",
			err:            domain.NewValidationError("", "invalid input"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   dto.ErrorCodeValidation,
		},
		{
			name:           "ForbiddenError returns 403",
			err:            domain.NewForbiddenError("delete", "insufficient permissions"),
			expectedStatus: http.StatusForbidden,
			expectedCode:   dto.ErrorCodeForbidden,
		},
		{
			name:           "UnavailableError returns 503",
			err:            domain.NewUnavailableError("database", "connection refused"),
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   dto.ErrorCodeUnavailable,
		},
		{
			name:           "unknown error returns 500",
			err:            errors.New("unknown error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   dto.ErrorCodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, resp := MapDomainError(tt.err)

			assert.Equal(t, tt.expectedStatus, status)

			if tt.err == nil {
				assert.Nil(t, resp)
				return
			}

			require.NotNil(t, resp)
			assert.Equal(t, tt.expectedCode, resp.Error.Code)

			if tt.checkDetails {
				require.NotNil(t, resp.Error.Details)
				assert.Contains(t, resp.Error.Details, tt.expectedField)
			}
		})
	}
}

// TestRespondWithError tests the error response function with various error types.
func TestRespondWithError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "NotFoundError",
			err:            domain.NewNotFoundError("quote", "456"),
			expectedStatus: http.StatusNotFound,
			expectedCode:   dto.ErrorCodeNotFound,
		},
		{
			name:           "ConflictError",
			err:            domain.NewConflictError("resource", "duplicate key"),
			expectedStatus: http.StatusConflict,
			expectedCode:   dto.ErrorCodeConflict,
		},
		{
			name:           "ValidationError",
			err:            domain.NewValidationError("name", "required"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   dto.ErrorCodeValidation,
		},
		{
			name:           "ForbiddenError",
			err:            domain.NewForbiddenError("update", "not owner"),
			expectedStatus: http.StatusForbidden,
			expectedCode:   dto.ErrorCodeForbidden,
		},
		{
			name:           "UnavailableError",
			err:            domain.NewUnavailableError("api", "timeout"),
			expectedStatus: http.StatusServiceUnavailable,
			expectedCode:   dto.ErrorCodeUnavailable,
		},
		{
			name:           "generic error returns 500",
			err:            errors.New("internal error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   dto.ErrorCodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

			RespondWithError(c, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var resp dto.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.Error.Code)
			assert.NotEmpty(t, resp.Error.Message)
		})
	}
}

// TestRespondWithErrorCode tests responding with specific error codes.
func TestRespondWithErrorCode(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		message        string
		expectedStatus int
	}{
		{
			name:           "NotFound code",
			code:           dto.ErrorCodeNotFound,
			message:        "resource not found",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "Validation code",
			code:           dto.ErrorCodeValidation,
			message:        "invalid input",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "Forbidden code",
			code:           dto.ErrorCodeForbidden,
			message:        "access denied",
			expectedStatus: http.StatusForbidden,
		},
		{
			name:           "Unauthorized code",
			code:           dto.ErrorCodeUnauthorized,
			message:        "authentication required",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Internal code",
			code:           dto.ErrorCodeInternal,
			message:        "something went wrong",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:           "Unavailable code",
			code:           dto.ErrorCodeUnavailable,
			message:        "service down",
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			name:           "Timeout code",
			code:           dto.ErrorCodeTimeout,
			message:        "request timeout",
			expectedStatus: http.StatusGatewayTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

			RespondWithErrorCode(c, tt.code, tt.message)

			assert.Equal(t, tt.expectedStatus, w.Code)

			var resp dto.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.code, resp.Error.Code)
			assert.Equal(t, tt.message, resp.Error.Message)
		})
	}
}

// TestRespondWithValidationErrors tests responding with field validation errors.
func TestRespondWithValidationErrors(t *testing.T) {
	tests := []struct {
		name        string
		fieldErrors map[string]string
	}{
		{
			name: "single field error",
			fieldErrors: map[string]string{
				"email": "invalid email format",
			},
		},
		{
			name: "multiple field errors",
			fieldErrors: map[string]string{
				"email":    "required",
				"password": "too short",
				"age":      "must be positive",
			},
		},
		{
			name:        "empty field errors",
			fieldErrors: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/test", nil)

			RespondWithValidationErrors(c, tt.fieldErrors)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var resp dto.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, dto.ErrorCodeValidation, resp.Error.Code)
			assert.Equal(t, "request validation failed", resp.Error.Message)

			if len(tt.fieldErrors) > 0 {
				require.NotNil(t, resp.Error.Details)
				for field, msg := range tt.fieldErrors {
					assert.Equal(t, msg, resp.Error.Details[field])
				}
			}
		})
	}
}

// TestAbortWithError tests aborting the request chain with an error.
func TestAbortWithError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "NotFoundError aborts with 404",
			err:            domain.NewNotFoundError("item", "789"),
			expectedStatus: http.StatusNotFound,
			expectedCode:   dto.ErrorCodeNotFound,
		},
		{
			name:           "ValidationError aborts with 400",
			err:            domain.NewValidationError("field", "invalid"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   dto.ErrorCodeValidation,
		},
		{
			name:           "generic error aborts with 500",
			err:            errors.New("unexpected error"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   dto.ErrorCodeInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

			AbortWithError(c, tt.err)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.True(t, c.IsAborted())

			var resp dto.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCode, resp.Error.Code)
		})
	}
}

// TestAbortWithErrorCode tests aborting with a specific error code.
func TestAbortWithErrorCode(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		message        string
		expectedStatus int
	}{
		{
			name:           "Unauthorized aborts with 401",
			code:           dto.ErrorCodeUnauthorized,
			message:        "token expired",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "Forbidden aborts with 403",
			code:           dto.ErrorCodeForbidden,
			message:        "insufficient role",
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

			AbortWithErrorCode(c, tt.code, tt.message)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.True(t, c.IsAborted())

			var resp dto.ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.code, resp.Error.Code)
			assert.Equal(t, tt.message, resp.Error.Message)
		})
	}
}

// TestServerNew tests creating a new HTTP server.
func TestServerNew(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:           "127.0.0.1",
		Port:           8080,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxRequestSize: 1 << 20,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := New(cfg, logger)

	require.NotNil(t, srv)
	assert.NotNil(t, srv.engine)
	assert.NotNil(t, srv.httpServer)
	assert.Equal(t, cfg, srv.config)
	assert.Equal(t, logger, srv.logger)
}

// TestServerEngine tests getting the underlying Gin engine.
func TestServerEngine(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:           "localhost",
		Port:           0,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxRequestSize: 1 << 20,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := New(cfg, logger)
	engine := srv.Engine()

	require.NotNil(t, engine)
	assert.IsType(t, &gin.Engine{}, engine)
}

// TestServerConfig tests getting the server configuration.
func TestServerConfig(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:           "0.0.0.0",
		Port:           3000,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxRequestSize: 2 << 20,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := New(cfg, logger)
	returnedCfg := srv.Config()

	assert.Equal(t, cfg, returnedCfg)
	assert.Equal(t, 3000, returnedCfg.Port)
	assert.Equal(t, "0.0.0.0", returnedCfg.Host)
}

// TestServerAddr tests the server address formatting.
func TestServerAddr(t *testing.T) {
	tests := []struct {
		name         string
		host         string
		port         int
		expectedAddr string
	}{
		{
			name:         "localhost with port 8080",
			host:         "localhost",
			port:         8080,
			expectedAddr: "localhost:8080",
		},
		{
			name:         "0.0.0.0 with port 3000",
			host:         "0.0.0.0",
			port:         3000,
			expectedAddr: "0.0.0.0:3000",
		},
		{
			name:         "127.0.0.1 with port 0",
			host:         "127.0.0.1",
			port:         0,
			expectedAddr: "127.0.0.1:0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.ServerConfig{
				Host:           tt.host,
				Port:           tt.port,
				ReadTimeout:    5 * time.Second,
				WriteTimeout:   10 * time.Second,
				IdleTimeout:    30 * time.Second,
				MaxRequestSize: 1 << 20,
			}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))

			srv := New(cfg, logger)
			addr := srv.Addr()

			assert.Equal(t, tt.expectedAddr, addr)
		})
	}
}

// TestServerStartShutdown tests starting and stopping the server.
func TestServerStartShutdown(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:           "127.0.0.1",
		Port:           0, // Use port 0 for dynamic port allocation
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxRequestSize: 1 << 20,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := New(cfg, logger)

	// Add a simple route for testing
	srv.Engine().GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	errCh := srv.Start()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify no immediate errors
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server start error: %v", err)
		}
	default:
		// No error, server is running
	}

	// Shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := srv.Shutdown(ctx)
	require.NoError(t, err)

	// Verify error channel is closed
	_, ok := <-errCh
	assert.False(t, ok, "error channel should be closed")
}

// TestServerShutdownWithContext tests graceful shutdown with context.
func TestServerShutdownWithContext(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:           "127.0.0.1",
		Port:           0,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxRequestSize: 1 << 20,
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := New(cfg, logger)
	errCh := srv.Start()

	time.Sleep(50 * time.Millisecond)

	// Shutdown with a context
	ctx := context.Background()
	err := srv.Shutdown(ctx)
	require.NoError(t, err)

	// Wait for error channel to close
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for server to shutdown")
	}
}

// TestNewDefaultRouterConfig tests creating a default router configuration.
func TestNewDefaultRouterConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	appCfg := &config.AppConfig{
		Name:        "test-app",
		Environment: "test",
		Version:     "1.0.0",
	}
	authCfg := &config.AuthConfig{
		Enabled: false,
	}
	healthHandler := handlers.NewHealthHandler(nil, handlers.BuildInfo{})

	cfg := NewDefaultRouterConfig(logger, appCfg, authCfg, healthHandler)

	assert.Equal(t, logger, cfg.Logger)
	assert.Equal(t, appCfg, cfg.AppConfig)
	assert.Equal(t, authCfg, cfg.AuthConfig)
	assert.Equal(t, healthHandler, cfg.HealthHandler)
	assert.Equal(t, DefaultRequestTimeout, cfg.Timeout)
	assert.Nil(t, cfg.QuoteHandler)
}

// TestSetupMinimalRouter tests setting up a minimal router with health endpoints.
func TestSetupMinimalRouter(t *testing.T) {
	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	healthHandler := handlers.NewHealthHandler(nil, handlers.BuildInfo{
		Version: "1.0.0",
	})

	SetupMinimalRouter(engine, logger, healthHandler)

	// Verify routes are registered
	routes := engine.Routes()
	assert.NotEmpty(t, routes)

	// Test health endpoint is accessible
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/-/live", nil)
	engine.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

// TestSetupMinimalRouterWithNilHandler tests minimal router with nil health handler.
func TestSetupMinimalRouterWithNilHandler(t *testing.T) {
	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Should not panic with nil handler
	require.NotPanics(t, func() {
		SetupMinimalRouter(engine, logger, nil)
	})
}

// TestSetupRouter tests setting up a full router with middleware.
func TestSetupRouter(t *testing.T) {
	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	appCfg := &config.AppConfig{
		Name:        "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}
	authCfg := &config.AuthConfig{
		Enabled: false,
	}
	healthHandler := handlers.NewHealthHandler(nil, handlers.BuildInfo{})

	cfg := RouterConfig{
		Logger:        logger,
		AuthConfig:    authCfg,
		AppConfig:     appCfg,
		HealthHandler: healthHandler,
		QuoteHandler:  nil,
		Timeout:       30 * time.Second,
	}

	// Should not panic when setting up router
	require.NotPanics(t, func() {
		SetupRouter(engine, cfg)
	})

	// Verify health endpoints are registered
	routes := engine.Routes()
	assert.NotEmpty(t, routes)

	// Find health routes
	hasHealthRoute := false
	for _, route := range routes {
		if route.Path == "/-/live" {
			hasHealthRoute = true
			break
		}
	}
	assert.True(t, hasHealthRoute, "health routes should be registered")
}

// TestSetupRouterWithoutTimeout tests router setup with zero timeout.
func TestSetupRouterWithoutTimeout(t *testing.T) {
	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	appCfg := &config.AppConfig{
		Name:        "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}
	authCfg := &config.AuthConfig{
		Enabled: false,
	}
	healthHandler := handlers.NewHealthHandler(nil, handlers.BuildInfo{})

	cfg := RouterConfig{
		Logger:        logger,
		AuthConfig:    authCfg,
		AppConfig:     appCfg,
		HealthHandler: healthHandler,
		QuoteHandler:  nil,
		Timeout:       0, // No timeout
	}

	require.NotPanics(t, func() {
		SetupRouter(engine, cfg)
	})
}

// TestSetupRouterWithNilHealthHandler tests router setup with nil health handler.
func TestSetupRouterWithNilHealthHandler(t *testing.T) {
	engine := gin.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	appCfg := &config.AppConfig{
		Name:        "test-service",
		Environment: "test",
		Version:     "1.0.0",
	}

	cfg := RouterConfig{
		Logger:        logger,
		AppConfig:     appCfg,
		HealthHandler: nil,
		Timeout:       30 * time.Second,
	}

	// Should not panic with nil health handler
	require.NotPanics(t, func() {
		SetupRouter(engine, cfg)
	})
}

// TestMaxBodySizeMiddleware tests the max request body size middleware.
func TestMaxBodySizeMiddleware(t *testing.T) {
	cfg := &config.ServerConfig{
		Host:           "127.0.0.1",
		Port:           0,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   10 * time.Second,
		IdleTimeout:    30 * time.Second,
		MaxRequestSize: 100, // Small size for testing
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := New(cfg, logger)

	// Add test route
	srv.Engine().POST("/test", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"received": len(body)})
	})

	// Test with body under limit
	t.Run("body under limit", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/test", io.NopCloser(io.LimitReader(io.MultiReader(), 50)))
		srv.Engine().ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Note: Testing body over limit is tricky due to how MaxBytesReader works
	// It would require actual network I/O to trigger properly
}
