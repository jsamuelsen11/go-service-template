package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validConfig returns a fully valid configuration for testing.
func validConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:        "test-service",
			Version:     "1.0.0",
			Environment: "local",
		},
		Server: ServerConfig{
			Port:            8080,
			Host:            "0.0.0.0",
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    30 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 10 * time.Second,
			MaxRequestSize:  1048576,
		},
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},
		Client: ClientConfig{
			Timeout: 30 * time.Second,
			Retry: RetryConfig{
				MaxAttempts:     3,
				InitialInterval: 100 * time.Millisecond,
				MaxInterval:     5 * time.Second,
				Multiplier:      2.0,
				JitterFactor:    0.25,
			},
			CircuitBreaker: CircuitBreakerConfig{
				MaxFailures:   5,
				Timeout:       30 * time.Second,
				HalfOpenLimit: 3,
			},
			Transport: TransportConfig{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		Services: ServicesConfig{
			Quote: ServiceEndpointConfig{
				BaseURL: "https://api.quotable.io",
				Name:    "quote-service",
			},
		},
	}
}

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := validConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_AppConfig(t *testing.T) {
	t.Run("missing name", func(t *testing.T) {
		cfg := validConfig()
		cfg.App.Name = ""

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "app.name")
		assert.Contains(t, err.Error(), "required")
	})

	t.Run("missing version", func(t *testing.T) {
		cfg := validConfig()
		cfg.App.Version = ""

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "app.version")
	})

	t.Run("missing environment", func(t *testing.T) {
		cfg := validConfig()
		cfg.App.Environment = ""

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "app.environment")
	})

	t.Run("invalid environment", func(t *testing.T) {
		cfg := validConfig()
		cfg.App.Environment = "invalid"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "app.environment")
		assert.Contains(t, err.Error(), "must be one of")
	})
}

