package clients

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/rand/v2"
	"net"
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/middleware"
	"github.com/jsamuelsen/go-service-template/internal/platform/config"
	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

const (
	// instrumentationName is used for OpenTelemetry tracer and meter.
	instrumentationName = "github.com/jsamuelsen/go-service-template/internal/adapters/clients"

	// httpStatusCategoryDivisor divides status code to get category (2xx, 4xx, 5xx).
	httpStatusCategoryDivisor = 100

	// backoffJitterFactor is the jitter percentage for backoff calculation (±25%).
	backoffJitterFactor = 0.25

	// defaultTimeout is the default request timeout if not configured.
	defaultTimeout = 30 * time.Second

	// transportMaxIdleConns is the maximum number of idle connections.
	transportMaxIdleConns = 100

	// transportMaxIdleConnsPerHost is the maximum idle connections per host.
	transportMaxIdleConnsPerHost = 10

	// transportIdleConnTimeout is the idle connection timeout.
	transportIdleConnTimeout = 90 * time.Second

	// jitterRangeMultiplier converts rand [0,1) to [-1,1) for symmetric jitter.
	jitterRangeMultiplier = 2
)

// Config configures an HTTP client instance.
type Config struct {
	// BaseURL is the base URL for all requests (e.g., "https://api.example.com").
	BaseURL string

	// ServiceName identifies the downstream service for logging and tracing.
	ServiceName string

	// Timeout is the per-attempt request timeout.
	// Total wall-clock time may exceed this value due to retries and backoff.
	Timeout time.Duration

	// Retry configures retry behavior.
	Retry config.RetryConfig

	// Circuit configures circuit breaker behavior.
	Circuit config.CircuitBreakerConfig

	// AuthFunc is an optional function to inject authentication into requests.
	// It is called for each request attempt (including retries).
	AuthFunc func(*http.Request)

	// Logger is an optional logger. If nil, a default logger is used.
	Logger *slog.Logger
}

// Client is an instrumented HTTP client for downstream services.
// It provides:
//   - Retry with exponential backoff and jitter
//   - Circuit breaker protection
//   - OpenTelemetry tracing and metrics
//   - Request/correlation ID propagation
//   - Structured logging
type Client struct {
	http        *http.Client
	baseURL     string
	serviceName string
	cfg         *Config
	logger      *slog.Logger
	cb          *CircuitBreaker

	tracer trace.Tracer
	meter  metric.Meter

	// Metrics
	requestDuration metric.Float64Histogram
	requestTotal    metric.Int64Counter
}

// New creates a new instrumented HTTP client.
func New(cfg *Config) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}

	if cfg.ServiceName == "" {
		return nil, errors.New("service name is required")
	}

	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}

	// Initialize circuit breaker
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		MaxFailures:   cfg.Circuit.MaxFailures,
		Timeout:       cfg.Circuit.Timeout,
		HalfOpenLimit: cfg.Circuit.HalfOpenLimit,
	})

	// Set up logger
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger = logger.With(
		slog.String("component", "clients.Client"),
		slog.String("downstream", cfg.ServiceName),
	)

	// Set up circuit breaker logging
	cb.OnStateChange(func(from, to State) {
		logger.Warn("circuit breaker state changed",
			slog.String("from", from.String()),
			slog.String("to", to.String()),
		)
	})

	// Initialize telemetry
	tracer := otel.Tracer(instrumentationName)
	meter := otel.Meter(instrumentationName)

	requestDuration, err := meter.Float64Histogram(
		"http.client.request.duration",
		metric.WithDescription("Duration of HTTP client requests"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating duration metric: %w", err)
	}

	requestTotal, err := meter.Int64Counter(
		"http.client.request.total",
		metric.WithDescription("Total number of HTTP client requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request counter: %w", err)
	}

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        transportMaxIdleConns,
			MaxIdleConnsPerHost: transportMaxIdleConnsPerHost,
			IdleConnTimeout:     transportIdleConnTimeout,
		},
	}

	return &Client{
		http:            httpClient,
		baseURL:         strings.TrimSuffix(cfg.BaseURL, "/"),
		serviceName:     cfg.ServiceName,
		cfg:             cfg,
		logger:          logger,
		cb:              cb,
		tracer:          tracer,
		meter:           meter,
		requestDuration: requestDuration,
		requestTotal:    requestTotal,
	}, nil
}

