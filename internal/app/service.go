// Package app contains application services that orchestrate use cases.
// This is the application layer in Clean Architecture - it coordinates
// domain logic and infrastructure through ports.
//
// Application Layer Responsibilities:
//   - Orchestrate use cases (business workflows)
//   - Coordinate between domain and infrastructure
//   - Handle cross-cutting concerns (logging, transactions)
//   - Enforce business rules that span multiple entities
//
// What does NOT belong here:
//   - HTTP/gRPC specifics (that's adapters)
//   - Database queries (that's repository adapters)
//   - Domain logic (that's the domain layer)
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
	"github.com/jsamuelsen/go-service-template/internal/ports"
)

// Service is the main application service that orchestrates use cases.
// It depends on port interfaces, not concrete implementations.
//
// Example usage:
//
//	// In main.go or wire setup
//	repo := postgres.NewRepository(db)
//	client := http.NewExternalClient(httpClient)
//	flags := launchdarkly.NewClient(ldClient)
//	svc := app.NewService(repo, client, flags, logger)
//
//	// In HTTP handler
//	result, err := svc.ProcessOrder(ctx, orderInput)
type Service struct {
	// Port dependencies - inject via constructor.
	repo   ports.ExampleRepository
	client ports.ExampleClient
	flags  ports.FeatureFlags

	// Infrastructure.
	logger   *slog.Logger
	executor *Executor
}

// ServiceConfig holds optional configuration for the service.
type ServiceConfig struct {
	Logger *slog.Logger
}

// NewService creates a new application service with the given dependencies.
// All port dependencies are required; logger is optional (defaults to slog.Default).
func NewService(
	repo ports.ExampleRepository,
	client ports.ExampleClient,
	flags ports.FeatureFlags,
	cfg *ServiceConfig,
) *Service {
	logger := slog.Default()
	if cfg != nil && cfg.Logger != nil {
		logger = cfg.Logger
	}

	return &Service{
		repo:     repo,
		client:   client,
		flags:    flags,
		logger:   logger.With(slog.String("component", "app.Service")),
		executor: NewExecutor(logger),
	}
}

// ProcessExample demonstrates a use case using the transactional pattern.
// This is a skeleton showing the recommended structure.
//
// Use case pattern:
//  1. Get logger from context (includes request ID, trace ID)
//  2. Check feature flags if applicable
//  3. Execute operation using transactional pattern
//  4. Return domain result (not HTTP response)
func (s *Service) ProcessExample(ctx context.Context, id string) (*ports.ExampleData, error) {
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = s.logger
	}

	logger = logger.With(slog.String("use_case", "ProcessExample"), slog.String("id", id))

	// Check feature flag for new behavior.
	if s.flags != nil && s.flags.IsEnabled(ctx, "use-new-processing", false) {
		logger.DebugContext(ctx, "using new processing path")

		return s.processExampleNew(ctx, id)
	}

	// Execute using transactional pattern.
	return Execute(ctx, s.executor, Operation[string, *ports.ExampleData, *ports.ExampleData, *ports.ExampleData]{
		Name: "ProcessExample",

		Validate: func(ctx context.Context, id string) error {
			if id == "" {
				return NewExecutionValidationError("id is required", nil)
			}

			return nil
		},

		Perform: func(ctx context.Context, id string) (*ports.ExampleData, error) {
			// Fetch from external service.
			return s.client.Fetch(ctx, id)
		},

		Verify: func(ctx context.Context, id string, data *ports.ExampleData) (*ports.ExampleData, error) {
			// Verify the data is valid.
			if data == nil || data.ID != id {
				return nil, NewVerifyError("data mismatch", nil)
			}

			return data, nil
		},

		Archive: func(ctx context.Context, id string, data *ports.ExampleData) error {
			// Persist to repository.
			return s.repo.Save(ctx, &ports.ExampleEntity{ID: data.ID})
		},

		Respond: func(ctx context.Context, id string, data *ports.ExampleData) (*ports.ExampleData, error) {
			return data, nil
		},
	}, id)
}

// GetExample demonstrates a simple read use case.
// Not all operations need the full transactional pattern.
func (s *Service) GetExample(ctx context.Context, id string) (*ports.ExampleEntity, error) {
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = s.logger
	}

	logger.DebugContext(ctx, "fetching entity", slog.String("id", id))

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting example by id: %w", err)
	}

	return entity, nil
}

// DeleteExample demonstrates a delete use case with validation.
func (s *Service) DeleteExample(ctx context.Context, id string) error {
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = s.logger
	}

	logger = logger.With(slog.String("use_case", "DeleteExample"), slog.String("id", id))

	// Validate entity exists before deleting.
	_, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("checking example exists: %w", err)
	}

	logger.InfoContext(ctx, "deleting entity")

	err = s.repo.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting example: %w", err)
	}

	return nil
}

// processExampleNew is an alternative implementation behind a feature flag.
func (s *Service) processExampleNew(ctx context.Context, id string) (*ports.ExampleData, error) {
	// New implementation would go here.
	// This demonstrates how feature flags enable gradual rollouts.
	data, err := s.client.Fetch(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching example (new path): %w", err)
	}

	return data, nil
}
