package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoad_DefaultValues tests that hardcoded defaults are applied correctly.
// This test doesn't depend on YAML files - it only tests the defaults() function.
func TestLoad_DefaultValues(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	// Check defaults are applied (from defaults() function)
	assert.Equal(t, "go-service-template", cfg.App.Name)
	assert.Equal(t, "dev", cfg.App.Version)
	assert.Equal(t, "local", cfg.App.Environment)
	assert.Equal(t, DefaultServerPort, cfg.Server.Port)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, DefaultClientRetryMaxAttempts, cfg.Client.Retry.MaxAttempts)
	assert.Equal(t, DefaultClientRetryMultiplier, cfg.Client.Retry.Multiplier)
	assert.Equal(t, DefaultClientCircuitMaxFailures, cfg.Client.CircuitBreaker.MaxFailures)
}

// TestLoad_EnvVarOverrides tests that environment variables override defaults.
func TestLoad_EnvVarOverrides(t *testing.T) {
	// Set environment variables
	t.Setenv("APP_SERVER_PORT", "9090")
	t.Setenv("APP_LOG_LEVEL", "warn")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, "warn", cfg.Log.Level)
}

// TestLoad_DurationParsing tests that duration strings are parsed correctly.
func TestLoad_DurationParsing(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	// Verify durations are parsed correctly from defaults
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, 10*time.Second, cfg.Server.ShutdownTimeout)
	assert.Equal(t, 100*time.Millisecond, cfg.Client.Retry.InitialInterval)
	assert.Equal(t, 5*time.Second, cfg.Client.Retry.MaxInterval)
	assert.Equal(t, 30*time.Second, cfg.Client.Timeout)
}

// TestLoad_NonExistentProfile tests that a missing profile file doesn't cause errors.
func TestLoad_NonExistentProfile(t *testing.T) {
	// Should not error - missing profile file is silently ignored
	cfg, err := Load("nonexistent")
	require.NoError(t, err)

	// Should fall back to defaults
	assert.Equal(t, "go-service-template", cfg.App.Name)
}

// TestLoad_BoolEnvVar tests that boolean environment variables are parsed correctly.
func TestLoad_BoolEnvVar(t *testing.T) {
	t.Setenv("APP_TELEMETRY_ENABLED", "true")

	cfg, err := Load("")
	require.NoError(t, err)

	assert.True(t, cfg.Telemetry.Enabled)
}

// TestLoad_AuthHeaderDefaults tests that auth header defaults are set correctly.
func TestLoad_AuthHeaderDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	// Check default auth headers
	assert.Equal(t, "X-User-Claims", cfg.Auth.ClaimsHeader)
	assert.Equal(t, "X-User-Roles", cfg.Auth.RolesHeader)
	assert.Equal(t, "X-User-Scopes", cfg.Auth.ScopesHeader)
	assert.Equal(t, "X-User-ID", cfg.Auth.SubjectHeader)
}

// TestLoad_LogFileDefaults tests that log file defaults are set correctly.
func TestLoad_LogFileDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	// Check log file defaults
	assert.False(t, cfg.Log.File.Enabled)
	assert.Equal(t, "./logs/app.log", cfg.Log.File.Path)
	assert.Equal(t, DefaultLogFileMaxSizeMB, cfg.Log.File.MaxSizeMB)
	assert.Equal(t, DefaultLogFileMaxBackups, cfg.Log.File.MaxBackups)
	assert.Equal(t, DefaultLogFileMaxAgeDays, cfg.Log.File.MaxAgeDays)
	assert.True(t, cfg.Log.File.Compress)
}

// TestLoad_TelemetryDefaults tests that telemetry defaults are set correctly.
func TestLoad_TelemetryDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	assert.False(t, cfg.Telemetry.Enabled)
	assert.Equal(t, "go-service-template", cfg.Telemetry.ServiceName)
	assert.Equal(t, 1.0, cfg.Telemetry.SamplingRate)
}

// TestLoad_ClientDefaults tests that HTTP client defaults are set correctly.
func TestLoad_ClientDefaults(t *testing.T) {
	cfg, err := Load("")
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.Client.Timeout)
	assert.Equal(t, DefaultClientRetryMaxAttempts, cfg.Client.Retry.MaxAttempts)
	assert.Equal(t, 100*time.Millisecond, cfg.Client.Retry.InitialInterval)
	assert.Equal(t, 5*time.Second, cfg.Client.Retry.MaxInterval)
	assert.Equal(t, DefaultClientRetryMultiplier, cfg.Client.Retry.Multiplier)
	assert.Equal(t, DefaultClientCircuitMaxFailures, cfg.Client.CircuitBreaker.MaxFailures)
	assert.Equal(t, 30*time.Second, cfg.Client.CircuitBreaker.Timeout)
	assert.Equal(t, DefaultClientCircuitHalfOpenLimit, cfg.Client.CircuitBreaker.HalfOpenLimit)
}

// TestDefaults tests that the defaults map contains expected values.
func TestDefaults(t *testing.T) {
	d := defaults()

	assert.Equal(t, "go-service-template", d["app.name"])
	assert.Equal(t, "dev", d["app.version"])
	assert.Equal(t, "local", d["app.environment"])
	assert.Equal(t, DefaultServerPort, d["server.port"])
	assert.Equal(t, "0.0.0.0", d["server.host"])
	assert.Equal(t, "info", d["log.level"])
	assert.Equal(t, "json", d["log.format"])
	assert.Equal(t, DefaultClientRetryMaxAttempts, d["client.retry.max_attempts"])
	assert.Equal(t, DefaultClientRetryMultiplier, d["client.retry.multiplier"])
}
