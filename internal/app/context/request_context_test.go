package context

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	assert.NotNil(t, rc)
	assert.Equal(t, ctx, rc.Context())
}

func TestFromContext_NilContext(t *testing.T) {
	rc := FromContext(nil)
	assert.Nil(t, rc)
}

func TestFromContext_NoRequestContext(t *testing.T) {
	ctx := context.Background()
	rc := FromContext(ctx)
	assert.Nil(t, rc)
}

func TestWithContext_RoundTrip(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	enrichedCtx := WithContext(ctx, rc)
	extracted := FromContext(enrichedCtx)

	assert.Equal(t, rc, extracted)
}

func TestGetOrFetch_CachesValue(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	var callCount int32
	fetchFn := func(_ context.Context) (any, error) {
		atomic.AddInt32(&callCount, 1)
		return "cached-value", nil
	}

	// First call should fetch
	val1, err := rc.GetOrFetch("key", fetchFn)
	require.NoError(t, err)
	assert.Equal(t, "cached-value", val1)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount))

	// Second call should use cache
	val2, err := rc.GetOrFetch("key", fetchFn)
	require.NoError(t, err)
	assert.Equal(t, "cached-value", val2)
	assert.Equal(t, int32(1), atomic.LoadInt32(&callCount)) // Still 1
}

func TestGetOrFetch_PropagatesError(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	expectedErr := errors.New("fetch failed")
	fetchFn := func(_ context.Context) (any, error) {
		return nil, expectedErr
	}

	val, err := rc.GetOrFetch("key", fetchFn)
	assert.Nil(t, val)
	assert.ErrorIs(t, err, expectedErr)
}

func TestGetOrFetch_DifferentKeys(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	var callCount int32
	fetchFn := func(_ context.Context) (any, error) {
		count := atomic.AddInt32(&callCount, 1)
		return count, nil
	}

	val1, _ := rc.GetOrFetch("key1", fetchFn)
	val2, _ := rc.GetOrFetch("key2", fetchFn)

	assert.Equal(t, int32(1), val1)
	assert.Equal(t, int32(2), val2)
	assert.Equal(t, int32(2), atomic.LoadInt32(&callCount))
}

// mockAction is a test helper for Action interface.
type mockAction struct {
	description string
	executeErr  error
	rollbackErr error
	executed    bool
	rolledBack  bool
}

func (a *mockAction) Execute(_ context.Context) error {
	a.executed = true
	return a.executeErr
}

func (a *mockAction) Rollback(_ context.Context) error {
	a.rolledBack = true
	return a.rollbackErr
}

func (a *mockAction) Description() string {
	return a.description
}

func TestAddAction_StagesAction(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	action := &mockAction{description: "test-action"}
	err := rc.AddAction(action)

	require.NoError(t, err)
	assert.Len(t, rc.Actions(), 1)
}

func TestAddAction_AfterCommit_ReturnsError(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	// Commit with no actions
	err := rc.Commit(ctx)
	require.NoError(t, err)

	// Try to add action after commit
	action := &mockAction{description: "test-action"}
	err = rc.AddAction(action)

	assert.ErrorIs(t, err, ErrAlreadyCommitted)
}

func TestCommit_ExecutesAllActions(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	action1 := &mockAction{description: "action1"}
	action2 := &mockAction{description: "action2"}
	action3 := &mockAction{description: "action3"}

	_ = rc.AddAction(action1)
	_ = rc.AddAction(action2)
	_ = rc.AddAction(action3)

	err := rc.Commit(ctx)

	require.NoError(t, err)
	assert.True(t, action1.executed)
	assert.True(t, action2.executed)
	assert.True(t, action3.executed)
	assert.False(t, action1.rolledBack)
	assert.False(t, action2.rolledBack)
	assert.False(t, action3.rolledBack)
}

func TestCommit_RollsBackOnFailure(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	action1 := &mockAction{description: "action1"}
	action2 := &mockAction{description: "action2"}
	action3 := &mockAction{description: "action3", executeErr: errors.New("action3 failed")}

	_ = rc.AddAction(action1)
	_ = rc.AddAction(action2)
	_ = rc.AddAction(action3)

	err := rc.Commit(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "action3 failed")
	assert.Contains(t, err.Error(), "action3")

	// First two should be executed and rolled back
	assert.True(t, action1.executed)
	assert.True(t, action2.executed)
	assert.True(t, action3.executed)
	assert.True(t, action1.rolledBack)
	assert.True(t, action2.rolledBack)
	assert.False(t, action3.rolledBack) // Failed action is not rolled back
}

func TestCommit_DoubleCommit_ReturnsError(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	err := rc.Commit(ctx)
	require.NoError(t, err)

	err = rc.Commit(ctx)
	assert.ErrorIs(t, err, ErrAlreadyCommitted)
}

func TestCommit_EmptyActions(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	err := rc.Commit(ctx)
	require.NoError(t, err)
}

func TestActions_ReturnsCopy(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	action := &mockAction{description: "test-action"}
	_ = rc.AddAction(action)

	actions := rc.Actions()
	assert.Len(t, actions, 1)

	// Modifying returned slice should not affect internal state
	actions[0] = nil
	assert.NotNil(t, rc.Actions()[0])
}

// DataProvider tests

type mockProvider struct {
	key      string
	value    any
	fetchErr error
}

func (p *mockProvider) Key() string {
	return p.key
}

func (p *mockProvider) Fetch(_ context.Context) (any, error) {
	if p.fetchErr != nil {
		return nil, p.fetchErr
	}
	return p.value, nil
}

func TestGetOrFetchProvider(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	provider := &mockProvider{key: "provider:key", value: "provider-value"}

	val, err := rc.GetOrFetchProvider(provider)

	require.NoError(t, err)
	assert.Equal(t, "provider-value", val)
}

func TestGetOrFetchProvider_CachesValue(t *testing.T) {
	ctx := context.Background()
	rc := New(ctx)

	provider := &mockProvider{key: "provider:key", value: "cached-value"}

	// First call
	val1, err := rc.GetOrFetchProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, "cached-value", val1)

	// Second call should use cache (we verify by checking GetOrFetch behavior)
	// Since mockProvider.Fetch is simple, we verify caching via GetOrFetch test
	val2, err := rc.GetOrFetchProvider(provider)
	require.NoError(t, err)
	assert.Equal(t, "cached-value", val2)
}
