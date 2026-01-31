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

// Config is the root configuration structure.
type Config struct {
	App       AppConfig       `koanf:"app" validate:"required"`
	Server    ServerConfig    `koanf:"server" validate:"required"`
	Log       LogConfig       `koanf:"log" validate:"required"`
	Telemetry TelemetryConfig `koanf:"telemetry"`
	Auth      AuthConfig      `koanf:"auth"`
}

// AppConfig contains application-level settings.
type AppConfig struct {
	Name        string `koanf:"name" validate:"required"`
	Version     string `koanf:"version" validate:"required"`
	Environment string `koanf:"environment" validate:"required,oneof=local dev qa prod test"`
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port            int           `koanf:"port" validate:"required,min=1,max=65535"`
	Host            string        `koanf:"host" validate:"required"`
	ReadTimeout     time.Duration `koanf:"read_timeout" validate:"required,min=1s"`
	WriteTimeout    time.Duration `koanf:"write_timeout" validate:"required,min=1s"`
	IdleTimeout     time.Duration `koanf:"idle_timeout" validate:"required,min=1s"`
	ShutdownTimeout time.Duration `koanf:"shutdown_timeout" validate:"required,min=1s"`
	MaxRequestSize  int64         `koanf:"max_request_size" validate:"required,min=1"`
}

// LogConfig contains logging settings.
type LogConfig struct {
	Level  string `koanf:"level" validate:"required,oneof=debug info warn error"`
	Format string `koanf:"format" validate:"required,oneof=json text"`
}

// TelemetryConfig contains OpenTelemetry settings.
type TelemetryConfig struct {
	Enabled      bool    `koanf:"enabled"`
	Endpoint     string  `koanf:"endpoint" validate:"required_if=Enabled true,omitempty,url"`
	ServiceName  string  `koanf:"service_name" validate:"required_if=Enabled true"`
	SamplingRate float64 `koanf:"sampling_rate" validate:"min=0,max=1"`
}

// AuthConfig contains authentication settings.
type AuthConfig struct {
	Enabled       bool   `koanf:"enabled"`
	JWKSEndpoint  string `koanf:"jwks_endpoint" validate:"required_if=Enabled true,omitempty,url"`
	Issuer        string `koanf:"issuer" validate:"required_if=Enabled true"`
	Audience      string `koanf:"audience" validate:"required_if=Enabled true"`
	ClaimsHeader  string `koanf:"claims_header"`
	RolesHeader   string `koanf:"roles_header"`
	ScopesHeader  string `koanf:"scopes_header"`
	SubjectHeader string `koanf:"subject_header"`
}

// defaults returns the default configuration values.
func defaults() map[string]any {
	return map[string]any{
		"app.name":        "go-service-template",
		"app.version":     "dev",
		"app.environment": "local",

		"server.port":             8080,
		"server.host":             "0.0.0.0",
		"server.read_timeout":     "30s",
		"server.write_timeout":    "30s",
		"server.idle_timeout":     "120s",
		"server.shutdown_timeout": "10s",
		"server.max_request_size": 1048576, // 1MB

		"log.level":  "info",
		"log.format": "json",

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
	if err := k.Load(confmap.Provider(defaults(), "."), nil); err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// 2. Load base config file if it exists
	if err := loadFileIfExists(k, "configs/base.yaml"); err != nil {
		return nil, fmt.Errorf("loading base config: %w", err)
	}

	// 3. Load profile config file if it exists
	if profile != "" {
		profilePath := fmt.Sprintf("configs/%s.yaml", profile)
		if err := loadFileIfExists(k, profilePath); err != nil {
			return nil, fmt.Errorf("loading profile config %q: %w", profile, err)
		}
	}

	// 4. Load environment variables with APP_ prefix
	if err := k.Load(env.Provider("APP_", ".", func(s string) string {
		// APP_SERVER_PORT -> server.port
		return strings.ReplaceAll(
			strings.ToLower(strings.TrimPrefix(s, "APP_")),
			"_",
			".",
		)
	}), nil); err != nil {
		return nil, fmt.Errorf("loading env vars: %w", err)
	}

	// Unmarshal into Config struct
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
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