// Do executes an HTTP request with retry, circuit breaker, tracing, and logging.
//
// Note: Retry only works correctly for requests with no body (GET, DELETE) or requests
// where req.GetBody is set (allowing the body to be rewound). For POST/PUT with streaming
// bodies, ensure GetBody is set or limit MaxAttempts to 1.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	startTime := time.Now()
	logger := logging.FromContext(ctx).With(
		slog.String("downstream", c.serviceName),
		slog.String("method", req.Method),
		slog.String("path", req.URL.Path),
	)

	// Check circuit breaker
	if !c.cb.Allow() {
		c.recordMetrics(ctx, req.Method, 0, time.Since(startTime), "circuit_open")
		logger.Warn("request blocked by circuit breaker")
		return nil, ErrCircuitOpen
	}

	// Inject headers
	c.injectHeaders(ctx, req)

	// Create span
	ctx, span := c.tracer.Start(ctx, fmt.Sprintf("HTTP %s %s", req.Method, c.serviceName),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("http.method", req.Method),
			attribute.String("http.url", req.URL.String()),
			attribute.String("peer.service", c.serviceName),
		),
	)
	defer span.End()

	// Propagate trace context
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// Execute with retry
	resp, lastErr := c.executeWithRetry(ctx, req, logger, startTime)

	// Record result
	return c.recordResult(ctx, req, resp, lastErr, span, logger, startTime)
}

// executeWithRetry performs the HTTP request with retry logic.
func (c *Client) executeWithRetry(ctx context.Context, req *http.Request, logger *slog.Logger, startTime time.Time) (*http.Response, error) {
	var lastErr error
	var resp *http.Response

	for attempt := 0; attempt < c.cfg.Retry.MaxAttempts; attempt++ {
		if attempt > 0 {
			if err := c.waitForRetry(ctx, req, attempt, logger, startTime); err != nil {
				return nil, err
			}
		}

		resp, lastErr = c.http.Do(req.WithContext(ctx))

		if shouldRetry, err := c.handleAttemptResult(resp, lastErr, attempt, logger); shouldRetry {
			lastErr = err
			continue
		}

		if lastErr != nil {
			break
		}

		return resp, nil
	}

	return nil, lastErr
}

// waitForRetry waits for the backoff duration before retrying.
func (c *Client) waitForRetry(ctx context.Context, req *http.Request, attempt int, logger *slog.Logger, startTime time.Time) error {
	backoff := c.calculateBackoff(attempt)
	logger.Debug("retrying request",
		slog.Int("attempt", attempt+1),
		slog.Duration("backoff", backoff),
	)

	select {
	case <-ctx.Done():
		c.cb.RecordFailure()
		c.recordMetrics(ctx, req.Method, 0, time.Since(startTime), "context_canceled")
		return ctx.Err()
	case <-time.After(backoff):
	}

	// Re-inject auth on retry (token may have changed)
	if c.cfg.AuthFunc != nil {
		c.cfg.AuthFunc(req)
	}

	return nil
}

// handleAttemptResult checks the response and determines if retry is needed.
// Returns (shouldRetry, error).
func (c *Client) handleAttemptResult(resp *http.Response, err error, attempt int, logger *slog.Logger) (bool, error) {
	if err != nil {
		if isRetryableError(err) {
			logger.Debug("request failed with retryable error",
				slog.Int("attempt", attempt+1),
				slog.Any("error", err),
			)
			return true, err
		}
		return false, err
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		logger.Debug("request failed with server error",
			slog.Int("attempt", attempt+1),
			slog.Int("status", resp.StatusCode),
		)
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Debug("failed to close response body", slog.Any("error", closeErr))
		}
		return true, fmt.Errorf("server error: %d", resp.StatusCode)
	}

	return false, nil
}

