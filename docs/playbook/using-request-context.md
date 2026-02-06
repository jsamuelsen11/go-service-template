# Using Request Context

This guide shows how to use the Two-Phase Request Context Pattern for orchestration services.

## Overview

The Request Context pattern provides:

- **Phase 1:** Lazy memoization using an in-memory cache to avoid duplicate API calls within a request
- **Phase 2:** Staged writes with automatic rollback on failure

**When to use:**

- Orchestrating multiple downstream service calls
- Sharing fetched data across multiple operations in a request
- Coordinating writes that should succeed or fail together

---

## Phase 1: Lazy Memoization

### Basic Usage

```go
import appctx "github.com/jsamuelsen/go-service-template/internal/app/context"

func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
    // Create request context
    rc := appctx.New(ctx)

    // Fetch order (will call API)
    orderVal, err := rc.GetOrFetch("order:"+orderID, func(ctx context.Context) (any, error) {
        return s.orderClient.GetByID(ctx, orderID)
    })
    if err != nil {
        return err
    }
    order := orderVal.(*domain.Order)

    // Fetch user (will call API)
    userVal, err := rc.GetOrFetch("user:"+order.UserID, func(ctx context.Context) (any, error) {
        return s.userClient.GetByID(ctx, order.UserID)
    })
    if err != nil {
        return err
    }
    user := userVal.(*domain.User)

    // Later: fetch order again (uses in-memory cache - no API call)
    orderVal, _ = rc.GetOrFetch("order:"+orderID, nil)

    return nil
}
```

### Using DataProvider for Type Safety

```go
type OrderProvider struct {
    client  ports.OrderClient
    orderID string
}

func (p *OrderProvider) Key() string {
    return "order:" + p.orderID
}

func (p *OrderProvider) Fetch(ctx context.Context) (any, error) {
    return p.client.GetByID(ctx, p.orderID)
}

// Usage
provider := &OrderProvider{client: s.orderClient, orderID: orderID}
orderVal, err := rc.GetOrFetchProvider(provider)
```

---

## Phase 2: Staged Writes

### Implementing an Action

```go
type ChargePaymentAction struct {
    paymentClient ports.PaymentClient
    userID        string
    amount        int64
    chargeID      string // Set after Execute for rollback
}

func (a *ChargePaymentAction) Execute(ctx context.Context) error {
    charge, err := a.paymentClient.Charge(ctx, a.userID, a.amount)
    if err != nil {
        return err
    }
    a.chargeID = charge.ID
    return nil
}

func (a *ChargePaymentAction) Rollback(ctx context.Context) error {
    if a.chargeID == "" {
        return nil // Nothing to rollback
    }
    return a.paymentClient.Refund(ctx, a.chargeID)
}

func (a *ChargePaymentAction) Description() string {
    return fmt.Sprintf("charge payment: user=%s amount=%d", a.userID, a.amount)
}
```

### Staging and Committing Actions

```go
func (s *OrderService) ProcessOrder(ctx context.Context, orderID string) error {
    rc := appctx.New(ctx)

    // Phase 1: Fetch data (in-memory cache)
    order, _ := rc.GetOrFetch("order:"+orderID, s.fetchOrder)
    user, _ := rc.GetOrFetch("user:"+order.UserID, s.fetchUser)

    // Phase 2: Stage writes
    if err := rc.AddAction(&UpdateInventoryAction{Items: order.Items}); err != nil {
        return err
    }
    if err := rc.AddAction(&ChargePaymentAction{
        paymentClient: s.paymentClient,
        userID:        user.ID,
        amount:        order.Total,
    }); err != nil {
        return err
    }
    if err := rc.AddAction(&SendConfirmationAction{Email: user.Email}); err != nil {
        return err
    }

    // Execute all actions - rollback on failure
    return rc.Commit(ctx)
}
```

---

## Checklist

- [ ] Import `appctx "github.com/jsamuelsen/go-service-template/internal/app/context"`
- [ ] Create `RequestContext` with `appctx.New(ctx)`
- [ ] Use consistent cache keys (e.g., `"entity:id"` format)
- [ ] Implement `Action` interface for write operations
- [ ] Implement `Rollback()` for actions that can be undone
- [ ] Call `Commit()` at the end of the orchestration
- [ ] Handle errors from `Commit()` appropriately

---

## Related

- [Architecture](../ARCHITECTURE.md#request-context-pattern) - Pattern overview
- [Patterns](../PATTERNS.md#two-phase-request-context-pattern) - Detailed pattern documentation
- [ADR-0001](../adr/0001-hexagonal-architecture.md#request-context-pattern-for-orchestration) - Architectural decision
