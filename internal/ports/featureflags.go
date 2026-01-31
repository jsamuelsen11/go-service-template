package ports

import (
	"context"
)

// FeatureFlags defines the contract for feature flag evaluation.
// This port allows the application to check feature enablement without
// knowing the underlying provider (LaunchDarkly, Unleash, ConfigCat, etc.).
//
// Design principles:
//   - Always provide default values for graceful degradation
//   - Context parameter for user/request targeting
//   - Synchronous evaluation (async flag updates happen in adapter)
//
// Example usage:
//
//	if flags.IsEnabled(ctx, "new-checkout-flow", false) {
//	    return s.newCheckoutFlow(ctx, cart)
//	}
//	return s.legacyCheckoutFlow(ctx, cart)
type FeatureFlags interface {
	// IsEnabled checks if a boolean feature flag is enabled.
	// Returns defaultValue if the flag doesn't exist or evaluation fails.
	// The context may contain user attributes for targeting rules.
	IsEnabled(ctx context.Context, flag string, defaultValue bool) bool

	// GetString retrieves a string feature flag value.
	// Returns defaultValue if the flag doesn't exist or evaluation fails.
	// Useful for A/B testing with string variants.
	GetString(ctx context.Context, flag string, defaultValue string) string

	// GetInt retrieves an integer feature flag value.
	// Returns defaultValue if the flag doesn't exist or evaluation fails.
	// Useful for gradual rollouts with percentage values.
	GetInt(ctx context.Context, flag string, defaultValue int) int

	// GetFloat retrieves a float feature flag value.
	// Returns defaultValue if the flag doesn't exist or evaluation fails.
	// Useful for threshold configurations.
	GetFloat(ctx context.Context, flag string, defaultValue float64) float64

	// GetJSON retrieves a JSON feature flag value and unmarshals into target.
	// Returns error if the flag doesn't exist, evaluation fails, or unmarshal fails.
	// Useful for complex configuration objects.
	GetJSON(ctx context.Context, flag string, target any) error
}

// FeatureFlagUser represents user context for targeted flag evaluation.
// Embed this in context using FeatureFlagUserKey.
type FeatureFlagUser struct {
	// ID is the unique user identifier.
	ID string

	// Anonymous indicates if this is an anonymous/unauthenticated user.
	Anonymous bool

	// Attributes contains custom attributes for targeting rules.
	// Example: {"plan": "enterprise", "region": "us-east-1"}
	Attributes map[string]any
}

// FeatureFlagUserKey is the context key for FeatureFlagUser.
type featureFlagUserKey struct{}

// FeatureFlagUserKey is used to store/retrieve FeatureFlagUser from context.
var FeatureFlagUserKey = featureFlagUserKey{}

// WithFeatureFlagUser adds user context for feature flag evaluation.
func WithFeatureFlagUser(ctx context.Context, user *FeatureFlagUser) context.Context {
	return context.WithValue(ctx, FeatureFlagUserKey, user)
}

// GetFeatureFlagUser retrieves the user from context, or nil if not present.
func GetFeatureFlagUser(ctx context.Context) *FeatureFlagUser {
	if user, ok := ctx.Value(FeatureFlagUserKey).(*FeatureFlagUser); ok {
		return user
	}

	return nil
}
