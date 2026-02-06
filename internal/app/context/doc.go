// Package context provides request-scoped context management for orchestration
// services using a Two-Phase Request Context Pattern.
//
// # Phase 1: Lazy Memoization
//
// Use GetOrFetch to cache expensive operations within a request:
//
//	rc := context.FromContext(ctx)
//	user, err := rc.GetOrFetch("user:123", func(ctx context.Context) (any, error) {
//	    return userClient.GetByID(ctx, "123")
//	})
//
// Subsequent calls with the same key return the cached value without re-fetching.
//
// # Phase 2: Staged Writes
//
// Collect write operations and execute them atomically:
//
//	rc.AddAction(&UpdateUserAction{UserID: "123", Name: "New Name"})
//	rc.AddAction(&SendEmailAction{To: "user@example.com"})
//
//	if err := rc.Commit(ctx); err != nil {
//	    // Actions are rolled back automatically
//	}
//
// # Usage in Application Services
//
//	func (s *Service) ProcessOrder(ctx context.Context, orderID string) error {
//	    rc := context.New(ctx)
//	    ctx = context.WithContext(ctx, rc)
//
//	    // Phase 1: Fetch data (memoized)
//	    order, _ := rc.GetOrFetch("order:"+orderID, s.orderClient.Get)
//	    user, _ := rc.GetOrFetch("user:"+order.UserID, s.userClient.Get)
//
//	    // Phase 2: Stage writes
//	    rc.AddAction(&ChargePaymentAction{...})
//	    rc.AddAction(&CreateShipmentAction{...})
//
//	    return rc.Commit(ctx)
//	}
package context