func TestConfig_Validate_ValidEnvironments(t *testing.T) {
	validEnvs := []string{"local", "dev", "qa", "prod", "test"}

	for _, env := range validEnvs {
		t.Run(env, func(t *testing.T) {
			cfg := validConfig()
			cfg.App.Environment = env

			err := cfg.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestConfig_Validate_ServerConfig(t *testing.T) {
	t.Run("valid port range", func(t *testing.T) {
		tests := []struct {
			name    string
			port    int
			wantErr bool
		}{
			{"minimum valid port", 1, false},
			{"typical port", 8080, false},
			{"maximum valid port", 65535, false},
			{"zero port", 0, true},
			{"negative port", -1, true},
			{"port too high", 65536, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				cfg := validConfig()
				cfg.Server.Port = tt.port

				err := cfg.Validate()
				if tt.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "server.port")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("missing host", func(t *testing.T) {
		cfg := validConfig()
		cfg.Server.Host = ""

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server.host")
	})

	t.Run("timeout minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Server.ReadTimeout = 500 * time.Millisecond // Less than 1s minimum

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server.readtimeout")
	})

	t.Run("max request size minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Server.MaxRequestSize = 0

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "server.maxrequestsize")
	})
}

func TestConfig_Validate_LogConfig(t *testing.T) {
	t.Run("valid log levels", func(t *testing.T) {
		levels := []string{"debug", "info", "warn", "error"}
		for _, level := range levels {
			t.Run(level, func(t *testing.T) {
				cfg := validConfig()
				cfg.Log.Level = level

				err := cfg.Validate()
				assert.NoError(t, err)
			})
		}
	})

	t.Run("invalid log level", func(t *testing.T) {
		cfg := validConfig()
		cfg.Log.Level = "invalid"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "log.level")
		assert.Contains(t, err.Error(), "must be one of")
	})

	t.Run("case sensitive log level", func(t *testing.T) {
		cfg := validConfig()
		cfg.Log.Level = "DEBUG" // Should be lowercase

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "log.level")
	})

	t.Run("valid log formats", func(t *testing.T) {
		formats := []string{"json", "text", "pretty"}
		for _, format := range formats {
			t.Run(format, func(t *testing.T) {
				cfg := validConfig()
				cfg.Log.Format = format

				err := cfg.Validate()
				assert.NoError(t, err)
			})
		}
	})

	t.Run("invalid log format", func(t *testing.T) {
		cfg := validConfig()
		cfg.Log.Format = "xml"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "log.format")
	})
}

func TestConfig_Validate_LogFileConfig(t *testing.T) {
	t.Run("file logging disabled - path not required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Log.File.Enabled = false
		cfg.Log.File.Path = ""

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("file logging enabled - path required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Log.File.Enabled = true
		cfg.Log.File.Path = ""

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "log.file.path")
	})

	t.Run("file logging enabled with valid config", func(t *testing.T) {
		cfg := validConfig()
		cfg.Log.File.Enabled = true
		cfg.Log.File.Path = "/var/log/app.log"
		cfg.Log.File.MaxSizeMB = 100
		cfg.Log.File.MaxBackups = 3
		cfg.Log.File.MaxAgeDays = 28

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("max size bounds", func(t *testing.T) {
		cfg := validConfig()
		cfg.Log.File.Enabled = true
		cfg.Log.File.Path = "/var/log/app.log"
		cfg.Log.File.MaxSizeMB = 1025 // Exceeds max of 1024

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "log.file.maxsize")
	})
}

func TestConfig_Validate_TelemetryConfig(t *testing.T) {
	t.Run("telemetry disabled - endpoint not required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Telemetry.Enabled = false
		cfg.Telemetry.Endpoint = ""

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("telemetry enabled - endpoint required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Telemetry.Enabled = true
		cfg.Telemetry.Endpoint = ""
		cfg.Telemetry.ServiceName = "test"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "telemetry.endpoint")
	})

	t.Run("telemetry enabled - service name required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Telemetry.Enabled = true
		cfg.Telemetry.Endpoint = "http://localhost:4317"
		cfg.Telemetry.ServiceName = ""

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "telemetry.servicename")
	})

	t.Run("telemetry enabled - invalid endpoint URL", func(t *testing.T) {
		cfg := validConfig()
		cfg.Telemetry.Enabled = true
		cfg.Telemetry.Endpoint = "not-a-url"
		cfg.Telemetry.ServiceName = "test"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "telemetry.endpoint")
	})

	t.Run("telemetry enabled with valid config", func(t *testing.T) {
		cfg := validConfig()
		cfg.Telemetry.Enabled = true
		cfg.Telemetry.Endpoint = "http://localhost:4317"
		cfg.Telemetry.ServiceName = "test-service"
		cfg.Telemetry.SamplingRate = 0.5

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("sampling rate bounds", func(t *testing.T) {
		tests := []struct {
			rate    float64
			wantErr bool
		}{
			{0.0, false},
			{0.5, false},
			{1.0, false},
			{-0.1, true},
			{1.1, true},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("rate_%v", tt.rate), func(t *testing.T) {
				cfg := validConfig()
				cfg.Telemetry.SamplingRate = tt.rate

				err := cfg.Validate()
				if tt.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "telemetry.samplingrate")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestConfig_Validate_AuthConfig(t *testing.T) {
	t.Run("auth disabled - fields not required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Auth.Enabled = false

		err := cfg.Validate()
		assert.NoError(t, err)
	})

	t.Run("auth enabled - jwks endpoint required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Auth.Enabled = true
		cfg.Auth.JWKSEndpoint = ""
		cfg.Auth.Issuer = "https://auth.example.com"
		cfg.Auth.Audience = "my-api"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth.jwksendpoint")
	})

	t.Run("auth enabled - issuer required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Auth.Enabled = true
		cfg.Auth.JWKSEndpoint = "https://auth.example.com/.well-known/jwks.json"
		cfg.Auth.Issuer = ""
		cfg.Auth.Audience = "my-api"

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth.issuer")
	})

	t.Run("auth enabled - audience required", func(t *testing.T) {
		cfg := validConfig()
		cfg.Auth.Enabled = true
		cfg.Auth.JWKSEndpoint = "https://auth.example.com/.well-known/jwks.json"
		cfg.Auth.Issuer = "https://auth.example.com"
		cfg.Auth.Audience = ""

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "auth.audience")
	})

	t.Run("auth enabled with valid config", func(t *testing.T) {
		cfg := validConfig()
		cfg.Auth.Enabled = true
		cfg.Auth.JWKSEndpoint = "https://auth.example.com/.well-known/jwks.json"
		cfg.Auth.Issuer = "https://auth.example.com"
		cfg.Auth.Audience = "my-api"

		err := cfg.Validate()
		assert.NoError(t, err)
	})
}

func TestConfig_Validate_ClientConfig(t *testing.T) {
	t.Run("timeout minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Client.Timeout = 50 * time.Millisecond // Less than 100ms minimum

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client.timeout")
	})
}

