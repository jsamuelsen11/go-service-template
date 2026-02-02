# Implementing a Feature Flag

This guide explains how to use feature flags for gradual rollouts, A/B testing, and kill switches.

## Overview

The template provides a `FeatureFlags` port interface that abstracts the underlying feature flag
provider (LaunchDarkly, Unleash, ConfigCat, etc.).

**File:** `internal/ports/featureflags.go`

```go
type FeatureFlags interface {
    IsEnabled(ctx context.Context, flag string, defaultValue bool) bool
    GetString(ctx context.Context, flag string, defaultValue string) string
    GetInt(ctx context.Context, flag string, defaultValue int) int
    GetFloat(ctx context.Context, flag string, defaultValue float64) float64
    GetJSON(ctx context.Context, flag string, target any) error
}
```

---

## Step 1: Inject FeatureFlags into Your Service

Add the `FeatureFlags` port to your service configuration.

**File:** `internal/app/checkout_service.go`

```go
package app

import (
    "context"
    "log/slog"

    "github.com/jsamuelsen/go-service-template/internal/domain"
    "github.com/jsamuelsen/go-service-template/internal/ports"
)

// CheckoutServiceConfig holds dependencies for CheckoutService.
type CheckoutServiceConfig struct {
    OrderRepo    ports.OrderRepository
    PaymentClient ports.PaymentClient
    Flags        ports.FeatureFlags  // Add feature flags
    Logger       *slog.Logger
}

// CheckoutService handles checkout use cases.
type CheckoutService struct {
    orderRepo    ports.OrderRepository
    paymentClient ports.PaymentClient
    flags        ports.FeatureFlags
    logger       *slog.Logger
}

// NewCheckoutService creates a new checkout service.
func NewCheckoutService(cfg CheckoutServiceConfig) *CheckoutService {
    return &CheckoutService{
        orderRepo:    cfg.OrderRepo,
        paymentClient: cfg.PaymentClient,
        flags:        cfg.Flags,
        logger:       cfg.Logger,
    }
}
```

---

## Step 2: Use Boolean Flags

Use `IsEnabled()` for on/off feature toggles.

```go
func (s *CheckoutService) Checkout(ctx context.Context, cart *domain.Cart) (*domain.Order, error) {
    // Feature flag: Use new checkout flow
    if s.flags.IsEnabled(ctx, "new-checkout-flow", false) {
        return s.newCheckoutFlow(ctx, cart)
    }

    return s.legacyCheckoutFlow(ctx, cart)
}
```

**Key points:**

- Always provide a `defaultValue` for graceful degradation
- Default `false` means the feature is off if the flag service is unavailable

---

## Step 3: Use String Flags for A/B Testing

Use `GetString()` for multi-variant experiments.

```go
func (s *CheckoutService) GetPaymentOptions(ctx context.Context) []string {
    // A/B test: Payment button layout
    variant := s.flags.GetString(ctx, "payment-button-layout", "horizontal")

    switch variant {
    case "vertical":
        return []string{"credit_card", "paypal", "apple_pay", "google_pay"}
    case "two-column":
        return []string{"credit_card", "paypal"}
    default: // "horizontal"
        return []string{"credit_card", "paypal", "apple_pay"}
    }
}
```

---

## Step 4: Use Numeric Flags for Rollouts

Use `GetInt()` or `GetFloat()` for percentage rollouts or thresholds.

```go
func (s *CheckoutService) ApplyDiscount(ctx context.Context, order *domain.Order) {
    // Gradual rollout: Discount percentage
    discountPercent := s.flags.GetInt(ctx, "checkout-discount-percent", 0)

    if discountPercent > 0 {
        order.ApplyDiscount(discountPercent)
    }
}

func (s *CheckoutService) CheckFraudThreshold(ctx context.Context, amount int64) bool {
    // Configurable threshold
    threshold := s.flags.GetFloat(ctx, "fraud-threshold", 1000.00)

    return float64(amount) > threshold
}
```

---

## Step 5: Use JSON Flags for Complex Config

Use `GetJSON()` for structured configuration.

```go
type ShippingConfig struct {
    FreeShippingThreshold int      `json:"free_shipping_threshold"`
    EnabledCarriers       []string `json:"enabled_carriers"`
    ExpressDeliveryFee    float64  `json:"express_delivery_fee"`
}

func (s *CheckoutService) GetShippingOptions(ctx context.Context, order *domain.Order) (*ShippingConfig, error) {
    var config ShippingConfig

    // Load complex configuration from feature flag
    if err := s.flags.GetJSON(ctx, "shipping-config", &config); err != nil {
        // Use defaults on error
        return &ShippingConfig{
            FreeShippingThreshold: 50,
            EnabledCarriers:       []string{"usps", "ups"},
            ExpressDeliveryFee:    9.99,
        }, nil
    }

    return &config, nil
}
```

