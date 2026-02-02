# Adding a Downstream Client

This guide walks through adding a new HTTP client to call an external service, using the Anti-Corruption Layer (ACL) pattern.

## Overview

Adding a downstream client involves:

1. **Ports** - Define the client interface
2. **ACL Adapter** - Implement the interface with external DTOs
3. **Health Check** - Implement health checking
4. **Wiring** - Create and register in main.go

---

## Step 1: Define Port Interface

Define the contract in the ports layer. This keeps the domain isolated from external API details.

**File:** `internal/ports/services.go`

```go
package ports

import (
    "context"

    "github.com/jsamuelsen/go-service-template/internal/domain"
)

// PaymentClient defines the contract for payment operations.
type PaymentClient interface {
    // ProcessPayment submits a payment for processing.
    ProcessPayment(ctx context.Context, amount int64, currency string) (*domain.Payment, error)

    // GetPayment retrieves a payment by ID.
    GetPayment(ctx context.Context, id string) (*domain.Payment, error)

    // RefundPayment initiates a refund for a payment.
    RefundPayment(ctx context.Context, paymentID string, amount int64) (*domain.Refund, error)
}
```

**Key points:**

- Context as first parameter
- Return domain types, not external DTOs
- Document each method

---

## Step 2: Create ACL Adapter

The ACL adapter translates between external API responses and domain types.

**File:** `internal/adapters/clients/acl/payment_client.go`

```go
// Package acl implements the Anti-Corruption Layer pattern for external services.
package acl

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "net/http"

    "github.com/jsamuelsen/go-service-template/internal/adapters/clients"
    "github.com/jsamuelsen/go-service-template/internal/domain"
)

// PaymentClientConfig contains configuration for the payment client.
type PaymentClientConfig struct {
    // Client is the HTTP client to use for requests.
    Client *clients.Client

    // Logger is the structured logger.
    Logger *slog.Logger
}

// PaymentClient implements ports.PaymentClient using an external payment API.
type PaymentClient struct {
    client *clients.Client
    logger *slog.Logger
}

// NewPaymentClient creates a new payment client adapter.
func NewPaymentClient(cfg PaymentClientConfig) *PaymentClient {
    if cfg.Client == nil {
        panic("PaymentClient: Client is required")
    }

    logger := cfg.Logger
    if logger == nil {
        logger = slog.Default()
    }

    return &PaymentClient{
        client: cfg.Client,
        logger: logger,
    }
}

// External DTOs - these are private (unexported) and never leak outside the ACL.
type externalPaymentResponse struct {
    PaymentID string `json:"payment_id"`
    Amount    int64  `json:"amount"`
    Currency  string `json:"currency"`
    Status    string `json:"status"`
    CreatedAt string `json:"created_at"`
}

// ProcessPayment submits a payment for processing.
// Implements ports.PaymentClient.
func (c *PaymentClient) ProcessPayment(ctx context.Context, amount int64, currency string) (*domain.Payment, error) {
    c.logger.DebugContext(ctx, "processing payment",
        slog.Int64("amount", amount),
        slog.String("currency", currency),
    )

    // Create request body
    body := map[string]any{
        "amount":   amount,
        "currency": currency,
    }

    resp, err := c.client.Post(ctx, "/v1/payments", body)
    if err != nil {
        return nil, domain.NewUnavailableError("payment-service", err.Error())
    }
    defer func() { _ = resp.Body.Close() }()

    if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
        return nil, c.handleErrorResponse(resp)
    }

    return c.parsePaymentResponse(resp.Body)
}

// GetPayment retrieves a payment by ID.
// Implements ports.PaymentClient.
func (c *PaymentClient) GetPayment(ctx context.Context, id string) (*domain.Payment, error) {
    c.logger.DebugContext(ctx, "getting payment", slog.String("payment_id", id))

    resp, err := c.client.Get(ctx, "/v1/payments/"+id)
    if err != nil {
        return nil, domain.NewUnavailableError("payment-service", err.Error())
    }
    defer func() { _ = resp.Body.Close() }()

    if resp.StatusCode == http.StatusNotFound {
        return nil, domain.NewNotFoundError("payment", id)
    }

    if resp.StatusCode != http.StatusOK {
        return nil, c.handleErrorResponse(resp)
    }

    return c.parsePaymentResponse(resp.Body)
}

// parsePaymentResponse reads and translates the external DTO to a domain Payment.
func (c *PaymentClient) parsePaymentResponse(body io.Reader) (*domain.Payment, error) {
    var external externalPaymentResponse

    if err := json.NewDecoder(body).Decode(&external); err != nil {
        return nil, fmt.Errorf("decoding payment response: %w", err)
    }

    return c.translateToDomain(&external), nil
}

// translateToDomain converts the external API response to a domain Payment.
// This isolates the domain from external API changes.
func (c *PaymentClient) translateToDomain(ext *externalPaymentResponse) *domain.Payment {
    return &domain.Payment{
        ID:       ext.PaymentID,
        Amount:   ext.Amount,
        Currency: ext.Currency,
        Status:   domain.PaymentStatus(ext.Status),
    }
}

// handleErrorResponse converts HTTP error responses to domain errors.
func (c *PaymentClient) handleErrorResponse(resp *http.Response) error {
    body, _ := io.ReadAll(resp.Body)

    c.logger.Warn("payment API error",
        slog.Int("status_code", resp.StatusCode),
        slog.String("body", string(body)),
    )

    switch resp.StatusCode {
    case http.StatusNotFound:
        return domain.ErrNotFound
    case http.StatusBadRequest, http.StatusUnprocessableEntity:
        return domain.NewValidationError(string(body))
    case http.StatusConflict:
        return domain.ErrConflict
    case http.StatusUnauthorized, http.StatusForbidden:
        return domain.ErrForbidden
    default:
        return domain.NewUnavailableError("payment-service", fmt.Sprintf("HTTP %d", resp.StatusCode))
    }
}
```

