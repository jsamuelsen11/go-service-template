package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Context tests

func TestFromContext_NilContext(t *testing.T) {
	logger := FromContext(nil) //nolint:staticcheck // Testing nil guard intentionally
	assert.NotNil(t, logger)
	assert.Equal(t, defaultLogger, logger)
}

func TestFromContext_NoLogger(t *testing.T) {
	ctx := context.Background()
	logger := FromContext(ctx)
	assert.NotNil(t, logger)
	assert.Equal(t, defaultLogger, logger)
}

func TestFromContext_WithLogger(t *testing.T) {
	customLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := WithContext(context.Background(), customLogger)
	logger := FromContext(ctx)
	assert.NotNil(t, logger)
	assert.Equal(t, customLogger, logger)
}

func TestWithContext(t *testing.T) {
	customLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := WithContext(context.Background(), customLogger)
	retrievedLogger := FromContext(ctx)
	assert.Equal(t, customLogger, retrievedLogger)
}

func TestWithRequestID(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	ctx := WithContext(context.Background(), logger)
	ctx = WithRequestID(ctx, "req-123")

	fromCtx := FromContext(ctx)
	fromCtx.InfoContext(ctx, "test message")

	output := buf.String()
	assert.Contains(t, output, "req-123")
	assert.Contains(t, output, "request_id")

	// Verify JSON structure
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "req-123", logEntry["request_id"])
}

func TestWithTraceID(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	ctx := WithContext(context.Background(), logger)
	ctx = WithTraceID(ctx, "trace-456")

	fromCtx := FromContext(ctx)
	fromCtx.InfoContext(ctx, "test message")

	output := buf.String()
	assert.Contains(t, output, "trace-456")
	assert.Contains(t, output, "trace_id")

	// Verify JSON structure
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "trace-456", logEntry["trace_id"])
}

func TestWithCorrelationID(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	ctx := WithContext(context.Background(), logger)
	ctx = WithCorrelationID(ctx, "corr-789")

	fromCtx := FromContext(ctx)
	fromCtx.InfoContext(ctx, "test message")

	output := buf.String()
	assert.Contains(t, output, "corr-789")
	assert.Contains(t, output, "correlation_id")

	// Verify JSON structure
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "corr-789", logEntry["correlation_id"])
}

func TestSetDefault(t *testing.T) {
	// Save original default logger
	originalDefault := defaultLogger

	// Create custom logger
	customLogger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Set as default
	SetDefault(customLogger)

	// Verify default logger is updated
	logger := FromContext(context.Background())
	assert.Equal(t, customLogger, logger)
	assert.Equal(t, customLogger, defaultLogger)

	// Restore original default
	SetDefault(originalDefault)
}

// Logger tests

func TestNew(t *testing.T) {
	cfg := &Config{
		Level:   "info",
		Format:  "json",
		Service: "test-service",
		Version: "1.0.0",
	}

	logger := New(cfg)
	assert.NotNil(t, logger)
}

func TestNewWithWriter_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:   "info",
		Format:  "json",
		Service: "test-service",
		Version: "1.0.0",
	}

	logger := NewWithWriter(cfg, &buf)
	require.NotNil(t, logger)

	logger.Info("test message", slog.String("key", "value"))

	output := buf.String()
	assert.Contains(t, output, "test message")
	assert.Contains(t, output, "test-service")
	assert.Contains(t, output, "1.0.0")

	// Verify it's valid JSON
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "test message", logEntry["msg"])
	assert.Equal(t, "test-service", logEntry["service_name"])
	assert.Equal(t, "1.0.0", logEntry["service_version"])
}

func TestNewWithWriter_TextFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:   "debug",
		Format:  "text",
		Service: "test-service",
		Version: "1.0.0",
	}

	logger := NewWithWriter(cfg, &buf)
	require.NotNil(t, logger)

	logger.Debug("debug message")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "test-service")
}

