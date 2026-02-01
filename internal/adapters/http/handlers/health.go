// Package handlers provides HTTP request handlers for the service.
package handlers

import (
	"net/http"
	"runtime"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jsamuelsen/go-service-template/internal/ports"
)

// BuildInfo contains build-time information about the service.
// These values are typically injected at build time using ldflags.
type BuildInfo struct {
	// Version is the semantic version of the service.
	Version string `json:"version"`

	// Commit is the git commit SHA.
	Commit string `json:"commit"`

	// BuildTime is the timestamp when the binary was built.
	BuildTime string `json:"buildTime"`

	// GoVersion is the Go version used to build the binary.
	GoVersion string `json:"goVersion"`
}

// NewBuildInfo creates a BuildInfo with the Go version automatically set.
func NewBuildInfo(version, commit, buildTime string) BuildInfo {
	return BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildTime: buildTime,
		GoVersion: runtime.Version(),
	}
}

// HealthHandler handles health-related HTTP endpoints.
type HealthHandler struct {
	registry  ports.HealthRegistry
	buildInfo BuildInfo
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(registry ports.HealthRegistry, buildInfo BuildInfo) *HealthHandler {
	return &HealthHandler{
		registry:  registry,
		buildInfo: buildInfo,
	}
}

// livenessResponse is the response structure for /-/live endpoint.
type livenessResponse struct {
	Status string `json:"status"`
}

// Liveness handles the /-/live endpoint.
// Returns 200 OK if the process is running. This endpoint is used by
// Kubernetes liveness probes to detect if the container needs to be restarted.
//
// This endpoint should always return 200 as long as the process is running.
// It should NOT check any dependencies - that's what readiness is for.
func (h *HealthHandler) Liveness(c *gin.Context) {
	c.JSON(http.StatusOK, livenessResponse{
		Status: "ok",
	})
}

// readinessResponse is the response structure for /-/ready endpoint.
type readinessResponse struct {
	Status string                        `json:"status"`
	Checks map[string]*ports.CheckResult `json:"checks,omitempty"`
}

// Readiness handles the /-/ready endpoint.
// Returns 200 OK if all registered health checks pass, 503 Service Unavailable otherwise.
// This endpoint is used by Kubernetes readiness probes to determine if the
// container should receive traffic.
func (h *HealthHandler) Readiness(c *gin.Context) {
	result := h.registry.CheckAll(c.Request.Context())

	resp := readinessResponse{
		Status: string(result.Status),
		Checks: result.Checks,
	}

	status := http.StatusOK
	if result.Status == ports.HealthStatusUnhealthy {
		status = http.StatusServiceUnavailable
	}

	c.JSON(status, resp)
}

// BuildInfoHandler handles the /-/build endpoint.
// Returns build information including version, commit, and build time.
func (h *HealthHandler) BuildInfoHandler(c *gin.Context) {
	c.JSON(http.StatusOK, h.buildInfo)
}

// MetricsHandler returns an http.Handler for Prometheus metrics.
// Use this with gin.WrapH() to register it as a route.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RegisterHealthRoutes registers all health-related routes on the given router group.
// Routes are registered under the /-/ prefix:
//   - GET /-/live - Liveness probe
//   - GET /-/ready - Readiness probe
//   - GET /-/build - Build information
//   - GET /-/metrics - Prometheus metrics
func (h *HealthHandler) RegisterHealthRoutes(rg *gin.RouterGroup) {
	rg.GET("/live", h.Liveness)
	rg.GET("/ready", h.Readiness)
	rg.GET("/build", h.BuildInfoHandler)
	rg.GET("/metrics", gin.WrapH(MetricsHandler()))
}

// RegisterHealthRoutesOnEngine is a convenience method to register health routes
// directly on the engine using the /-/ prefix.
func (h *HealthHandler) RegisterHealthRoutesOnEngine(engine *gin.Engine) {
	health := engine.Group("/-")
	h.RegisterHealthRoutes(health)
}
