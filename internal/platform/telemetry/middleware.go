package telemetry

import (
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	instrumentationName = "github.com/jsamuelsen/go-service-template/telemetry"
)

// Metrics holds HTTP server metrics.
type Metrics struct {
	requestDuration metric.Float64Histogram
	requestTotal    metric.Int64Counter
	activeRequests  metric.Int64UpDownCounter
}

// NewMetrics creates HTTP server metrics.
func NewMetrics() (*Metrics, error) {
	meter := otel.Meter(instrumentationName)

	requestDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("HTTP request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	requestTotal, err := meter.Int64Counter(
		"http.server.request.total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	activeRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		requestDuration: requestDuration,
		requestTotal:    requestTotal,
		activeRequests:  activeRequests,
	}, nil
}

// Middleware returns Gin middleware for OpenTelemetry tracing and metrics.
// Uses otelgin for tracing and adds custom metrics and X-Trace-ID header.
func Middleware(serviceName string) gin.HandlerFunc {
	// Create metrics - errors are logged but don't prevent the middleware from working
	metrics, err := NewMetrics()
	if err != nil {
		otel.Handle(err)
	}

	return func(c *gin.Context) {
		start := time.Now()

		// Record active request
		if metrics != nil {
			attrs := []attribute.KeyValue{
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
			}

			metrics.activeRequests.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
			defer metrics.activeRequests.Add(c.Request.Context(), -1, metric.WithAttributes(attrs...))
		}

		// Process request
		c.Next()

		// Get trace ID from span and add to response header
		span := trace.SpanFromContext(c.Request.Context())
		if span.SpanContext().HasTraceID() {
			c.Header("X-Trace-ID", span.SpanContext().TraceID().String())
		}

		// Record metrics
		if metrics != nil {
			duration := time.Since(start).Seconds()
			attrs := []attribute.KeyValue{
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.route", c.FullPath()),
				attribute.Int("http.status_code", c.Writer.Status()),
			}
			metrics.requestDuration.Record(c.Request.Context(), duration, metric.WithAttributes(attrs...))
			metrics.requestTotal.Add(c.Request.Context(), 1, metric.WithAttributes(attrs...))
		}
	}
}

// TracingMiddleware returns just the otelgin tracing middleware.
// Use this if you want tracing without custom metrics.
func TracingMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