func TestNewWithWriter_PrettyFormat(t *testing.T) {
	var buf bytes.Buffer
	cfg := &Config{
		Level:   "info",
		Format:  "pretty",
		Service: "test-service",
		Version: "1.0.0",
	}

	logger := NewWithWriter(cfg, &buf)
	require.NotNil(t, logger)

	logger.Info("pretty message")

	output := buf.String()
	assert.Contains(t, output, "pretty message")
}

func TestNewWithWriter_WithFileConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	var buf bytes.Buffer
	cfg := &Config{
		Level:   "info",
		Format:  "json",
		Service: "test-service",
		Version: "1.0.0",
		File: FileConfig{
			Enabled:    true,
			Path:       logFile,
			MaxSizeMB:  1,
			MaxBackups: 3,
			MaxAgeDays: 7,
			Compress:   true,
		},
	}

	logger := NewWithWriter(cfg, &buf)
	require.NotNil(t, logger)

	logger.Info("test message to file")

	// Verify message went to the buffer (terminal)
	output := buf.String()
	assert.Contains(t, output, "test message to file")

	// Verify log file was created
	assert.FileExists(t, logFile)

	// Verify content was written to file
	content, err := os.ReadFile(logFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test message to file")
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{
			name:     "trace level",
			input:    "trace",
			expected: LevelTrace,
		},
		{
			name:     "debug level",
			input:    "debug",
			expected: slog.LevelDebug,
		},
		{
			name:     "info level",
			input:    "info",
			expected: slog.LevelInfo,
		},
		{
			name:     "warn level",
			input:    "warn",
			expected: slog.LevelWarn,
		},
		{
			name:     "warning level",
			input:    "warning",
			expected: slog.LevelWarn,
		},
		{
			name:     "error level",
			input:    "error",
			expected: slog.LevelError,
		},
		{
			name:     "unknown level defaults to info",
			input:    "unknown",
			expected: slog.LevelInfo,
		},
		{
			name:     "empty string defaults to info",
			input:    "",
			expected: slog.LevelInfo,
		},
		{
			name:     "case insensitive DEBUG",
			input:    "DEBUG",
			expected: slog.LevelDebug,
		},
		{
			name:     "case insensitive ERROR",
			input:    "ERROR",
			expected: slog.LevelError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSlogToCharmLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    slog.Level
		expected log.Level
	}{
		{
			name:     "trace maps to debug",
			input:    LevelTrace,
			expected: log.DebugLevel,
		},
		{
			name:     "debug level",
			input:    slog.LevelDebug,
			expected: log.DebugLevel,
		},
		{
			name:     "info level",
			input:    slog.LevelInfo,
			expected: log.InfoLevel,
		},
		{
			name:     "warn level",
			input:    slog.LevelWarn,
			expected: log.WarnLevel,
		},
		{
			name:     "error level",
			input:    slog.LevelError,
			expected: log.ErrorLevel,
		},
		{
			name:     "very low level maps to debug",
			input:    slog.Level(-12),
			expected: log.DebugLevel,
		},
		{
			name:     "very high level maps to error",
			input:    slog.Level(12),
			expected: log.ErrorLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := slogToCharmLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MultiHandler tests

func TestNewMultiHandler(t *testing.T) {
	handler1 := slog.NewTextHandler(io.Discard, nil)
	handler2 := slog.NewJSONHandler(io.Discard, nil)

	multi := NewMultiHandler(handler1, handler2)
	assert.NotNil(t, multi)
	assert.Len(t, multi.handlers, 2)
}

func TestMultiHandler_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		handlers []slog.Handler
		level    slog.Level
		expected bool
	}{
		{
			name: "true if any handler enabled",
			handlers: []slog.Handler{
				slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}),
				slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}),
			},
			level:    slog.LevelInfo,
			expected: true, // debug handler accepts info
		},
		{
			name: "false if no handler enabled",
			handlers: []slog.Handler{
				slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}),
				slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}),
			},
			level:    slog.LevelInfo,
			expected: false, // both require error level
		},
		{
			name: "true if all handlers enabled",
			handlers: []slog.Handler{
				slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}),
				slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}),
			},
			level:    slog.LevelWarn,
			expected: true, // both accept warn
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			multi := NewMultiHandler(tt.handlers...)
			result := multi.Enabled(context.Background(), tt.level)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMultiHandler_Handle(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	handler1 := slog.NewJSONHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelDebug})
	handler2 := slog.NewJSONHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})

	multi := NewMultiHandler(handler1, handler2)
	logger := slog.New(multi)

	// Log at info level - both handlers should receive it
	logger.Info("test message", slog.String("key", "value"))

	// Both buffers should contain the message
	assert.Contains(t, buf1.String(), "test message")
	assert.Contains(t, buf2.String(), "test message")

	buf1.Reset()
	buf2.Reset()

	// Log at debug level - only handler1 should receive it
	logger.Debug("debug message")

	assert.Contains(t, buf1.String(), "debug message")
	assert.Empty(t, buf2.String()) // handler2 requires info level
}