**Key points:**

- External DTOs are unexported (private)
- `translateToDomain()` isolates the domain from API changes
- Map HTTP status codes to domain errors
- Log errors with context

---

## Step 3: Implement Health Check

Add the `HealthChecker` interface to your client.

**File:** `internal/adapters/clients/acl/payment_client.go` (continued)

```go
// Name returns the health check name for this client.
// Implements ports.HealthChecker.
func (c *PaymentClient) Name() string {
    return "payment-service"
}

// Check performs a health check by calling the API's health endpoint.
// Implements ports.HealthChecker.
func (c *PaymentClient) Check(ctx context.Context) error {
    resp, err := c.client.Get(ctx, "/health")
    if err != nil {
        return err
    }
    defer func() { _ = resp.Body.Close() }()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("payment API returned status %d", resp.StatusCode)
    }

    return nil
}
```

**Key points:**

- Return a unique name for the health check
- Use a lightweight endpoint (health/ping)
- Respect context cancellation

---

## Step 4: Wire in main.go

Create the HTTP client, ACL adapter, and register health checks.

**File:** `cmd/service/main.go`

```go
func run() error {
    // ... existing initialization ...

    // Create HTTP client for payment service
    paymentHTTPClient, err := clients.New(&clients.Config{
        BaseURL:     cfg.PaymentAPI.BaseURL,  // e.g., "https://api.stripe.com"
        ServiceName: "payment-service",
        Timeout:     cfg.Client.Timeout,
        Retry:       cfg.Client.Retry,
        Circuit:     cfg.Client.CircuitBreaker,
        Logger:      logger,
        AuthFunc: func(req *http.Request) {
            // Add API key authentication
            req.Header.Set("Authorization", "Bearer "+cfg.PaymentAPI.APIKey)
        },
    })
    if err != nil {
        return fmt.Errorf("creating payment HTTP client: %w", err)
    }

    // Create payment client adapter (ACL pattern)
    paymentClient := acl.NewPaymentClient(acl.PaymentClientConfig{
        Client: paymentHTTPClient,
        Logger: logger,
    })

    // Register as health checker
    if err := healthRegistry.Register(paymentClient); err != nil {
        return fmt.Errorf("registering payment client health check: %w", err)
    }

    // Create service that uses the payment client
    orderService := app.NewOrderService(app.OrderServiceConfig{
        PaymentClient: paymentClient,
        Logger:        logger,
    })

    // ... rest of startup ...
}
```

---

## Client Configuration

The base HTTP client supports these configuration options:

| Parameter               | Description              | Default  |
| ----------------------- | ------------------------ | -------- |
| `BaseURL`               | External API base URL    | Required |
| `ServiceName`           | Name for metrics/logging | Required |
| `Timeout`               | Per-request timeout      | 30s      |
| `Retry.MaxAttempts`     | Maximum retry attempts   | 3        |
| `Retry.InitialInterval` | Initial backoff delay    | 100ms    |
| `Retry.MaxInterval`     | Maximum backoff delay    | 5s       |
| `Retry.Multiplier`      | Backoff multiplier       | 2.0      |
| `Circuit.MaxFailures`   | Failures to open circuit | 5        |
| `Circuit.Timeout`       | Time before half-open    | 30s      |
| `Circuit.HalfOpenLimit` | Successes to close       | 3        |
| `AuthFunc`              | Auth header injection    | nil      |

---

## Error Mapping Reference

| HTTP Status  | Domain Error            | Usage                   |
| ------------ | ----------------------- | ----------------------- |
| 404          | `domain.ErrNotFound`    | Resource doesn't exist  |
| 400, 422     | `domain.ErrValidation`  | Invalid input           |
| 409          | `domain.ErrConflict`    | Concurrent modification |
| 401, 403     | `domain.ErrForbidden`   | Auth failure            |
| 5xx, Network | `domain.ErrUnavailable` | Service unavailable     |

---

## Checklist

- [ ] Port interface defined in `internal/ports/`
- [ ] ACL adapter created in `internal/adapters/clients/acl/`
- [ ] External DTOs are unexported (private)
- [ ] `translateToDomain()` function converts to domain types
- [ ] HTTP errors mapped to domain errors
- [ ] `HealthChecker` interface implemented
- [ ] HTTP client created in main.go
- [ ] Registered with health registry
- [ ] Mocks generated for testing (`task generate`)

---

## Related Documentation

- [Architecture: ACL Pattern](../ARCHITECTURE.md#error-translation-acl)
- [Architecture: Circuit Breaker](../ARCHITECTURE.md#circuit-breaker-states)
- [Adding a Health Check](./adding-health-check.md)