// recordResult records the final result and updates metrics/circuit breaker.
func (c *Client) recordResult(ctx context.Context, req *http.Request, resp *http.Response, lastErr error, span trace.Span, logger *slog.Logger, startTime time.Time) (*http.Response, error) {
	duration := time.Since(startTime)

	if lastErr != nil {
		c.cb.RecordFailure()
		span.SetStatus(codes.Error, lastErr.Error())
		c.recordMetrics(ctx, req.Method, 0, duration, "error")
		logger.Error("request failed",
			slog.Duration("duration", duration),
			slog.Any("error", lastErr),
		)
		return nil, fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
	}

	c.cb.RecordSuccess()
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
	if resp.StatusCode >= http.StatusBadRequest {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
	}

	statusCategory := fmt.Sprintf("%dxx", resp.StatusCode/httpStatusCategoryDivisor)
	c.recordMetrics(ctx, req.Method, resp.StatusCode, duration, statusCategory)

	logger.Debug("request completed",
		slog.Int("status", resp.StatusCode),
		slog.Duration("duration", duration),
	)

	return resp, nil
}

// Get performs an HTTP GET request.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL(path), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	return c.Do(ctx, req)
}

// Post performs an HTTP POST request.
func (c *Client) Post(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.buildURL(path), body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return c.Do(ctx, req)
}

// Put performs an HTTP PUT request.
func (c *Client) Put(ctx context.Context, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.buildURL(path), body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	return c.Do(ctx, req)
}

// Delete performs an HTTP DELETE request.
func (c *Client) Delete(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.buildURL(path), http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	return c.Do(ctx, req)
}

// CircuitState returns the current state of the circuit breaker.
func (c *Client) CircuitState() State {
	return c.cb.State()
}

// injectHeaders adds request ID, correlation ID, and auth to the request.
func (c *Client) injectHeaders(ctx context.Context, req *http.Request) {
	// Propagate request ID
	if requestID := middleware.RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set(middleware.HeaderRequestID, requestID)
	}

	// Propagate correlation ID
	if correlationID := middleware.CorrelationIDFromContext(ctx); correlationID != "" {
		req.Header.Set(middleware.HeaderCorrelationID, correlationID)
	}

	// Inject auth if configured
	if c.cfg.AuthFunc != nil {
		c.cfg.AuthFunc(req)
	}
}

// buildURL constructs the full URL from base URL and path.
func (c *Client) buildURL(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return c.baseURL + path
}

// calculateBackoff returns the backoff duration for the given attempt.
// Uses exponential backoff with jitter.
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential: initial * multiplier^attempt
	backoff := float64(c.cfg.Retry.InitialInterval) * math.Pow(c.cfg.Retry.Multiplier, float64(attempt))

	// Cap at max interval
	if backoff > float64(c.cfg.Retry.MaxInterval) {
		backoff = float64(c.cfg.Retry.MaxInterval)
	}

	// Add jitter (±25%)
	jitterMultiplier := rand.Float64()*jitterRangeMultiplier - 1 //nolint:gosec // No need for crypto-grade randomness
	jitter := backoff * backoffJitterFactor * jitterMultiplier
	backoff += jitter

	return time.Duration(backoff)
}

// recordMetrics records request metrics.
func (c *Client) recordMetrics(ctx context.Context, method string, statusCode int, duration time.Duration, result string) {
	attrs := []attribute.KeyValue{
		attribute.String("http.method", method),
		attribute.String("peer.service", c.serviceName),
		attribute.String("result", result),
	}

	if statusCode > 0 {
		attrs = append(attrs, attribute.Int("http.status_code", statusCode))
	}

	c.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attrs...))
	c.requestTotal.Add(ctx, 1, metric.WithAttributes(attrs...))
}

// isRetryableError determines if an error is retryable.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are not retryable
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Network timeout errors are retryable
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	// Connection refused, reset, etc. are retryable
	var opErr *net.OpError

	return errors.As(err, &opErr)
}
