// Package config provides configuration loading and management using koanf.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Default configuration values.
const (
	// DefaultServerPort is the default HTTP server port.
	DefaultServerPort = 8080

	// DefaultMaxRequestSize is the default maximum request body size (1MB).
	DefaultMaxRequestSize = 1 << 20 // 1048576 bytes

	// DefaultClientRetryMaxAttempts is the default number of retry attempts.
	DefaultClientRetryMaxAttempts = 3

	// DefaultClientRetryMultiplier is the default exponential backoff multiplier.
	DefaultClientRetryMultiplier = 2.0

	// DefaultClientRetryJitterFactor is the default jitter percentage (±25%).
	DefaultClientRetryJitterFactor = 0.25

	// DefaultClientCircuitMaxFailures is the default failures before circuit opens.
	DefaultClientCircuitMaxFailures = 5

	// DefaultClientCircuitHalfOpenLimit is the default successes to close circuit.
	DefaultClientCircuitHalfOpenLimit = 3

	// DefaultTransportMaxIdleConns is the default max idle connections.
	DefaultTransportMaxIdleConns = 100

	// DefaultTransportMaxIdleConnsPerHost is the default max idle connections per host.
	DefaultTransportMaxIdleConnsPerHost = 10

	// DefaultTransportIdleConnTimeout is the default idle connection timeout.
	DefaultTransportIdleConnTimeout = 90 * time.Second

	// DefaultLogFileMaxSizeMB is the default max log file size in megabytes.
	DefaultLogFileMaxSizeMB = 100

	// DefaultLogFileMaxBackups is the default number of old log files to retain.
	DefaultLogFileMaxBackups = 3

	// DefaultLogFileMaxAgeDays is the default max days to retain old log files.
	DefaultLogFileMaxAgeDays = 28
)

// Config is the root configuration structure.
type Config struct {
	App       AppConfig       `koanf:"app"       validate:"required"`
	Server    ServerConfig    `koanf:"server"    validate:"required"`
	Log       LogConfig       `koanf:"log"       validate:"required"`
	Telemetry TelemetryConfig `koanf:"telemetry"`
	Auth      AuthConfig      `koanf:"auth"`
	Client    ClientConfig    `koanf:"client"    validate:"required"`
	Services  ServicesConfig  `koanf:"services"  validate:"required"`
}

// AppConfig contains application-level settings.
type AppConfig struct {
	Name        string `koanf:"name"        validate:"required"`
	Version     string `koanf:"version"     validate:"required"`
	Environment string `koanf:"environment" validate:"required,oneof=local dev qa prod test"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port            int           `koanf:"port"             validate:"required,min=1,max=65535"`
	Host            string        `koanf:"host"             validate:"required"`
	ReadTimeout     time.Duration `koanf:"read_timeout"     validate:"required,min=1s"`
	WriteTimeout    time.Duration `koanf:"write_timeout"    validate:"required,min=1s"`
	IdleTimeout     time.Duration `koanf:"idle_timeout"     validate:"required,min=1s"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout" validate:"required,min=1s"`
	MaxRequestSize  int64         `koanf:"max_request_size" validate:"required,min=1"`
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level  string        `koanf:"level"  validate:"required,oneof=debug info warn error"`
	Format string        `koanf:"format" validate:"required,oneof=json text pretty"`
	File   LogFileConfig `koanf:"file"`
}

// LogFileConfig contains rolling log file settings.
type LogFileConfig struct {
	Enabled    bool   `koanf:"enabled"`
	Path       string `koanf:"path"       validate:"required_if=Enabled true"`
	MaxSizeMB  int    `koanf:"max_size"   validate:"omitempty,min=1,max=1024"`
	MaxBackups int    `koanf:"max_backups" validate:"omitempty,min=0,max=100"`
	MaxAgeDays int    `koanf:"max_age"    validate:"omitempty,min=0,max=365"`
	Compress   bool   `koanf:"compress"`
}

// TelemetryConfig contains OpenTelemetry settings.
type TelemetryConfig struct {
	Enabled      bool    `koanf:"enabled"`
	Endpoint     string  `koanf:"endpoint"      validate:"required_if=Enabled true,omitempty,url"`
	ServiceName  string  `koanf:"service_name"  validate:"required_if=Enabled true"`
	SamplingRate float64 `koanf:"sampling_rate" validate:"min=0,max=1"`
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	Enabled       bool   `koanf:"enabled"`
	JWKSEndpoint  string `koanf:"jwks_endpoint"  validate:"required_if=Enabled true,omitempty,url"`
	Issuer        string `koanf:"issuer"         validate:"required_if=Enabled true"`
	Audience      string `koanf:"audience"       validate:"required_if=Enabled true"`
	ClaimsHeader  string `koanf:"claims_header"`
	RolesHeader   string `koanf:"roles_header"`
	ScopesHeader  string `koanf:"scopes_header"`
	SubjectHeader string `koanf:"subject_header"`
}

// ClientConfig contains HTTP client settings for downstream services.
type ClientConfig struct {
	Timeout        time.Duration        `koanf:"timeout"         validate:"required,min=100ms"`
	Retry          RetryConfig          `koanf:"retry"           validate:"required"`
	CircuitBreaker CircuitBreakerConfig `koanf:"circuit_breaker" validate:"required"`
	Transport      TransportConfig      `koanf:"transport"       validate:"required"`
}

