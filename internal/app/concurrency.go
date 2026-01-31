package app

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

// Parallel executes multiple functions concurrently and returns on first error.
// All goroutines are canceled when any function returns an error.
//
// Example:
//
//	user, posts, err := Parallel3(ctx,
//	    func(ctx context.Context) (*User, error) { return userSvc.Get(ctx, userID) },
//	    func(ctx context.Context) ([]Post, error) { return postSvc.List(ctx, userID) },
//	)
func Parallel[T any](ctx context.Context, fns ...func(context.Context) (T, error)) ([]T, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make([]T, len(fns))

	for i, fn := range fns {
		g.Go(func() error {
			result, err := fn(ctx)
			if err != nil {
				return err
			}

			results[i] = result

			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return nil, fmt.Errorf("parallel execution failed: %w", err)
	}

	return results, nil
}

// Parallel2 executes two functions concurrently and returns both results or first error.
func Parallel2[T1, T2 any](
	ctx context.Context,
	fn1 func(context.Context) (T1, error),
	fn2 func(context.Context) (T2, error),
) (result1 T1, result2 T2, err error) {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var fnErr error

		result1, fnErr = fn1(ctx)

		return fnErr
	})

	g.Go(func() error {
		var fnErr error

		result2, fnErr = fn2(ctx)

		return fnErr
	})

	err = g.Wait()
	if err != nil {
		var (
			zero1 T1
			zero2 T2
		)

		return zero1, zero2, fmt.Errorf("parallel execution failed: %w", err)
	}

	return result1, result2, nil
}

// Parallel3 executes three functions concurrently and returns all results or first error.
func Parallel3[T1, T2, T3 any](
	ctx context.Context,
	fn1 func(context.Context) (T1, error),
	fn2 func(context.Context) (T2, error),
	fn3 func(context.Context) (T3, error),
) (result1 T1, result2 T2, result3 T3, err error) {
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		var fnErr error

		result1, fnErr = fn1(ctx)

		return fnErr
	})

	g.Go(func() error {
		var fnErr error

		result2, fnErr = fn2(ctx)

		return fnErr
	})

	g.Go(func() error {
		var fnErr error

		result3, fnErr = fn3(ctx)

		return fnErr
	})

	err = g.Wait()
	if err != nil {
		var (
			zero1 T1
			zero2 T2
			zero3 T3
		)

		return zero1, zero2, zero3, fmt.Errorf("parallel execution failed: %w", err)
	}

	return result1, result2, result3, nil
}

// ParallelLimit executes functions with bounded concurrency.
// At most 'limit' goroutines run simultaneously.
//
// Example:
//
//	results, err := ParallelLimit(ctx, 5, fetchFuncs...)
func ParallelLimit[T any](
	ctx context.Context,
	limit int,
	fns ...func(context.Context) (T, error),
) ([]T, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	results := make([]T, len(fns))

	for i, fn := range fns {
		g.Go(func() error {
			result, err := fn(ctx)
			if err != nil {
				return err
			}

			results[i] = result

			return nil
		})
	}

	err := g.Wait()
	if err != nil {
		return nil, fmt.Errorf("parallel execution failed: %w", err)
	}

	return results, nil
}

// PartialResult holds a result or an error for partial success patterns.
type PartialResult[T any] struct {
	Value T
	Err   error
}

// ParallelPartial executes functions and collects all results, even on partial failure.
// Unlike Parallel, this does not cancel on first error.
//
// Example:
//
//	results := ParallelPartial(ctx, fetchFuncs...)
//	var successful []Data
//	var failed []error
//	for _, r := range results {
//	    if r.Err != nil {
//	        failed = append(failed, r.Err)
//	    } else {
//	        successful = append(successful, r.Value)
//	    }
//	}
func ParallelPartial[T any](
	ctx context.Context,
	fns ...func(context.Context) (T, error),
) []PartialResult[T] {
	results := make([]PartialResult[T], len(fns))

	var wg sync.WaitGroup

	for i, fn := range fns {
		wg.Go(func() {
			value, err := fn(ctx)
			results[i] = PartialResult[T]{Value: value, Err: err}
		})
	}

	wg.Wait()

	return results
}

// ParallelPartialLimit executes functions with bounded concurrency, collecting all results.
func ParallelPartialLimit[T any](
	ctx context.Context,
	limit int,
	fns ...func(context.Context) (T, error),
) []PartialResult[T] {
	results := make([]PartialResult[T], len(fns))
	sem := make(chan struct{}, limit)

	var wg sync.WaitGroup

	for i, fn := range fns {
		wg.Go(func() {
			sem <- struct{}{}

			defer func() { <-sem }()

			value, err := fn(ctx)
			results[i] = PartialResult[T]{Value: value, Err: err}
		})
	}

	wg.Wait()

	return results
}

// FanOut distributes work items across a fixed number of workers.
// Each worker processes items sequentially, but workers run in parallel.
//
// Example:
//
//	err := FanOut(ctx, 3, userIDs, func(ctx context.Context, userID string) error {
//	    return sendEmail(ctx, userID)
//	})
func FanOut[T any](ctx context.Context, workers int, items []T, fn func(context.Context, T) error) error {
	g, ctx := errgroup.WithContext(ctx)
	itemChan := make(chan T)

	// Start workers.
	for range workers {
		g.Go(func() error {
			for item := range itemChan {
				err := fn(ctx, item)
				if err != nil {
					return err
				}
			}

			return nil
		})
	}

	// Feed items to workers.
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

	err := g.Wait()
	if err != nil {
		return fmt.Errorf("fan out failed: %w", err)
	}

	return nil
}
