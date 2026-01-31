# Go Patterns Guide

This document covers idiomatic Go patterns for common scenarios in this service template.

## Table of Contents

- [Concurrency](#concurrency)
- [Service Layer](#service-layer)
- [Error Handling](#error-handling)

---

## Concurrency

Use `golang.org/x/sync/errgroup` directly for concurrent operations.
It's well-documented, idiomatic, and every Go developer knows it.

### Parallel Operations with Cancellation

When any operation fails, cancel all others:

```go
import "golang.org/x/sync/errgroup"

func FetchUserData(ctx context.Context, userID string) (*UserData, error) {
    g, ctx := errgroup.WithContext(ctx)

    var user *User
    var posts []Post
    var settings *Settings

    g.Go(func() error {
        var err error
        user, err = userService.Get(ctx, userID)
        return err
    })

    g.Go(func() error {
        var err error
        posts, err = postService.List(ctx, userID)
        return err
    })

    g.Go(func() error {
        var err error
        settings, err = settingsService.Get(ctx, userID)
        return err
    })

    if err := g.Wait(); err != nil {
        return nil, fmt.Errorf("fetching user data: %w", err)
    }

    return &UserData{User: user, Posts: posts, Settings: settings}, nil
}
```

### Bounded Concurrency

Limit concurrent operations to protect downstream services:

```go
func ProcessItems(ctx context.Context, items []Item) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(10) // Max 10 concurrent operations

    for _, item := range items {
        g.Go(func() error {
            return processItem(ctx, item)
        })
    }

    return g.Wait()
}
```

### Worker Pool (Fan-Out)

Distribute work across a fixed number of workers:

```go
func ProcessWithWorkers(ctx context.Context, items []string) error {
    g, ctx := errgroup.WithContext(ctx)
    itemChan := make(chan string)

    // Start workers
    const numWorkers = 5
    for range numWorkers {
        g.Go(func() error {
            for item := range itemChan {
                if err := process(ctx, item); err != nil {
                    return err
                }
            }
            return nil
        })
    }

    // Feed items to workers
    g.Go(func() error {
        defer close(itemChan)
        for _, item := range items {
            select {
            case itemChan <- item:
            case <-ctx.Done():
                return ctx.Err()
            }
        }
        return nil
    })

    return g.Wait()
}
```

### Partial Success (Collect All Errors)

When you need results from all operations, even if some fail:

```go
import "errors"

func FetchAll(ctx context.Context, ids []string) ([]Result, error) {
    results := make([]Result, len(ids))
    var mu sync.Mutex
    var errs error

    var wg sync.WaitGroup
    for i, id := range ids {
        wg.Add(1)
        go func() {
            defer wg.Done()

            result, err := fetch(ctx, id)

            mu.Lock()
            defer mu.Unlock()

            if err != nil {
                errs = errors.Join(errs, fmt.Errorf("fetch %s: %w", id, err))
            } else {
                results[i] = result
            }
        }()
    }

    wg.Wait()
    return results, errs
}
```

---

## Service Layer

Services are thin orchestrators that coordinate between domain logic and infrastructure.

### Basic Service Structure

```go
type OrderService struct {
    repo     OrderRepository
    payment  PaymentClient
    notify   NotificationService
    logger   *slog.Logger
}

func NewOrderService(
    repo OrderRepository,
    payment PaymentClient,
    notify NotificationService,
    logger *slog.Logger,
) *OrderService {
    return &OrderService{
        repo:    repo,
        payment: payment,
        notify:  notify,
        logger:  logger.With(slog.String("service", "order")),
    }
}
```

### Use Case Method Pattern

Each public method represents a use case:

```go
func (s *OrderService) PlaceOrder(ctx context.Context, req PlaceOrderRequest) (*Order, error) {
    // 1. Validate input
    if err := req.Validate(); err != nil {
        return nil, err
    }

    // 2. Execute business logic
    order := domain.NewOrder(req.CustomerID, req.Items)

    // 3. Coordinate with external services
    if err := s.payment.Charge(ctx, order.Total()); err != nil {
        return nil, fmt.Errorf("charging payment: %w", err)
    }

    // 4. Persist state
    if err := s.repo.Save(ctx, order); err != nil {
        return nil, fmt.Errorf("saving order: %w", err)
    }

    // 5. Side effects (async is often better)
    go s.notify.OrderPlaced(context.Background(), order)

    return order, nil
}
```

### Guidelines

1. **Keep services thin** - Business rules belong in domain, not services
2. **One responsibility** - Each method handles one use case
3. **Explicit dependencies** - Inject via constructor, no globals
4. **Log at boundaries** - Log once when entering/exiting, not in every layer

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

### Use Domain Error Types

Define typed errors for business failures (see `internal/domain/errors.go`):

```go
// In domain/errors.go
var ErrNotFound = errors.New("not found")

type NotFoundError struct {
    Entity string
    ID     string
}

func (e *NotFoundError) Error() string {
    return fmt.Sprintf("%s %q not found", e.Entity, e.ID)
}

func (e *NotFoundError) Unwrap() error {
    return ErrNotFound
}

// Usage
return nil, &domain.NotFoundError{Entity: "user", ID: id}

// Checking
if errors.Is(err, domain.ErrNotFound) {
    // Handle not found
}
```

### Error Handling at Boundaries

Handle and log errors at service boundaries (HTTP handlers, gRPC handlers):

```go
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := h.service.GetUser(r.Context(), chi.URLParam(r, "id"))
    if err != nil {
        switch {
        case errors.Is(err, domain.ErrNotFound):
            http.Error(w, "User not found", http.StatusNotFound)
        case errors.Is(err, domain.ErrValidation):
            http.Error(w, err.Error(), http.StatusBadRequest)
        default:
            h.logger.Error("failed to get user", slog.Any("error", err))
            http.Error(w, "Internal error", http.StatusInternalServerError)
        }
        return
    }

    json.NewEncoder(w).Encode(user)
}
```

### Aggregate Multiple Errors

When processing batches where partial failure is acceptable:

```go
var errs error
for _, item := range items {
    if err := process(item); err != nil {
        errs = errors.Join(errs, err)
    }
}
if errs != nil {
    return fmt.Errorf("processing items: %w", errs)
}
```

---

## References

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Blog: Error Handling](https://go.dev/blog/error-handling-and-go)
- [errgroup documentation](https://pkg.go.dev/golang.org/x/sync/errgroup)
