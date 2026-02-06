# Adding Custom Metrics

This guide explains how to add custom OpenTelemetry metrics for observability.

## Overview

The template uses OpenTelemetry for metrics, which exports to Prometheus format at `/-/metrics`.
Built-in metrics include HTTP request duration and count; this guide covers adding
application-specific metrics.

---

## Metric Types

| Type              | Description                    | Use Case                       |
| ----------------- | ------------------------------ | ------------------------------ |
| **Counter**       | Monotonically increasing value | Request counts, errors, events |
| **UpDownCounter** | Can increase or decrease       | Active connections, queue size |
| **Histogram**     | Distribution of values         | Latencies, request sizes       |
| **Gauge**         | Point-in-time value            | Temperature, memory usage      |

---

## Step 1: Get the Meter

Get a meter from the global OpenTelemetry provider.

```go
package app

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/metric"
)

// Get a meter for your package
var meter = otel.Meter("github.com/jsamuelsen/go-service-template/internal/app")
```

---

## Step 2: Create Metric Instruments

### Counter Example

Track the number of orders processed:

```go
package app

import (
    "context"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("github.com/jsamuelsen/go-service-template/internal/app")

// Define metrics
var (
    ordersProcessed metric.Int64Counter
    orderValue      metric.Float64Histogram
)

func init() {
    var err error

    ordersProcessed, err = meter.Int64Counter(
        "app.orders.processed",
        metric.WithDescription("Number of orders processed"),
        metric.WithUnit("{order}"),
    )
    if err != nil {
        panic(err)
    }

    orderValue, err = meter.Float64Histogram(
        "app.orders.value",
        metric.WithDescription("Order value distribution"),
        metric.WithUnit("USD"),
    )
    if err != nil {
        panic(err)
    }
}

// OrderService with metrics
type OrderService struct {
    // ... other fields
}

func (s *OrderService) ProcessOrder(ctx context.Context, order *domain.Order) error {
    // Process the order...

    // Record metrics
    ordersProcessed.Add(ctx, 1,
        metric.WithAttributes(
            attribute.String("status", "success"),
            attribute.String("payment_method", order.PaymentMethod),
        ),
    )

    orderValue.Record(ctx, order.TotalAmount,
        metric.WithAttributes(
            attribute.String("currency", order.Currency),
        ),
    )

    return nil
}
```

### Histogram Example

Track operation latency:

```go
package app

import (
    "context"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("github.com/jsamuelsen/go-service-template/internal/app")

var paymentLatency metric.Float64Histogram

func init() {
    var err error
    paymentLatency, err = meter.Float64Histogram(
        "app.payment.latency",
        metric.WithDescription("Payment processing latency"),
        metric.WithUnit("s"),
    )
    if err != nil {
        panic(err)
    }
}

func (s *PaymentService) ProcessPayment(ctx context.Context, payment *domain.Payment) error {
    start := time.Now()

    // Process payment...
    err := s.client.Charge(ctx, payment)

    // Record latency
    duration := time.Since(start).Seconds()
    status := "success"
    if err != nil {
        status = "error"
    }

    paymentLatency.Record(ctx, duration,
        metric.WithAttributes(
            attribute.String("status", status),
            attribute.String("provider", payment.Provider),
        ),
    )

    return err
}
```

### UpDownCounter Example

Track active operations:

```go
var activeJobs metric.Int64UpDownCounter

func init() {
    var err error
    activeJobs, err = meter.Int64UpDownCounter(
        "app.jobs.active",
        metric.WithDescription("Number of active background jobs"),
        metric.WithUnit("{job}"),
    )
    if err != nil {
        panic(err)
    }
}

func (s *JobService) RunJob(ctx context.Context, job *domain.Job) error {
    // Increment when starting
    activeJobs.Add(ctx, 1,
        metric.WithAttributes(attribute.String("type", job.Type)),
    )

    // Decrement when done (use defer for safety)
    defer func() {
        activeJobs.Add(ctx, -1,
            metric.WithAttributes(attribute.String("type", job.Type)),
        )
    }()

    return s.execute(ctx, job)
}
```

---

## Step 3: Add Attributes (Labels)

Attributes add dimensions to metrics for filtering and grouping.

```go
// Good: Low cardinality attributes
metric.WithAttributes(
    attribute.String("status", "success"),      // Limited values: success, error
    attribute.String("method", "POST"),         // Limited values: GET, POST, etc.
    attribute.String("region", "us-east-1"),    // Limited values
)

// Bad: High cardinality attributes (avoid!)
metric.WithAttributes(
    attribute.String("user_id", userID),        // Millions of unique values
    attribute.String("request_id", requestID),  // Unique per request
    attribute.String("timestamp", time.Now().String()),
)
```

