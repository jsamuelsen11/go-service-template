# Logging Strategy

This document describes the logging strategy for this service template, including log levels,
when to use each level, structured logging patterns, and configuration options.

## Overview

The service uses Go's standard `log/slog` package (Go 1.21+) with:

- **Custom TRACE level** for verbose debugging of external calls
- **Secret redaction** via [masq](https://github.com/shogo82148/go-masq) (automatic)
- **Rolling file logs** via [lumberjack](https://github.com/natefinch/lumberjack)
- **Pretty terminal output** via [charmbracelet/log](https://github.com/charmbracelet/log)
- **Context-aware logging** with request/correlation/trace IDs

## Log Levels

| Level   | Value | slog Constant        | When to Use                                                    |
| ------- | ----- | -------------------- | -------------------------------------------------------------- |
| `TRACE` | -8    | `logging.LevelTrace` | External call lifecycle, DTO translation, wire-level debugging |
| `DEBUG` | -4    | `slog.LevelDebug`    | Decision points, cache hits/misses, feature flag evaluation    |
| `INFO`  | 0     | `slog.LevelInfo`     | Business events, request start/complete, service lifecycle     |
| `WARN`  | 4     | `slog.LevelWarn`     | Recoverable issues, circuit breaker changes, retries           |
| `ERROR` | 8     | `slog.LevelError`    | Operation failures, unrecoverable errors                       |

### TRACE - Verbose External Call Debugging

Use TRACE for detailed debugging of external service interactions.
This level is below DEBUG and is only enabled when explicitly configured.

**What to log:**

- Entry/exit of external API calls with path and entity identifiers
- HTTP status codes from downstream services
- DTO translation details (field values, never secrets)
- Wire-level timing information

**Example (ACL client):**

```go
// Entry point
c.logger.Log(ctx, logging.LevelTrace, "starting request",
    slog.String("path", path),
    slog.String("quote_id", id))

// After response
c.logger.Log(ctx, logging.LevelTrace, "request complete",
    slog.String("path", path),
    slog.Int("status", resp.StatusCode))

// After DTO translation
c.logger.Log(ctx, logging.LevelTrace, "translated external DTO to domain",
    slog.String("quote_id", quote.ID),
    slog.String("author", quote.Author))
```

### DEBUG - Development & Troubleshooting

Use DEBUG for information helpful during development or troubleshooting production issues.

**What to log:**

- Feature flag evaluation results
- Cache hits/misses
- Conditional branch decisions
- Query parameters (non-sensitive)
- Internal state useful for debugging

**Example:**

```go
c.logger.DebugContext(ctx, "fetching quote by ID",
    slog.String("quote_id", id))

logger.Debug("feature flag evaluated",
    slog.String("flag", "use-new-algorithm"),
    slog.Bool("enabled", enabled))
```

### INFO - Normal Operations

Use INFO for normal operational messages that indicate the system is working correctly.

**What to log:**

- Request received/completed (handled automatically by middleware)
- Business operation success ("quote fetched", "order created")
- Service startup/shutdown
- Configuration loaded (non-sensitive values only)

**Example:**

```go
s.logger.InfoContext(ctx, "fetching random quote")

logger.Info("server started",
    slog.String("address", addr),
    slog.String("environment", env))
```

### WARN - Attention Needed

Use WARN for situations that may require attention but don't prevent the system from functioning.

**What to log:**

- Circuit breaker state changes (closed to open, open to half-open)
- Retry attempts with backoff duration
- Degraded service responses
- Deprecated feature usage
- Recoverable errors that succeed after retry

**Example:**

```go
logger.Warn("circuit breaker state changed",
    slog.String("from", from.String()),
    slog.String("to", to.String()))

logger.Debug("retrying request",
    slog.Int("attempt", attempt+1),
    slog.Duration("backoff", backoff))
```

### ERROR - Action Required

Use ERROR for failures that impact user experience or require investigation.

**What to log:**

- Failed operations that could not be recovered
- Unhandled exceptions (recovered panics)
- Dependency failures after all retries exhausted
- Data integrity issues
- Security-relevant failures

**Example:**

```go
s.logger.ErrorContext(ctx, "failed to fetch random quote",
    slog.Any("error", err))

ctxLogger.Error("panic recovered",
    slog.Any("error", r),
    slog.String("stack", string(stack)),
    slog.String("path", c.Request.URL.Path))
```

## What NOT to Log

- **Secrets**: Passwords, tokens, API keys (masq redaction catches common patterns, but don't log them intentionally)
- **Full request/response bodies**: Use TRACE sparingly for wire-level debugging only
- **PII without explicit need**: Don't log personal information unless required for debugging
- **High-cardinality IDs at INFO level**: Use DEBUG for detailed entity IDs
- **Health check requests**: Automatically skipped by middleware (paths starting with `/-/`)

## Layer-Specific Guidelines

| Layer                    | What to Log                                  | Typical Level |
| ------------------------ | -------------------------------------------- | ------------- |
| **HTTP Handlers**        | Request start/complete, validation errors    | INFO/WARN     |
| **Application Services** | Business events, orchestration decisions     | INFO/DEBUG    |
| **ACL Clients**          | External call lifecycle, DTO translations    | TRACE/DEBUG   |
| **Domain Layer**         | Nothing (pure logic, no infrastructure deps) | -             |
| **Infrastructure**       | Connection events, circuit breaker state     | WARN/ERROR    |

### Domain Layer Exception

The domain layer (`internal/domain/`) should contain **no logging**.
Domain services are pure business logic with no infrastructure dependencies. This:

- Keeps domain logic testable without mocks
- Prevents log noise from frequently-called business rules
- Follows DDD principles of infrastructure-agnostic domains

If you need to observe domain logic, do so from the application layer that calls it.

## Structured Logging Patterns

### Standard Fields (Automatic)

The logging middleware automatically adds these fields to all logs:

| Field             | Source              | Purpose                                     |
| ----------------- | ------------------- | ------------------------------------------- |
| `service_name`    | Config              | Identifies the service                      |
| `service_version` | Config              | Tracks deployment version                   |
| `request_id`      | Middleware          | Unique identifier for this request          |
| `correlation_id`  | Header or generated | Tracks business transaction across services |
| `trace_id`        | OpenTelemetry       | Links to distributed traces                 |

### Context-Aware Logging

Always use context-aware logging to include request metadata:

```go
// Get logger from context (includes request_id, correlation_id, trace_id)
logger := logging.FromContext(ctx)

// Log with context - fields automatically included
logger.InfoContext(ctx, "processing order",
    slog.String("order_id", orderID),
    slog.Int("item_count", len(items)))
```

### Status-Based Level Selection

HTTP middleware automatically selects log level based on response status:

```go
level := slog.LevelInfo
if status >= http.StatusInternalServerError {
    level = slog.LevelError
} else if status >= http.StatusBadRequest {
    level = slog.LevelWarn
}
```

This means:

- 2xx/3xx responses: INFO
- 4xx responses: WARN (client errors)
- 5xx responses: ERROR (server errors)

### Adding Context Fields

Enrich the logger with domain-specific fields:

```go
// In middleware - add request IDs
ctx = logging.WithRequestID(ctx, requestID)
ctx = logging.WithCorrelationID(ctx, correlationID)
ctx = logging.WithTraceID(ctx, traceID)

// In handlers/services - add domain context
logger := logging.FromContext(ctx).With(
    slog.String("user_id", userID),
    slog.String("tenant_id", tenantID),
)
```

## Configuration

### Base Configuration (`configs/base.yaml`)

```yaml
log:
  level: info # trace, debug, info, warn, error
  format: json # json, text, pretty
```

### Local Development (`configs/local.yaml`)

```yaml
log:
  level: debug # More verbose for development
  format: pretty # Colorful terminal output
  file:
    enabled: true
    path: ./logs/app.log
    max_size: 10 # MB before rotation
    max_backups: 2 # Number of old files to keep
    max_age: 7 # Days to retain old files
    compress: false # Don't compress in dev
```

### Production Recommendations

```yaml
log:
  level: info # Minimal noise
  format: json # Machine-parseable for aggregation
  file:
    enabled: false # Use stdout for container logs
```

For TRACE-level debugging in production (temporary):

```yaml
log:
  level: trace # Enable temporarily for debugging
```

## File Rotation

When `log.file.enabled: true`, logs are written to rolling files using lumberjack:

| Setting       | Description                        | Local | Production |
| ------------- | ---------------------------------- | ----- | ---------- |
| `max_size`    | Max file size (MB) before rotation | 10    | 100        |
| `max_backups` | Number of old files to retain      | 2     | 5          |
| `max_age`     | Days to retain old files           | 7     | 30         |
| `compress`    | Gzip old files                     | false | true       |

With `format: pretty` and file logging enabled, you get:

- **Terminal**: Colorful pretty-printed output
- **File**: Structured JSON logs (for log aggregation)

## Secret Redaction

Secrets are automatically redacted from logs using [masq](https://github.com/shogo82148/go-masq).
For comprehensive details, see [SECRET_REDACTION.md](./SECRET_REDACTION.md).

**Default redacted patterns:**

- Field names: `password`, `token`, `apiKey`, `secret`, `authorization`, `bearer`, `cookie`, `session`, `privateKey`, `credential`
- Field prefixes: `secret*`, `private*`
- Value patterns: JWT tokens (`eyJ...`), Bearer tokens, Basic auth

**Custom redaction:**

```go
// Add struct tag for custom types
type Config struct {
    APIKey string `masq:"secret"`
}
```

## Related Documentation

- [Architecture: Observability](./ARCHITECTURE.md#observability) - Tracing and metrics
- [Secret Redaction](./SECRET_REDACTION.md) - Comprehensive redaction guide
- [Troubleshooting](./TROUBLESHOOTING.md) - Common logging issues
