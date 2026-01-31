// Package app contains application services that orchestrate use cases.
// This is the application layer in Clean Architecture - it coordinates
// domain logic and infrastructure through ports.
//
// Application Layer Responsibilities:
//   - Orchestrate use cases (business workflows)
//   - Coordinate between domain and infrastructure
//   - Handle cross-cutting concerns (logging)
//   - Enforce business rules that span multiple entities
//
// What does NOT belong here:
//   - HTTP/gRPC specifics (that's adapters)
//   - Database queries (that's repository adapters)
//   - Core domain logic (that's the domain layer)
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
	"github.com/jsamuelsen/go-service-template/internal/ports"
)

// Service orchestrates use cases using injected dependencies.
// It depends on port interfaces, not concrete implementations.
//
// Example usage:
//
//	// In main.go or wire setup
//	repo := postgres.NewRepository(db)
//	client := http.NewExternalClient(httpClient)
//	flags := launchdarkly.NewClient(ldClient)
//	svc := app.NewService(repo, client, flags, &app.ServiceConfig{Logger: logger})
//
//	// In HTTP handler
//	result, err := svc.ProcessExample(ctx, id)
type Service struct {
	repo   ports.ExampleRepository
	client ports.ExampleClient
	flags  ports.FeatureFlags
	logger *slog.Logger
}

// ServiceConfig holds optional configuration for the service.
type ServiceConfig struct {
	Logger *slog.Logger
}

// NewService creates a new application service with the given dependencies.
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
		repo:   repo,
		client: client,
		flags:  flags,
		logger: logger.With(slog.String("component", "app.Service")),
	}
}

// ProcessExample demonstrates a typical use case.
// It validates input, fetches from external service, and persists.
func (s *Service) ProcessExample(ctx context.Context, id string) (*ports.ExampleData, error) {
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = s.logger
	}

	logger = logger.With(slog.String("method", "ProcessExample"), slog.String("id", id))

	// Validate input.
	if id == "" {
		return nil, fmt.Errorf("validating input: %w", domain.NewValidationError("id", "cannot be empty"))
	}

	// Check feature flag for new behavior (example).
	if s.flags != nil && s.flags.IsEnabled(ctx, "use-new-processing", false) {
		logger.DebugContext(ctx, "using new processing path")
	}

	// Fetch from external service.
	data, err := s.client.Fetch(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetching data: %w", err)
	}

	// Persist to repository.
	err = s.repo.Save(ctx, &ports.ExampleEntity{ID: data.ID})
	if err != nil {
		return nil, fmt.Errorf("saving entity: %w", err)
	}

	logger.InfoContext(ctx, "processed successfully")

	return data, nil
}

// GetExample retrieves an entity by ID.
func (s *Service) GetExample(ctx context.Context, id string) (*ports.ExampleEntity, error) {
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = s.logger
	}

	logger.DebugContext(ctx, "fetching entity", slog.String("id", id))

	entity, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting example: %w", err)
	}

	return entity, nil
}

// DeleteExample removes an entity by ID.
func (s *Service) DeleteExample(ctx context.Context, id string) error {
	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = s.logger
	}

	logger = logger.With(slog.String("method", "DeleteExample"), slog.String("id", id))

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