//nolint:dupl // Test functions with similar structure are acceptable
func TestConfig_Validate_RetryConfig(t *testing.T) {
	t.Run("max attempts bounds", func(t *testing.T) {
		tests := []struct {
			attempts int
			wantErr  bool
		}{
			{1, false},
			{3, false},
			{10, false},
			{0, true},
			{11, true},
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("attempts_%d", tt.attempts), func(t *testing.T) {
				cfg := validConfig()
				cfg.Client.Retry.MaxAttempts = tt.attempts

				err := cfg.Validate()
				if tt.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "client.retry.maxattempts")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("initial interval minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Client.Retry.InitialInterval = 5 * time.Millisecond // Less than 10ms minimum

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client.retry.initialinterval")
	})

	t.Run("max interval minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Client.Retry.MaxInterval = 50 * time.Millisecond // Less than 100ms minimum

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client.retry.maxinterval")
	})

	t.Run("multiplier bounds", func(t *testing.T) {
		tests := []struct {
			multiplier float64
			wantErr    bool
		}{
			{1.1, false},
			{2.0, false},
			{10.0, false},
			{1.0, true},  // Less than 1.1 minimum
			{10.1, true}, // Greater than 10 maximum
		}

		for _, tt := range tests {
			t.Run(fmt.Sprintf("multiplier_%v", tt.multiplier), func(t *testing.T) {
				cfg := validConfig()
				cfg.Client.Retry.Multiplier = tt.multiplier

				err := cfg.Validate()
				if tt.wantErr {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "client.retry.multiplier")
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestConfig_Validate_CircuitBreakerConfig(t *testing.T) {
	t.Run("max failures minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Client.CircuitBreaker.MaxFailures = 0

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client.circuitbreaker.maxfailures")
	})

	t.Run("timeout minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Client.CircuitBreaker.Timeout = 500 * time.Millisecond // Less than 1s minimum

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client.circuitbreaker.timeout")
	})

	t.Run("half open limit minimum", func(t *testing.T) {
		cfg := validConfig()
		cfg.Client.CircuitBreaker.HalfOpenLimit = 0

		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "client.circuitbreaker.halfopenlimit")
	})
}

func TestConfig_Validate_MultipleErrors(t *testing.T) {
	cfg := &Config{
		App: AppConfig{
			Name:        "",        // missing
			Version:     "",        // missing
			Environment: "invalid", // invalid
		},
		Server: ServerConfig{
			Port: -1, // invalid
		},
		// Other fields will fail required validation
	}

	err := cfg.Validate()
	require.Error(t, err)

	// Should report multiple errors
	errStr := err.Error()
	assert.Contains(t, errStr, "app.name")
	assert.Contains(t, errStr, "app.version")
}

func TestFormatFieldPath(t *testing.T) {
	tests := []struct {
		namespace string
		expected  string
	}{
		{"Config.Server.Port", "server.port"},
		{"Config.App.Name", "app.name"},
		{"Config.Client.Retry.MaxAttempts", "client.retry.maxattempts"},
		{"Config.Log.File.Path", "log.file.path"},
		{"Config.Telemetry.SamplingRate", "telemetry.samplingrate"},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			result := formatFieldPath(tt.namespace)
			assert.Equal(t, tt.expected, result)
		})
	}
}
