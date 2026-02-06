// Package logging provides structured logging using Go's slog package.
package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"gopkg.in/natefinch/lumberjack.v2"
)

// logDirPerms is the permission mode for log directories.
const logDirPerms = 0o750

// LevelTrace is a custom log level below Debug for verbose tracing.
// slog.LevelDebug is -4, so Trace at -8 is more verbose.
const LevelTrace = slog.Level(-8)

// Config holds logging configuration.
type Config struct {
	Level   string     // debug, info, warn, error
	Format  string     // json, text, pretty
	Service string     // service name for default attrs
	Version string     // service version for default attrs
	File    FileConfig // rolling file configuration
}

// FileConfig holds rolling log file configuration.
type FileConfig struct {
	Enabled    bool
	Path       string
	MaxSizeMB  int // megabytes
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// New creates a new configured slog.Logger.
// For "pretty" format with file logging enabled, logs go to both:
// - Terminal: colorful pretty-printed output
// - File: structured JSON logs for aggregation
func New(cfg *Config) *slog.Logger {
	return NewWithWriter(cfg, os.Stdout)
}

// NewWithWriter creates a new configured slog.Logger with a custom writer.
// Includes secret redaction by default. See docs/SECRET_REDACTION.md for details.
func NewWithWriter(cfg *Config, w io.Writer) *slog.Logger {
	level := parseLevel(cfg.Level)

	// Create the terminal handler based on format
	var terminalHandler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "pretty":
		terminalHandler = newPrettyHandler(w, level)
	case "text":
		terminalHandler = newTextHandler(w, level)
	default: // "json"
		terminalHandler = newJSONHandler(w, level)
	}

	// If file logging is enabled, create a multi-handler
	var handler slog.Handler
	if cfg.File.Enabled && cfg.File.Path != "" {
		fileHandler := newFileHandler(cfg.File, level)
		handler = NewMultiHandler(terminalHandler, fileHandler)
	} else {
		handler = terminalHandler
	}

	// Add default attributes
	logger := slog.New(handler).With(
		slog.String("service_name", cfg.Service),
		slog.String("service_version", cfg.Version),
	)

	return logger
}

// newPrettyHandler creates a charmbracelet/log handler for colorful terminal output.
func newPrettyHandler(w io.Writer, level slog.Level) slog.Handler {
	charmLevel := slogToCharmLevel(level)

	logger := log.NewWithOptions(w, log.Options{
		ReportTimestamp: true,
		TimeFormat:      time.Kitchen,
		Level:           charmLevel,
		ReportCaller:    false,
	})

	return logger
}

// newTextHandler creates a standard slog text handler with redaction.
func newTextHandler(w io.Writer, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: NewReplaceAttr(),
	}
	return slog.NewTextHandler(w, opts)
}

// newJSONHandler creates a standard slog JSON handler with redaction.
// AddSource enables source file and line info for debugging.
func newJSONHandler(w io.Writer, level slog.Level) slog.Handler {
	opts := &slog.HandlerOptions{
		AddSource:   true,
		Level:       level,
		ReplaceAttr: NewReplaceAttr(),
	}
	return slog.NewJSONHandler(w, opts)
}

// newFileHandler creates a JSON handler writing to rolling log files.
func newFileHandler(cfg FileConfig, level slog.Level) slog.Handler {
	// Ensure log directory exists
	dir := filepath.Dir(cfg.Path)
	if dir != "" && dir != "." {
		_ = os.MkdirAll(dir, logDirPerms)
	}

	// Create lumberjack rolling writer
	roller := &lumberjack.Logger{
		Filename:   cfg.Path,
		MaxSize:    cfg.MaxSizeMB,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAgeDays,
		Compress:   cfg.Compress,
		LocalTime:  true,
	}

	opts := &slog.HandlerOptions{
		Level:       level,
		ReplaceAttr: NewReplaceAttr(),
	}

	// File logs are always JSON for structured log aggregation
	return slog.NewJSONHandler(roller, opts)
}

// slogToCharmLevel converts slog.Level to charmbracelet/log level.
// Note: charm/log doesn't have a trace level, so trace maps to debug.
func slogToCharmLevel(level slog.Level) log.Level {
	switch {
	case level <= LevelTrace:
		return log.DebugLevel // charm/log doesn't have trace, use debug
	case level <= slog.LevelDebug:
		return log.DebugLevel
	case level <= slog.LevelInfo:
		return log.InfoLevel
	case level <= slog.LevelWarn:
		return log.WarnLevel
	default:
		return log.ErrorLevel
	}
}

// parseLevel converts a string log level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return LevelTrace
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
