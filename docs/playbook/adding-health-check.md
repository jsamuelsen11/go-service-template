# Adding a Health Check

This guide walks through adding a health check for a dependency (database, external service, etc.).

## Overview

Health checks allow Kubernetes to determine if the service is ready to accept traffic.
The readiness endpoint (`/-/ready`) runs all registered health checks concurrently.

**Components:**

- `HealthChecker` - Interface for individual health checks
- `HealthRegistry` - Aggregates and runs all checks

---

## The HealthChecker Interface

```go
// From internal/ports/health.go
type HealthChecker interface {
    // Name returns a unique identifier for this health check.
    Name() string

    // Check performs the health check and returns an error if unhealthy.
    Check(ctx context.Context) error
}
```

**Requirements:**

- `Name()` must be unique across all registered checkers
- `Check()` must respect context cancellation/deadlines
- Return `nil` if healthy, non-nil error if unhealthy

---

## Example: Database Health Check

**File:** `internal/adapters/postgres/health.go`

```go
package postgres

import (
    "context"
    "database/sql"
)

// DatabaseHealthCheck implements ports.HealthChecker for a SQL database.
type DatabaseHealthCheck struct {
    db *sql.DB
}

// NewDatabaseHealthCheck creates a new database health checker.
func NewDatabaseHealthCheck(db *sql.DB) *DatabaseHealthCheck {
    return &DatabaseHealthCheck{db: db}
}

// Name returns the health check name.
func (h *DatabaseHealthCheck) Name() string {
    return "postgres"
}

// Check verifies database connectivity.
func (h *DatabaseHealthCheck) Check(ctx context.Context) error {
    return h.db.PingContext(ctx)
}
```

---

## Example: HTTP Service Health Check

For external HTTP services, implement the check on the ACL client:

**File:** `internal/adapters/clients/acl/payment_client.go`

```go
// Name returns the health check name.
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

---

## Registering Health Checks

Register health checks during startup in `main.go`:

**File:** `cmd/service/main.go`

```go
func run() error {
    // ... initialization ...

    // Create health registry
    healthRegistry := ports.NewHealthRegistry()

    // Create database connection
    db, err := sql.Open("postgres", cfg.Database.DSN)
    if err != nil {
        return fmt.Errorf("connecting to database: %w", err)
    }

    // Register database health check
    dbCheck := postgres.NewDatabaseHealthCheck(db)
    if err := healthRegistry.Register(dbCheck); err != nil {
        return fmt.Errorf("registering database health check: %w", err)
    }

    // Create Redis client
    redisClient := redis.NewClient(&redis.Options{
        Addr: cfg.Redis.Addr,
    })

    // Register Redis health check
    redisCheck := redis.NewRedisHealthCheck(redisClient)
    if err := healthRegistry.Register(redisCheck); err != nil {
        return fmt.Errorf("registering redis health check: %w", err)
    }

    // Create external client (already implements HealthChecker)
    paymentClient := acl.NewPaymentClient(...)

    // Register payment client health check
    if err := healthRegistry.Register(paymentClient); err != nil {
        return fmt.Errorf("registering payment health check: %w", err)
    }

    // Create health handler
    healthHandler := handlers.NewHealthHandler(healthRegistry, buildInfo)

    // ... rest of startup ...
}
```

---

## Health Check Response

The readiness endpoint returns JSON with all check results:

```json
{
  "status": "healthy",
  "checks": {
    "postgres": {
      "status": "healthy",
      "duration": "1.234ms"
    },
    "redis": {
      "status": "healthy",
      "duration": "0.567ms"
    },
    "payment-service": {
      "status": "unhealthy",
      "message": "connection refused",
      "duration": "5.001s"
    }
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

If any check fails, the overall status is `unhealthy` and HTTP status is 503.

---

## Best Practices

### 1. Use Lightweight Checks

Health checks run frequently. Keep them fast:

```go
// Good: Simple ping
func (h *DBCheck) Check(ctx context.Context) error {
    return h.db.PingContext(ctx)
}

// Bad: Heavy query
func (h *DBCheck) Check(ctx context.Context) error {
    return h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM large_table").Scan(&count)
}
```

### 2. Respect Context Timeout

Always use context-aware methods:

```go
// Good
func (h *Check) Check(ctx context.Context) error {
    return h.client.PingContext(ctx)
}

// Bad - ignores timeout
func (h *Check) Check(ctx context.Context) error {
    return h.client.Ping()
}
```

### 3. Return Meaningful Errors

Include diagnostic information in errors:

```go
func (h *Check) Check(ctx context.Context) error {
    err := h.db.PingContext(ctx)
    if err != nil {
        return fmt.Errorf("database ping failed: %w", err)
    }
    return nil
}
```

### 4. Use Unique Names

Each health check must have a unique name:

```go
// Good - descriptive names
return "postgres-primary"
return "payment-service"
return "inventory-api"

// Bad - generic names
return "database"
return "service"
return "api"
```

---

## Checklist

- [ ] Implement `HealthChecker` interface
- [ ] Return unique name from `Name()`
- [ ] Respect context in `Check()`
- [ ] Register with `healthRegistry.Register()`
- [ ] Check appears in `/-/ready` response
- [ ] Test unhealthy path

---

## Related Documentation

- [Architecture: Health Endpoints](../ARCHITECTURE.md)
- [Adding a Downstream Client](./adding-downstream-client.md) - Clients auto-implement HealthChecker
