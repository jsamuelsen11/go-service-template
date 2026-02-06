package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/jsamuelsen/go-service-template/internal/mocks"
	"github.com/jsamuelsen/go-service-template/internal/ports"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewBuildInfo(t *testing.T) {
	bi := NewBuildInfo("1.0.0", "abc123", "2024-01-15T10:00:00Z")

	assert.Equal(t, "1.0.0", bi.Version)
	assert.Equal(t, "abc123", bi.Commit)
	assert.Equal(t, "2024-01-15T10:00:00Z", bi.BuildTime)
	assert.Equal(t, runtime.Version(), bi.GoVersion)
}

func TestNewHealthHandler(t *testing.T) {
	registry := mocks.NewMockHealthRegistry(t)
	buildInfo := NewBuildInfo("1.0.0", "abc123", "2024-01-15T10:00:00Z")

	handler := NewHealthHandler(registry, buildInfo)

	require.NotNil(t, handler)
}

func TestHealthHandler_Liveness(t *testing.T) {
	registry := mocks.NewMockHealthRegistry(t)
	handler := NewHealthHandler(registry, BuildInfo{})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/-/live", nil)

	handler.Liveness(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp livenessResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

func TestHealthHandler_Readiness(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mocks.MockHealthRegistry)
		expectedStatus int
		expectedBody   string
	}{
		{
			name: "all checks healthy",
			setupMock: func(m *mocks.MockHealthRegistry) {
				m.EXPECT().CheckAll(mock.Anything).Return(&ports.HealthResult{
					Status: ports.HealthStatusHealthy,
					Checks: map[string]*ports.CheckResult{
						"database": {Status: ports.HealthStatusHealthy},
						"cache":    {Status: ports.HealthStatusHealthy},
					},
				})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "healthy",
		},
		{
			name: "one check unhealthy",
			setupMock: func(m *mocks.MockHealthRegistry) {
				m.EXPECT().CheckAll(mock.Anything).Return(&ports.HealthResult{
					Status: ports.HealthStatusUnhealthy,
					Checks: map[string]*ports.CheckResult{
						"database": {Status: ports.HealthStatusHealthy},
						"cache":    {Status: ports.HealthStatusUnhealthy, Message: "connection refused"},
					},
				})
			},
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   "unhealthy",
		},
		{
			name: "no checks registered",
			setupMock: func(m *mocks.MockHealthRegistry) {
				m.EXPECT().CheckAll(mock.Anything).Return(&ports.HealthResult{
					Status: ports.HealthStatusHealthy,
					Checks: map[string]*ports.CheckResult{},
				})
			},
			expectedStatus: http.StatusOK,
			expectedBody:   "healthy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := mocks.NewMockHealthRegistry(t)
			tt.setupMock(registry)

			handler := NewHealthHandler(registry, BuildInfo{})

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/-/ready", nil)

			handler.Readiness(c)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedBody)
		})
	}
}

func TestHealthHandler_BuildInfoHandler(t *testing.T) {
	registry := mocks.NewMockHealthRegistry(t)
	buildInfo := BuildInfo{
		Version:   "1.2.3",
		Commit:    "def456",
		BuildTime: "2024-02-01T12:00:00Z",
		GoVersion: "go1.21.0",
	}

	handler := NewHealthHandler(registry, buildInfo)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/-/build", nil)

	handler.BuildInfoHandler(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp BuildInfo
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", resp.Version)
	assert.Equal(t, "def456", resp.Commit)
	assert.Equal(t, "2024-02-01T12:00:00Z", resp.BuildTime)
	assert.Equal(t, "go1.21.0", resp.GoVersion)
}

func TestMetricsHandler(t *testing.T) {
	handler := MetricsHandler()

	require.NotNil(t, handler)

	// Test that it returns prometheus metrics
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)

	handler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	// Prometheus metrics are text/plain with version
	assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
}

func TestHealthHandler_RegisterHealthRoutes(t *testing.T) {
	registry := mocks.NewMockHealthRegistry(t)
	registry.EXPECT().CheckAll(mock.Anything).Return(&ports.HealthResult{
		Status: ports.HealthStatusHealthy,
		Checks: map[string]*ports.CheckResult{},
	}).Maybe()

	handler := NewHealthHandler(registry, BuildInfo{Version: "test"})

	router := gin.New()
	group := router.Group("/-")
	handler.RegisterHealthRoutes(group)

	routes := router.Routes()

	expectedRoutes := []string{
		"GET /-/live",
		"GET /-/ready",
		"GET /-/build",
		"GET /-/metrics",
	}

	routeMap := make(map[string]bool)
	for _, r := range routes {
		routeMap[r.Method+" "+r.Path] = true
	}

	for _, expected := range expectedRoutes {
		assert.True(t, routeMap[expected], "missing route: %s", expected)
	}
}

func TestHealthHandler_RegisterHealthRoutesOnEngine(t *testing.T) {
	registry := mocks.NewMockHealthRegistry(t)
	registry.EXPECT().CheckAll(mock.Anything).Return(&ports.HealthResult{
		Status: ports.HealthStatusHealthy,
		Checks: map[string]*ports.CheckResult{},
	}).Maybe()

	handler := NewHealthHandler(registry, BuildInfo{})

	router := gin.New()
	handler.RegisterHealthRoutesOnEngine(router)

	// Test that liveness endpoint works
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/-/live", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
