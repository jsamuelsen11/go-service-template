package context

import (
	"context"
	"sync"
)

type ctxKey struct{}

// RequestContext provides request-scoped lazy loading and action collection.
type RequestContext struct {
	ctx       context.Context
	cache     sync.Map   // Thread-safe memoization cache
	actions   []Action   // Staged write actions
	mu        sync.Mutex // Protects actions slice
	committed bool
}

// New creates a new RequestContext wrapping the given context.
func New(ctx context.Context) *RequestContext {
	return &RequestContext{ctx: ctx}
}

// FromContext extracts RequestContext, returns nil if not present.
func FromContext(ctx context.Context) *RequestContext {
	if ctx == nil {
		return nil
	}
	if rc, ok := ctx.Value(ctxKey{}).(*RequestContext); ok {
		return rc
	}
	return nil
}

// WithContext stores RequestContext in the context.
func WithContext(ctx context.Context, rc *RequestContext) context.Context {
	return context.WithValue(ctx, ctxKey{}, rc)
}

// GetOrFetch retrieves cached value or executes fetchFn and caches result.
// Thread-safe via sync.Map.
func (rc *RequestContext) GetOrFetch(key string, fetchFn func(ctx context.Context) (any, error)) (any, error) {
	// Fast path: check cache
	if cached, ok := rc.cache.Load(key); ok {
		return cached, nil
	}

	// Slow path: fetch and cache
	value, err := fetchFn(rc.ctx)
	if err != nil {
		return nil, err
	}

	// Store (LoadOrStore handles race)
	actual, _ := rc.cache.LoadOrStore(key, value)
	return actual, nil
}

// Context returns the underlying context.
func (rc *RequestContext) Context() context.Context {
	return rc.ctx
}
