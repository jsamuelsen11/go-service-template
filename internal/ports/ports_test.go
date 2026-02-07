package ports

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockChecker implements HealthChecker for testing.
type mockChecker struct {
	name string
	err  error
}

func (m *mockChecker) Name() string {
	return m.name
}

func (m *mockChecker) Check(ctx context.Context) error {
	return m.err
}

// TestNewHealthRegistry verifies that a new registry is created with empty checkers.
func TestNewHealthRegistry(t *testing.T) {
	registry := NewHealthRegistry()

	require.NotNil(t, registry)
	assert.NotNil(t, registry.checkers)
	assert.Empty(t, registry.checkers)
}

// TestRegister_Success verifies that a checker can be registered successfully.
func TestRegister_Success(t *testing.T) {
	registry := NewHealthRegistry()
	checker := &mockChecker{name: "database"}

	err := registry.Register(checker)

	require.NoError(t, err)
	assert.Len(t, registry.checkers, 1)
	assert.Equal(t, "database", registry.checkers[0].Name())
}

// TestRegister_DuplicateName verifies that registering duplicate checker names returns an error.
func TestRegister_DuplicateName(t *testing.T) {
	registry := NewHealthRegistry()
	checker1 := &mockChecker{name: "database"}
	checker2 := &mockChecker{name: "database"}

	err := registry.Register(checker1)
	require.NoError(t, err)

	err = registry.Register(checker2)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrDuplicateChecker)
	assert.Contains(t, err.Error(), "database")
	assert.Len(t, registry.checkers, 1)
}

// TestCheckAll_NoCheckers verifies that an empty registry returns healthy status.
func TestCheckAll_NoCheckers(t *testing.T) {
	registry := NewHealthRegistry()
	ctx := context.Background()

	result := registry.CheckAll(ctx)

	require.NotNil(t, result)
	assert.Equal(t, HealthStatusHealthy, result.Status)
	assert.NotNil(t, result.Checks)
	assert.Empty(t, result.Checks)
	assert.False(t, result.Timestamp.IsZero())
}

// TestCheckAll_AllHealthy verifies that multiple healthy checkers result in healthy status.
func TestCheckAll_AllHealthy(t *testing.T) {
	registry := NewHealthRegistry()
	checker1 := &mockChecker{name: "database", err: nil}
	checker2 := &mockChecker{name: "cache", err: nil}
	checker3 := &mockChecker{name: "queue", err: nil}

	require.NoError(t, registry.Register(checker1))
	require.NoError(t, registry.Register(checker2))
	require.NoError(t, registry.Register(checker3))

	ctx := context.Background()
	result := registry.CheckAll(ctx)

	require.NotNil(t, result)
	assert.Equal(t, HealthStatusHealthy, result.Status)
	assert.Len(t, result.Checks, 3)

	// Verify all checks are healthy
	assert.Equal(t, HealthStatusHealthy, result.Checks["database"].Status)
	assert.Equal(t, HealthStatusHealthy, result.Checks["cache"].Status)
	assert.Equal(t, HealthStatusHealthy, result.Checks["queue"].Status)

	// Verify no error messages
	assert.Empty(t, result.Checks["database"].Message)
	assert.Empty(t, result.Checks["cache"].Message)
	assert.Empty(t, result.Checks["queue"].Message)
}

// TestCheckAll_OneUnhealthy verifies that one failing checker makes the overall result unhealthy.
func TestCheckAll_OneUnhealthy(t *testing.T) {
	registry := NewHealthRegistry()
	checker1 := &mockChecker{name: "database", err: nil}
	checker2 := &mockChecker{name: "cache", err: errors.New("connection timeout")}
	checker3 := &mockChecker{name: "queue", err: nil}

	require.NoError(t, registry.Register(checker1))
	require.NoError(t, registry.Register(checker2))
	require.NoError(t, registry.Register(checker3))

	ctx := context.Background()
	result := registry.CheckAll(ctx)

	require.NotNil(t, result)
	assert.Equal(t, HealthStatusUnhealthy, result.Status)
	assert.Len(t, result.Checks, 3)

	// Verify individual statuses
	assert.Equal(t, HealthStatusHealthy, result.Checks["database"].Status)
	assert.Equal(t, HealthStatusUnhealthy, result.Checks["cache"].Status)
	assert.Equal(t, HealthStatusHealthy, result.Checks["queue"].Status)

	// Verify error message is captured
	assert.Empty(t, result.Checks["database"].Message)
	assert.Equal(t, "connection timeout", result.Checks["cache"].Message)
	assert.Empty(t, result.Checks["queue"].Message)
}