func TestMultiHandler_WithAttrs(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	handler1 := slog.NewJSONHandler(&buf1, nil)
	handler2 := slog.NewJSONHandler(&buf2, nil)

	multi := NewMultiHandler(handler1, handler2)
	attrs := []slog.Attr{
		slog.String("attr1", "value1"),
		slog.String("attr2", "value2"),
	}

	newMulti := multi.WithAttrs(attrs)
	assert.NotNil(t, newMulti)

	logger := slog.New(newMulti)
	logger.Info("test message")

	// Both buffers should contain the attributes
	output1 := buf1.String()
	output2 := buf2.String()

	assert.Contains(t, output1, "attr1")
	assert.Contains(t, output1, "value1")
	assert.Contains(t, output2, "attr1")
	assert.Contains(t, output2, "value1")
}

func TestMultiHandler_WithGroup(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	handler1 := slog.NewJSONHandler(&buf1, nil)
	handler2 := slog.NewJSONHandler(&buf2, nil)

	multi := NewMultiHandler(handler1, handler2)
	newMulti := multi.WithGroup("mygroup")
	assert.NotNil(t, newMulti)

	logger := slog.New(newMulti)
	logger.Info("test message", slog.String("key", "value"))

	// Both buffers should contain the group
	output1 := buf1.String()
	output2 := buf2.String()

	assert.Contains(t, output1, "mygroup")
	assert.Contains(t, output2, "mygroup")
}

// Redact tests

func TestDefaultRedactOptions(t *testing.T) {
	opts := DefaultRedactOptions()
	assert.NotEmpty(t, opts)
	assert.Greater(t, len(opts), 10, "should have multiple redaction options")
}

func TestNewReplaceAttr(t *testing.T) {
	tests := []struct {
		name         string
		fieldName    string
		fieldValue   string
		shouldRedact bool
	}{
		{
			name:         "redact password",
			fieldName:    "password",
			fieldValue:   "secret123",
			shouldRedact: true,
		},
		{
			name:         "redact token",
			fieldName:    "token",
			fieldValue:   "my-secret-token",
			shouldRedact: true,
		},
		{
			name:         "redact apiKey",
			fieldName:    "apiKey",
			fieldValue:   "api-key-value",
			shouldRedact: true,
		},
		{
			name:         "redact api_key",
			fieldName:    "api_key",
			fieldValue:   "api-key-value",
			shouldRedact: true,
		},
		{
			name:         "redact accessToken",
			fieldName:    "accessToken",
			fieldValue:   "access-token-value",
			shouldRedact: true,
		},
		{
			name:         "redact authorization",
			fieldName:    "authorization",
			fieldValue:   "Bearer token123",
			shouldRedact: true,
		},
		{
			name:         "redact privateKey",
			fieldName:    "privateKey",
			fieldValue:   "private-key-data",
			shouldRedact: true,
		},
		{
			name:         "redact secretKey",
			fieldName:    "secretKey",
			fieldValue:   "secret-key-data",
			shouldRedact: true,
		},
		{
			name:         "do not redact normal field",
			fieldName:    "username",
			fieldValue:   "john.doe",
			shouldRedact: false,
		},
		{
			name:         "do not redact message",
			fieldName:    "msg",
			fieldValue:   "this is a message",
			shouldRedact: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			replaceAttr := NewReplaceAttr()
			handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: replaceAttr})
			logger := slog.New(handler)

			logger.Info("test", slog.String(tt.fieldName, tt.fieldValue))

			output := buf.String()
			if tt.shouldRedact {
				assert.NotContains(t, output, tt.fieldValue, "sensitive value should be redacted")
				assert.Contains(t, output, tt.fieldName, "field name should be present")
				// Check for redaction marker
				assert.True(t,
					strings.Contains(output, "REDACTED") || strings.Contains(output, "***"),
					"output should contain redaction marker",
				)
			} else {
				assert.Contains(t, output, tt.fieldValue, "non-sensitive value should not be redacted")
			}
		})
	}
}