**Warning:** High cardinality attributes cause metrics explosion and performance issues.

---

## Step 4: Verify in Prometheus

After adding metrics, verify they appear at `/-/metrics`:

```bash
# Run the service
task run

# Check metrics endpoint
curl http://localhost:8080/-/metrics | grep app_orders
```

Expected output:

```text
# HELP app_orders_processed Number of orders processed
# TYPE app_orders_processed counter
app_orders_processed{payment_method="credit_card",status="success"} 42
app_orders_processed{payment_method="paypal",status="success"} 15
app_orders_processed{payment_method="credit_card",status="error"} 3
```

---

## Metric Naming Conventions

Follow OpenTelemetry semantic conventions:

| Pattern                            | Example                | Description      |
| ---------------------------------- | ---------------------- | ---------------- |
| `{namespace}.{entity}.{action}`    | `app.orders.processed` | Business metrics |
| `{component}.{operation}.duration` | `db.query.duration`    | Latency          |
| `{component}.{resource}.count`     | `db.connections.count` | Counts           |
| `{component}.{resource}.size`      | `queue.messages.size`  | Sizes            |

**Rules:**

- Use lowercase with dots as separators
- Use singular nouns (`order` not `orders`)
- Include unit in the metric definition, not the name

---

## Common Patterns

### Error Rate Tracking

```go
var (
    requestsTotal metric.Int64Counter
    errorsTotal   metric.Int64Counter
)

func init() {
    requestsTotal, _ = meter.Int64Counter("app.requests.total")
    errorsTotal, _ = meter.Int64Counter("app.errors.total")
}

func (s *Service) HandleRequest(ctx context.Context, req *Request) error {
    requestsTotal.Add(ctx, 1)

    err := s.process(ctx, req)
    if err != nil {
        errorsTotal.Add(ctx, 1,
            metric.WithAttributes(
                attribute.String("error_type", errorType(err)),
            ),
        )
        return err
    }

    return nil
}
```

### SLI Metrics

```go
var (
    sliLatency metric.Float64Histogram
    sliAvailability metric.Int64Counter
)

func init() {
    sliLatency, _ = meter.Float64Histogram(
        "sli.latency",
        metric.WithDescription("Latency SLI for critical path"),
        metric.WithUnit("s"),
    )
    sliAvailability, _ = meter.Int64Counter(
        "sli.availability",
        metric.WithDescription("Availability SLI events"),
    )
}

func (s *Service) CriticalOperation(ctx context.Context) error {
    start := time.Now()
    err := s.execute(ctx)
    duration := time.Since(start).Seconds()

    // Record latency SLI
    sliLatency.Record(ctx, duration)

    // Record availability SLI
    status := "success"
    if err != nil {
        status = "failure"
    }
    sliAvailability.Add(ctx, 1,
        metric.WithAttributes(attribute.String("status", status)),
    )

    return err
}
```

---

## Metrics in Handlers

Add handler-specific metrics:

**File:** `internal/adapters/http/handlers/metrics.go`

```go
package handlers

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/metric"
)

var meter = otel.Meter("github.com/jsamuelsen/go-service-template/internal/adapters/http/handlers")

var (
    apiRequestSize metric.Int64Histogram
)

func init() {
    var err error
    apiRequestSize, err = meter.Int64Histogram(
        "http.request.body.size",
        metric.WithDescription("HTTP request body size"),
        metric.WithUnit("By"),
    )
    if err != nil {
        panic(err)
    }
}

func (h *Handler) CreateOrder(c *gin.Context) {
    // Record request size
    apiRequestSize.Record(c.Request.Context(), c.Request.ContentLength,
        metric.WithAttributes(
            attribute.String("endpoint", "/api/v1/orders"),
        ),
    )

    // ... handler logic
}
```

---

## Checklist

- [ ] Get meter with appropriate scope name
- [ ] Choose correct metric type (Counter, Histogram, etc.)
- [ ] Add meaningful description and unit
- [ ] Use low-cardinality attributes only
- [ ] Follow naming conventions
- [ ] Verify metrics at `/-/metrics`
- [ ] Document metric purpose

---

## Related Documentation

- [Architecture: Observability](../ARCHITECTURE.md#observability)
- [OpenTelemetry Go Metrics](https://opentelemetry.io/docs/languages/go/instrumentation/#metrics)
