// Package logging provides structured logging using Go's slog package.
package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// Config holds logging configuration.
type Config struct {
	Level   string // debug, info, warn, error
	Format  string // json, text
	Service string // service name for default attrs
	Version string // service version for default attrs
}

// New creates a new configured slog.Logger.
// Uses JSON handler for non-local environments, text handler for local.
func New(cfg Config) *slog.Logger {
	return NewWithWriter(cfg, os.Stdout)
}

// NewWithWriter creates a new configured slog.Logger with a custom writer.
// Includes secret redaction by default. See docs/SECRET_REDACTION.md for details.
func NewWithWriter(cfg Config, w io.Writer) *slog.Logger {
	level := parseLevel(cfg.Level)
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: NewReplaceAttr(),
	}

	var handler slog.Handler
	if strings.EqualFold(cfg.Format, "text") {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}

	// Add default attributes
	logger := slog.New(handler).With(
		slog.String("service_name", cfg.Service),
		slog.String("service_version", cfg.Version),
	)

	return logger
}

// parseLevel converts a string log level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