func TestNewReplaceAttr_JWTPattern(t *testing.T) {
	var buf bytes.Buffer
	replaceAttr := NewReplaceAttr()
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: replaceAttr})
	logger := slog.New(handler)

	// JWT-like token
	jwtToken := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"

	logger.Info("test", slog.String("authorization", jwtToken))

	output := buf.String()
	assert.NotContains(t, output, jwtToken, "JWT token should be redacted")
	assert.Contains(t, output, "authorization", "field name should be present")
}

func TestNewReplaceAttr_BearerPattern(t *testing.T) {
	var buf bytes.Buffer
	replaceAttr := NewReplaceAttr()
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: replaceAttr})
	logger := slog.New(handler)

	bearerToken := "Bearer abc123xyz456"

	logger.Info("test", slog.String("auth", bearerToken))

	output := buf.String()
	assert.NotContains(t, output, "abc123xyz456", "bearer token value should be redacted")
	assert.Contains(t, output, "auth", "field name should be present")
}

func TestNewReplaceAttr_CustomOptions(t *testing.T) {
	var buf bytes.Buffer

	// Import masq to use WithFieldName
	// Add custom field to redact
	replaceAttr := NewReplaceAttr()
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: replaceAttr})
	logger := slog.New(handler)

	// Test with a field that has 'secret' prefix (covered by WithFieldPrefix)
	logger.Info("test", slog.String("secret_config", "sensitive-data"))

	output := buf.String()
	assert.NotContains(t, output, "sensitive-data", "field with secret prefix should be redacted")
	assert.Contains(t, output, "secret_config", "field name should be present")
}

// Integration test combining context and redaction

func TestContextWithRedaction(t *testing.T) {
	var buf bytes.Buffer
	replaceAttr := NewReplaceAttr()
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{ReplaceAttr: replaceAttr})
	logger := slog.New(handler)

	ctx := WithContext(context.Background(), logger)
	ctx = WithRequestID(ctx, "req-integration-123")

	fromCtx := FromContext(ctx)
	fromCtx.Info("test message",
		slog.String("username", "john.doe"),
		slog.String("password", "super-secret"),
	)

	output := buf.String()

	// Should contain request ID and username
	assert.Contains(t, output, "req-integration-123")
	assert.Contains(t, output, "john.doe")

	// Should NOT contain password value
	assert.NotContains(t, output, "super-secret")
	assert.Contains(t, output, "password") // field name should be present
}

// Test multiple context IDs

func TestMultipleContextIDs(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, nil)
	logger := slog.New(handler)

	ctx := WithContext(context.Background(), logger)
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithTraceID(ctx, "trace-456")
	ctx = WithCorrelationID(ctx, "corr-789")

	fromCtx := FromContext(ctx)
	fromCtx.Info("test with all IDs")

	output := buf.String()

	// Verify all IDs are present
	assert.Contains(t, output, "req-123")
	assert.Contains(t, output, "trace-456")
	assert.Contains(t, output, "corr-789")
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, "trace_id")
	assert.Contains(t, output, "correlation_id")

	// Verify JSON structure
	var logEntry map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &logEntry)
	require.NoError(t, err)
	assert.Equal(t, "req-123", logEntry["request_id"])
	assert.Equal(t, "trace-456", logEntry["trace_id"])
	assert.Equal(t, "corr-789", logEntry["correlation_id"])
}
