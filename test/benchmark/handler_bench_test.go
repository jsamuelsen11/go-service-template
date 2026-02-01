package benchmark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/handlers"
	"github.com/jsamuelsen/go-service-template/internal/ports"
)

func init() {
	// Set Gin to release mode for accurate benchmarks
	gin.SetMode(gin.ReleaseMode)
}

// createGinContext creates a Gin context for handler testing.
func createGinContext(w http.ResponseWriter, r *http.Request) *gin.Context {
	c, _ := gin.CreateTestContext(w)
	c.Request = r
	return c
}

// setupHealthHandler creates a HealthHandler with a minimal registry for benchmarking.
func setupHealthHandler() *handlers.HealthHandler {
	registry := ports.NewHealthRegistry()
	buildInfo := handlers.NewBuildInfo("1.0.0", "abc123", "2024-01-01T00:00:00Z")
	return handlers.NewHealthHandler(registry, buildInfo)
}

// BenchmarkLivenessHandler measures the performance of the liveness endpoint.
// This is a critical path for Kubernetes probes and should be extremely fast.
func BenchmarkLivenessHandler(b *testing.B) {
	handler := setupHealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/-/live", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c := createGinContext(w, req)
		handler.Liveness(c)
	}
}

// BenchmarkReadinessHandler measures the performance of the readiness endpoint.
// This includes running all registered health checks.
func BenchmarkReadinessHandler(b *testing.B) {
	handler := setupHealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/-/ready", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c := createGinContext(w, req)
		handler.Readiness(c)
	}
}

// BenchmarkReadinessHandler_WithChecks measures readiness with registered health checks.
func BenchmarkReadinessHandler_WithChecks(b *testing.B) {
	registry := ports.NewHealthRegistry()

	// Register a simple health check
	_ = registry.Register(&simpleHealthChecker{name: "database"})
	_ = registry.Register(&simpleHealthChecker{name: "cache"})

	buildInfo := handlers.NewBuildInfo("1.0.0", "abc123", "2024-01-01T00:00:00Z")
	handler := handlers.NewHealthHandler(registry, buildInfo)
	req := httptest.NewRequest(http.MethodGet, "/-/ready", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c := createGinContext(w, req)
		handler.Readiness(c)
	}
}

// BenchmarkBuildInfoHandler measures the performance of the build info endpoint.
func BenchmarkBuildInfoHandler(b *testing.B) {
	handler := setupHealthHandler()
	req := httptest.NewRequest(http.MethodGet, "/-/build", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		c := createGinContext(w, req)
		handler.BuildInfoHandler(c)
	}
}

// BenchmarkMiddlewareChain measures the overhead of the middleware chain.
func BenchmarkMiddlewareChain(b *testing.B) {
	router := gin.New()

	// Add common middleware
	router.Use(gin.Recovery())

	// Simple handler
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// BenchmarkMiddlewareChain_Full measures the full middleware chain with all middleware.
func BenchmarkMiddlewareChain_Full(b *testing.B) {
	router := gin.New()

	// Add multiple middleware layers
	router.Use(gin.Recovery())
	router.Use(gin.Logger())

	// Simple handler
	router.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// simpleHealthChecker is a minimal health checker for benchmarking.
type simpleHealthChecker struct {
	name string
}

func (s *simpleHealthChecker) Name() string {
	return s.name
}

func (s *simpleHealthChecker) Check(_ context.Context) error {
	return nil
}
