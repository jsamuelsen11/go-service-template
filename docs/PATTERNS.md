# Go Patterns Guide

This document covers idiomatic Go patterns used in this service template.

## Table of Contents

- [Concurrency](#concurrency)
- [Context Patterns](#context-patterns)
- [Circuit Breaker](#circuit-breaker)
- [Retry with Backoff](#retry-with-backoff)
- [Service Layer](#service-layer)
- [Error Handling](#error-handling)
- [Validation](#validation)
- [Health Checks](#health-checks)

---

## Concurrency

Use `sync.WaitGroup` for coordinating concurrent operations.

### Concurrent Operations with WaitGroup

Run multiple operations concurrently and wait for all to complete:

```go
func (r *DefaultHealthRegistry) CheckAll(ctx context.Context) *HealthResult {
    result := &HealthResult{
        Status: HealthStatusHealthy,
        Checks: make(map[string]*CheckResult),
    }

    var (
        wg sync.WaitGroup
        mu sync.Mutex
    )

    for _, checker := range checkers {
        wg.Add(1)

        go func(c HealthChecker) {
            defer wg.Done()

            start := time.Now()
            err := c.Check(ctx)
            duration := time.Since(start)

            checkResult := &CheckResult{
                Status:   HealthStatusHealthy,
                Duration: duration,
            }

            if err != nil {
                checkResult.Status = HealthStatusUnhealthy
                checkResult.Message = err.Error()
            }

            mu.Lock()
            result.Checks[c.Name()] = checkResult
            if checkResult.Status == HealthStatusUnhealthy {
                result.Status = HealthStatusUnhealthy
            }
            mu.Unlock()
        }(checker)
    }

    wg.Wait()
    return result
}
```

### Key Points

1. **Always call `wg.Add(1)` before starting the goroutine**
2. **Use `defer wg.Done()` at the start of the goroutine**
3. **Protect shared state with `sync.Mutex`**
4. **Pass loop variables as function parameters** to avoid closure issues

---

## Context Patterns

Context carries request-scoped values, deadlines, and cancellation signals.

### Type-Safe Context Keys

Use a custom type for context keys to avoid collisions:

```go
// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
    ctxKeyRequestID     contextKey = "request_id"
    ctxKeyCorrelationID contextKey = "correlation_id"
)
```

### Context Value Storage and Extraction

Store values with type-safe setters:

```go
// ContextWithRequestID stores a request ID in the context.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
    return context.WithValue(ctx, ctxKeyRequestID, id)
}
```

Extract values with safe getters:

```go
// RequestIDFromContext extracts the request ID from context.
// Returns empty string if not set or if ctx is nil.
func RequestIDFromContext(ctx context.Context) string {
    if ctx == nil {
        return ""
    }

    if id, ok := ctx.Value(ctxKeyRequestID).(string); ok {
        return id
    }

    return ""
}
```

### Context Cancellation

Respect context cancellation in long-running operations:

```go
select {
case <-ctx.Done():
    return ctx.Err() // context.Canceled or context.DeadlineExceeded
case <-time.After(backoff):
    // Continue with operation
}
```

### Two-Phase Request Context Pattern

For orchestration services that need to:

1. Fetch data lazily with request-scoped in-memory memoization
2. Stage multiple write operations for atomic execution

See `/internal/app/context/` for the implementation.

#### Phase 1: Lazy Memoization with GetOrFetch

Uses an in-memory cache that lives only for the duration of the request:

```go
import appctx "github.com/jsamuelsen/go-service-template/internal/app/context"

func (s *Service) ProcessOrder(ctx context.Context, orderID string) error {
    rc := appctx.New(ctx)

    // First call fetches from service
    order, err := rc.GetOrFetch("order:"+orderID, func(ctx context.Context) (any, error) {
        return s.orderClient.GetByID(ctx, orderID)
    })

    // Subsequent calls return cached value - no duplicate API calls
    order, _ = rc.GetOrFetch("order:"+orderID, fetchFn)
}
```

#### Phase 2: Staged Writes with Rollback

```go
// Stage actions
_ = rc.AddAction(&UpdateInventoryAction{OrderID: orderID, Items: items})
_ = rc.AddAction(&ChargePaymentAction{Amount: total})
_ = rc.AddAction(&SendConfirmationAction{Email: user.Email})

// Execute all or rollback on failure
if err := rc.Commit(ctx); err != nil {
    // Failed action and all preceding actions are rolled back
    return err
}
```

#### When to Use

| Use Case                                | Pattern                              |
| --------------------------------------- | ------------------------------------ |
| Multiple API calls with shared data     | Phase 1 (GetOrFetch)                 |
| Coordinated writes to multiple services | Phase 2 (AddAction/Commit)           |
| Simple CRUD operations                  | Standard context (skip this pattern) |

---

## Circuit Breaker

The circuit breaker pattern prevents cascading failures by blocking requests to unhealthy services.

### State Machine

```text
    ┌─────────────────────────────────────────────┐
    │                                             │
    ▼                                             │
┌────────┐  MaxFailures  ┌────────┐  Timeout   ┌───────────┐
│ Closed │──────────────►│  Open  │───────────►│ Half-Open │
└────────┘               └────────┘            └───────────┘
    ▲                                             │    │
    │          HalfOpenLimit successes            │    │
    └─────────────────────────────────────────────┘    │
                                                       │
    ┌──────────────────────────────────────────────────┘
    │ Any failure
    ▼
┌────────┐
│  Open  │
└────────┘
```

### Implementation

```go
type CircuitBreaker struct {
    mu               sync.RWMutex
    state            State
    failures         int       // consecutive failures in closed state
    successes        int       // consecutive successes in half-open state
    lastFailure      time.Time
    cfg              CircuitBreakerConfig
    onStateChange    func(from, to State)
}

// Allow checks if a request should be allowed through.
func (cb *CircuitBreaker) Allow() bool {
    cb.mu.Lock()
    defer cb.mu.Unlock()

    switch cb.state {
    case StateClosed:
        return true

    case StateOpen:
        if time.Since(cb.lastFailure) >= cb.cfg.Timeout {
            cb.transitionTo(StateHalfOpen)
            return true
        }
        return false

    case StateHalfOpen:
        if cb.halfOpenRequests >= cb.cfg.HalfOpenLimit {
            return false
        }
        cb.halfOpenRequests++
        return true

    default:
        return false
    }
}
```

### State Change Callbacks

Register callbacks for logging or metrics:

```go
cb.OnStateChange(func(from, to State) {
    logger.Warn("circuit breaker state changed",
        slog.String("from", from.String()),
        slog.String("to", to.String()),
    )
})
```

---

## Retry with Backoff

Retry failed operations with exponential backoff and jitter.

### Exponential Backoff with Jitter

```go
// calculateBackoff returns the backoff duration for the given attempt.
func (c *Client) calculateBackoff(attempt int) time.Duration {
    // Exponential: initial * multiplier^attempt
    backoff := float64(c.cfg.Retry.InitialInterval) *
        math.Pow(c.cfg.Retry.Multiplier, float64(attempt))

    // Cap at max interval
    if backoff > float64(c.cfg.Retry.MaxInterval) {
        backoff = float64(c.cfg.Retry.MaxInterval)
    }

    // Add jitter (±25%) to prevent thundering herd
    jitter := backoff * 0.25 * (rand.Float64()*2 - 1)
    backoff += jitter

    return time.Duration(backoff)
}
```

### Retry Loop Pattern

```go
func (c *Client) executeWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
    var lastErr error

    for attempt := 0; attempt < c.cfg.Retry.MaxAttempts; attempt++ {
        if attempt > 0 {
            backoff := c.calculateBackoff(attempt)

            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            case <-time.After(backoff):
            }
        }

        resp, err := c.http.Do(req.WithContext(ctx))

        if err == nil && resp.StatusCode < 500 {
            return resp, nil
        }

        if !isRetryableError(err) && (resp == nil || resp.StatusCode < 500) {
            return resp, err
        }

        lastErr = err
    }

    return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
```

### Retryable Error Detection

```go
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
```

---

## Service Layer

Services are thin orchestrators that coordinate between domain logic and infrastructure.

### Service Structure with Config

```go
// QuoteService orchestrates quote-related use cases.
type QuoteService struct {
    quoteClient ports.QuoteClient
    logger      *slog.Logger
}

// QuoteServiceConfig contains configuration for the service.
type QuoteServiceConfig struct {
    QuoteClient ports.QuoteClient
    Logger      *slog.Logger
}

// NewQuoteService creates a new service with the provided dependencies.
// Panics if required dependencies are nil.
func NewQuoteService(cfg QuoteServiceConfig) *QuoteService {
    if cfg.QuoteClient == nil {
        panic("QuoteService: QuoteClient is required")
    }

    logger := cfg.Logger
    if logger == nil {
        logger = slog.Default()
    }

    return &QuoteService{
        quoteClient: cfg.QuoteClient,
        logger:      logger,
    }
}
```

### Use Case Method Pattern

Each public method represents a use case:

```go
func (s *QuoteService) GetRandomQuote(ctx context.Context) (*domain.Quote, error) {
    s.logger.InfoContext(ctx, "fetching random quote")

    quote, err := s.quoteClient.GetRandomQuote(ctx)
    if err != nil {
        s.logger.ErrorContext(ctx, "failed to fetch random quote",
            slog.Any("error", err),
        )
        return nil, err
    }

    s.logger.InfoContext(ctx, "fetched random quote",
        slog.String("quote_id", quote.ID),
    )

    return quote, nil
}
```

### Guidelines

1. **Keep services thin** - Business rules belong in domain, not services
2. **One responsibility** - Each method handles one use case
3. **Explicit dependencies** - Inject via constructor config, no globals
4. **Panic on required nil** - Fail fast during startup, not at runtime
5. **Default optional deps** - Use sensible defaults (e.g., `slog.Default()`)

---

## Error Handling

### Always Wrap Errors

Add context when propagating errors:

```go
user, err := repo.GetByID(ctx, id)
if err != nil {
    return nil, fmt.Errorf("getting user %s: %w", id, err)
}
```

### Domain Error Types

Define sentinel errors and typed errors for business failures:

```go
// Sentinel errors for use with errors.Is()
var (
    ErrNotFound   = errors.New("not found")
    ErrConflict   = errors.New("conflict")
    ErrValidation = errors.New("validation failed")
    ErrForbidden  = errors.New("forbidden")
)

// Typed error with context
type NotFoundError struct {
    Entity string
    ID     string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s with id %q not found", e.Entity, e.ID)
}

// Unwrap enables errors.Is() support
func (e *NotFoundError) Unwrap() error {
    return ErrNotFound
}

// Constructor function
func NewNotFoundError(entity, id string) error {
    return &NotFoundError{Entity: entity, ID: id}
}
```

### Error Checking

Use `errors.Is()` for sentinel errors:

```go
if errors.Is(err, domain.ErrNotFound) {
    // Handle not found
}
```

Use `errors.As()` for typed errors when you need the details:

```go
var notFoundErr *domain.NotFoundError
if errors.As(err, &notFoundErr) {
    log.Printf("Entity %s with ID %s not found", notFoundErr.Entity, notFoundErr.ID)
}
```

### Error Handling at Boundaries

Handle and log errors at service boundaries (HTTP handlers):

```go
func (h *Handler) GetUser(c *gin.Context) {
    user, err := h.service.GetUser(c.Request.Context(), id)
    if err != nil {
        switch {
        case errors.Is(err, domain.ErrNotFound):
            c.JSON(http.StatusNotFound, errorResponse("User not found"))
        case errors.Is(err, domain.ErrValidation):
            c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
        default:
            h.logger.Error("failed to get user", slog.Any("error", err))
            c.JSON(http.StatusInternalServerError, errorResponse("Internal error"))
        }
        return
    }

    c.JSON(http.StatusOK, user)
}
```

---

## Validation

Use struct tags for declarative validation with custom validators for business rules.

### Singleton Validator

```go
var (
    validate     *validator.Validate
    validateOnce sync.Once
)

func Validator() *validator.Validate {
    validateOnce.Do(func() {
        validate = validator.New()

        // Use JSON tag names in error messages
        validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
            name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
            if name == "-" {
                return ""
            }
            return name
        })

        // Register custom validators
        _ = validate.RegisterValidation("uuid", validateUUID)
    })

    return validate
}
```

### Custom Validators

```go
func validateUUID(fl validator.FieldLevel) bool {
    value := fl.Field().String()
    if value == "" {
        return true // Empty is ok, use 'required' tag if needed
    }

    _, err := uuid.Parse(value)
    return err == nil
}
```

### Extracting Field Errors

```go
func ValidationErrors(err error) map[string]string {
    fieldErrors := make(map[string]string)

    var validationErrs validator.ValidationErrors
    if errors.As(err, &validationErrs) {
        for _, fieldErr := range validationErrs {
            fieldName := fieldErr.Field()
            fieldErrors[fieldName] = validationMessage(fieldErr)
        }
    }

    return fieldErrors
}
```

### Combined Validation

Support both struct tags and custom `Validate()` method:

```go
type Validatable interface {
    Validate() error
}

func ValidateAll(v any) error {
    // First validate struct tags
    if err := Validate(v); err != nil {
        return err
    }

    // Then call custom validation if implemented
    if validatable, ok := v.(Validatable); ok {
        if err := validatable.Validate(); err != nil {
            return fmt.Errorf("%w: %w", ErrValidation, err)
        }
    }

    return nil
}
```

---

## Health Checks

Use a registry pattern to aggregate health checks from multiple components.

### HealthChecker Interface

```go
type HealthChecker interface {
    // Name returns a unique identifier for this health check.
    Name() string

    // Check performs the health check and returns an error if unhealthy.
    Check(ctx context.Context) error
}
```

### Health Registry

```go
type HealthRegistry interface {
    Register(checker HealthChecker) error
    CheckAll(ctx context.Context) *HealthResult
}

type DefaultHealthRegistry struct {
    mu       sync.RWMutex
    checkers []HealthChecker
}

func (r *DefaultHealthRegistry) Register(checker HealthChecker) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    // Check for duplicates
    for _, c := range r.checkers {
        if c.Name() == checker.Name() {
            return fmt.Errorf("%w: %s", ErrDuplicateChecker, checker.Name())
        }
    }

    r.checkers = append(r.checkers, checker)
    return nil
}
```

### Implementing a Health Check

Components implement both their main interface and `HealthChecker`:

```go
type QuoteClient struct {
    client *clients.Client
    // ...
}

// Name implements HealthChecker.
func (c *QuoteClient) Name() string {
    return "quote-service"
}

// Check implements HealthChecker.
func (c *QuoteClient) Check(ctx context.Context) error {
    _, err := c.GetRandomQuote(ctx)
    return err
}
```

---

## References

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Blog: Error Handling](https://go.dev/blog/error-handling-and-go)
- [Go Proverbs](https://go-proverbs.github.io/)