// contextAwareChecker implements HealthChecker that respects context cancellation.
type contextAwareChecker struct {
	name string
}

func (c *contextAwareChecker) Name() string {
	return c.name
}

func (c *contextAwareChecker) Check(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return nil
	}
}

// TestCheckAll_ContextCancelled verifies that the health check respects context cancellation.
func TestCheckAll_ContextCancelled(t *testing.T) {
	registry := NewHealthRegistry()
	checker := &contextAwareChecker{name: "slow-service"}

	require.NoError(t, registry.Register(checker))

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := registry.CheckAll(ctx)

	require.NotNil(t, result)
	assert.Equal(t, HealthStatusUnhealthy, result.Status)
	assert.Len(t, result.Checks, 1)
	assert.Equal(t, HealthStatusUnhealthy, result.Checks["slow-service"].Status)
	assert.Contains(t, result.Checks["slow-service"].Message, "context canceled")
}

// TestWithFeatureFlagUser verifies that a user can be stored in context.
func TestWithFeatureFlagUser(t *testing.T) {
	user := &FeatureFlagUser{
		ID:        "user-123",
		Anonymous: false,
	}

	ctx := WithFeatureFlagUser(context.Background(), user)

	require.NotNil(t, ctx)

	// Verify the user can be retrieved
	retrievedUser := GetFeatureFlagUser(ctx)
	require.NotNil(t, retrievedUser)
	assert.Equal(t, "user-123", retrievedUser.ID)
	assert.False(t, retrievedUser.Anonymous)
}

// TestGetFeatureFlagUser_Present verifies that a stored user can be retrieved.
func TestGetFeatureFlagUser_Present(t *testing.T) {
	user := &FeatureFlagUser{
		ID:        "user-456",
		Anonymous: true,
	}

	ctx := WithFeatureFlagUser(context.Background(), user)
	retrievedUser := GetFeatureFlagUser(ctx)

	require.NotNil(t, retrievedUser)
	assert.Equal(t, "user-456", retrievedUser.ID)
	assert.True(t, retrievedUser.Anonymous)
}

// TestGetFeatureFlagUser_NotPresent verifies that nil is returned when no user is in context.
func TestGetFeatureFlagUser_NotPresent(t *testing.T) {
	ctx := context.Background()

	user := GetFeatureFlagUser(ctx)

	assert.Nil(t, user)
}

// TestGetFeatureFlagUser_WithAttributes verifies that user attributes are preserved.
func TestGetFeatureFlagUser_WithAttributes(t *testing.T) {
	user := &FeatureFlagUser{
		ID:        "user-789",
		Anonymous: false,
		Attributes: map[string]any{
			"plan":   "enterprise",
			"region": "us-east-1",
			"beta":   true,
		},
	}

	ctx := WithFeatureFlagUser(context.Background(), user)
	retrievedUser := GetFeatureFlagUser(ctx)

	require.NotNil(t, retrievedUser)
	assert.Equal(t, "user-789", retrievedUser.ID)
	assert.False(t, retrievedUser.Anonymous)
	require.NotNil(t, retrievedUser.Attributes)
	assert.Len(t, retrievedUser.Attributes, 3)
	assert.Equal(t, "enterprise", retrievedUser.Attributes["plan"])
	assert.Equal(t, "us-east-1", retrievedUser.Attributes["region"])
	assert.Equal(t, true, retrievedUser.Attributes["beta"])
}