---

## User-Targeted Flags

For personalized flags (e.g., beta users, specific regions), add user context.

### Set User Context in Middleware

**File:** `internal/adapters/http/middleware/featureflags.go`

```go
package middleware

import (
    "github.com/gin-gonic/gin"

    "github.com/jsamuelsen/go-service-template/internal/ports"
)

// FeatureFlagUser injects user context for targeted feature flags.
func FeatureFlagUser() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract user from auth claims or headers
        claims := ExtractClaims(c, nil)

        if claims != nil {
            user := &ports.FeatureFlagUser{
                ID:        claims.Subject,
                Anonymous: false,
                Attributes: map[string]any{
                    "plan":   claims.Plan,
                    "region": c.GetHeader("X-User-Region"),
                },
            }

            // Add to context
            ctx := ports.WithFeatureFlagUser(c.Request.Context(), user)
            c.Request = c.Request.WithContext(ctx)
        }

        c.Next()
    }
}
```

### Use in Service

```go
func (s *CheckoutService) IsEligibleForBeta(ctx context.Context) bool {
    // The flag provider can use user attributes for targeting
    // e.g., only enable for users with plan="enterprise"
    return s.flags.IsEnabled(ctx, "beta-features", false)
}
```

---

## Wiring in main.go

```go
func run() error {
    // ... initialization ...

    // Create feature flag client (implementation depends on provider)
    flagClient := launchdarkly.NewClient(cfg.LaunchDarkly.SDKKey)
    // or: flagClient := unleash.NewClient(cfg.Unleash.URL)
    // or: flagClient := configcat.NewClient(cfg.ConfigCat.SDKKey)

    // Inject into services
    checkoutService := app.NewCheckoutService(app.CheckoutServiceConfig{
        // ... other deps ...
        Flags:  flagClient,
        Logger: logger,
    })

    // ... rest of startup ...
}
```

---

## Testing with Feature Flags

Use mock feature flags in tests:

```go
func TestCheckoutService_NewFlow(t *testing.T) {
    // Create mock feature flags
    mockFlags := mocks.NewMockFeatureFlags(t)

    // Configure mock to return true for new-checkout-flow
    mockFlags.EXPECT().
        IsEnabled(mock.Anything, "new-checkout-flow", false).
        Return(true)

    // Create service with mock
    service := app.NewCheckoutService(app.CheckoutServiceConfig{
        Flags: mockFlags,
        // ... other mocks ...
    })

    // Test new checkout flow
    order, err := service.Checkout(ctx, cart)
    assert.NoError(t, err)
    // ... assertions ...
}
```

---

## Best Practices

### 1. Always Provide Defaults

```go
// Good - safe default
s.flags.IsEnabled(ctx, "risky-feature", false)

// Good - safe default for config
s.flags.GetInt(ctx, "max-retries", 3)
```

### 2. Use Descriptive Flag Names

```go
// Good
"enable-new-checkout-flow"
"checkout-discount-percent"
"payment-button-layout-experiment"

// Bad
"flag1"
"test"
"feature_x"
```

### 3. Document Flag Behavior

```go
// ProcessPayment handles payment with optional new processor.
// Flag: "use-new-payment-processor" (default: false)
// - true: Use Stripe v2 API
// - false: Use legacy payment processor
func (s *Service) ProcessPayment(ctx context.Context, ...) error {
```

### 4. Clean Up Old Flags

Remove flag checks after full rollout:

```go
// Before: Feature flag (can be removed after 100% rollout)
if s.flags.IsEnabled(ctx, "new-checkout-flow", false) {
    return s.newCheckoutFlow(ctx, cart)
}
return s.legacyCheckoutFlow(ctx, cart)

// After: Flag removed, old code deleted
return s.newCheckoutFlow(ctx, cart)
```

---

## Checklist

- [ ] Add `FeatureFlags` to service config
- [ ] Inject via constructor
- [ ] Use appropriate method (`IsEnabled`, `GetString`, etc.)
- [ ] Provide safe default values
- [ ] Add user context for targeted flags (if needed)
- [ ] Write tests with mock flags
- [ ] Document flag behavior
- [ ] Plan for flag cleanup after rollout

---

## Related Documentation

- [Writing Unit Tests](./writing-unit-tests.md) - Mocking feature flags