// RetryConfig contains retry settings for HTTP clients.
type RetryConfig struct {
	MaxAttempts     int           `koanf:"max_attempts"     validate:"required,min=1,max=10"`
	InitialInterval time.Duration `koanf:"initial_interval" validate:"required,min=10ms"`
	MaxInterval     time.Duration `koanf:"max_interval"     validate:"required,min=100ms"`
	Multiplier      float64       `koanf:"multiplier"       validate:"required,min=1.1,max=10"`
	JitterFactor    float64       `koanf:"jitter_factor"    validate:"min=0,max=1"`
}

// CircuitBreakerConfig contains circuit breaker settings for HTTP clients.
type CircuitBreakerConfig struct {
	MaxFailures   int           `koanf:"max_failures"    validate:"required,min=1"`
	Timeout       time.Duration `koanf:"timeout"         validate:"required,min=1s"`
	HalfOpenLimit int           `koanf:"half_open_limit" validate:"required,min=1"`
}

// TransportConfig contains HTTP transport pool settings.
type TransportConfig struct {
	MaxIdleConns        int           `koanf:"max_idle_conns"         validate:"required,min=1"`
	MaxIdleConnsPerHost int           `koanf:"max_idle_conns_per_host" validate:"required,min=1"`
	IdleConnTimeout     time.Duration `koanf:"idle_conn_timeout"      validate:"required,min=1s"`
}

// ServicesConfig contains configuration for downstream services.
type ServicesConfig struct {
	Quote ServiceEndpointConfig `koanf:"quote" validate:"required"`
}

// ServiceEndpointConfig contains configuration for a downstream service endpoint.
type ServiceEndpointConfig struct {
	BaseURL string `koanf:"base_url" validate:"required,url"`
	Name    string `koanf:"name"     validate:"required"`
}

// defaults returns the default configuration values.
func defaults() map[string]any {
	return map[string]any{
		"app.name":        "go-service-template",
		"app.version":     "dev",
		"app.environment": "local",

		"server.port":             DefaultServerPort,
		"server.host":             "0.0.0.0",
		"server.read_timeout":     "30s",
		"server.write_timeout":    "30s",
		"server.idle_timeout":     "120s",
		"server.shutdown_timeout": "10s",
		"server.max_request_size": DefaultMaxRequestSize,

		"log.level":            "info",
		"log.format":           "json",
		"log.file.enabled":     false,
		"log.file.path":        "./logs/app.log",
		"log.file.max_size":    DefaultLogFileMaxSizeMB,
		"log.file.max_backups": DefaultLogFileMaxBackups,
		"log.file.max_age":     DefaultLogFileMaxAgeDays,
		"log.file.compress":    true,

		"telemetry.enabled":       false,
		"telemetry.endpoint":      "",
		"telemetry.service_name":  "go-service-template",
		"telemetry.sampling_rate": 1.0,

		"auth.enabled":        false,
		"auth.jwks_endpoint":  "",
		"auth.issuer":         "",
		"auth.audience":       "",
		"auth.claims_header":  "X-User-Claims",
		"auth.roles_header":   "X-User-Roles",
		"auth.scopes_header":  "X-User-Scopes",
		"auth.subject_header": "X-User-ID",

		"client.timeout":                           "30s",
		"client.retry.max_attempts":                DefaultClientRetryMaxAttempts,
		"client.retry.initial_interval":            "100ms",
		"client.retry.max_interval":                "5s",
		"client.retry.multiplier":                  DefaultClientRetryMultiplier,
		"client.retry.jitter_factor":               DefaultClientRetryJitterFactor,
		"client.circuit_breaker.max_failures":      DefaultClientCircuitMaxFailures,
		"client.circuit_breaker.timeout":           "30s",
		"client.circuit_breaker.half_open_limit":   DefaultClientCircuitHalfOpenLimit,
		"client.transport.max_idle_conns":          DefaultTransportMaxIdleConns,
		"client.transport.max_idle_conns_per_host": DefaultTransportMaxIdleConnsPerHost,
		"client.transport.idle_conn_timeout":       "90s",

		"services.quote.base_url": "https://api.quotable.io",
		"services.quote.name":     "quote-service",
	}
}

// Load loads configuration with the following precedence (highest to lowest):
//  1. Environment variables (APP_ prefix)
//  2. Profile config file (configs/{profile}.yaml)
//  3. Base config file (configs/base.yaml)
//  4. Default values
func Load(profile string) (*Config, error) {
	k := koanf.New(".")

	// 1. Load defaults
	err := k.Load(confmap.Provider(defaults(), "."), nil)
	if err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// 2. Load base config file if it exists
	err = loadFileIfExists(k, "configs/base.yaml")
	if err != nil {
		return nil, fmt.Errorf("loading base config: %w", err)
	}

	// 3. Load profile config file if it exists
	if profile != "" {
		profilePath := fmt.Sprintf("configs/%s.yaml", profile)

		err := loadFileIfExists(k, profilePath)
		if err != nil {
			return nil, fmt.Errorf("loading profile config %q: %w", profile, err)
		}
	}

	// 4. Load environment variables with APP_ prefix
	err = k.Load(env.Provider("APP_", ".", func(s string) string {
		return strings.ReplaceAll(
			strings.ToLower(strings.TrimPrefix(s, "APP_")),
			"_",
			".",
		)
	}), nil)
	if err != nil {
		return nil, fmt.Errorf("loading env vars: %w", err)
	}

	// Unmarshal into Config struct
	var cfg Config

	err = k.Unmarshal("", &cfg)
	if err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &cfg, nil
}

// loadFileIfExists loads a YAML config file if it exists.
// Returns nil if the file doesn't exist, error only for parse/read failures.
func loadFileIfExists(k *koanf.Koanf, path string) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil // File doesn't exist, that's fine
	}

	return k.Load(file.Provider(path), yaml.Parser())
}
