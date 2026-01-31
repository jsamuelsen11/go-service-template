// Package ports defines interfaces for external dependencies.
// Ports are contracts that adapters implement, allowing the application layer
// to depend on abstractions rather than concrete implementations.
//
// Port Design Principles:
//   - Context as first parameter (always) for cancellation and deadlines
//   - Return domain types, never external DTOs or infrastructure types
//   - Error returns use domain error types (ErrNotFound, ErrConflict, etc.)
//   - Methods represent business operations, not CRUD operations
//   - Keep interfaces small and focused (Interface Segregation Principle)
package ports

import (
	"context"
)

// ExampleRepository demonstrates the port pattern for data persistence.
// Real implementations would define domain-specific repository interfaces.
//
// Example usage in application layer:
//
//	type Service struct {
//	    repo ports.ExampleRepository
//	}
//
//	func NewService(repo ports.ExampleRepository) *Service {
//	    return &Service{repo: repo}
//	}
type ExampleRepository interface {
	// GetByID retrieves an entity by its identifier.
	// Returns domain.ErrNotFound if the entity does not exist.
	GetByID(ctx context.Context, id string) (*ExampleEntity, error)

	// Save persists an entity, creating or updating as appropriate.
	// Returns domain.ErrConflict if there's a version mismatch.
	Save(ctx context.Context, entity *ExampleEntity) error

	// Delete removes an entity by its identifier.
	// Returns domain.ErrNotFound if the entity does not exist.
	Delete(ctx context.Context, id string) error
}

// ExampleEntity is a placeholder domain entity for the example repository.
// Replace with actual domain entities in real implementations.
type ExampleEntity struct {
	ID      string
	Version int
}

// ExampleClient demonstrates the port pattern for external service calls.
// Adapters implement this interface to integrate with downstream services.
//
// Key considerations:
//   - Handle timeouts via context deadline
//   - Map external errors to domain errors
//   - Transform external DTOs to domain types
type ExampleClient interface {
	// Fetch retrieves data from an external service.
	// The implementation should respect context deadlines and cancellation.
	// Returns domain.ErrUnavailable if the service is unreachable.
	Fetch(ctx context.Context, id string) (*ExampleData, error)
}

// ExampleData is a placeholder for data returned from external services.
// This should be a domain type, not an external DTO.
type ExampleData struct {
	ID    string
	Value string
}

// EventPublisher defines the contract for publishing domain events.
// Implementations may use message queues, event buses, or other mechanisms.
type EventPublisher interface {
	// Publish sends an event to the configured destination.
	// Returns domain.ErrUnavailable if the messaging system is unreachable.
	Publish(ctx context.Context, event Event) error
}

// Event represents a domain event that can be published.
type Event interface {
	// EventType returns the type identifier for routing.
	EventType() string

	// Payload returns the event data for serialization.
	Payload() any
}

// Cache defines the contract for caching operations.
// Implementations may use Redis, Memcached, or in-memory caches.
type Cache interface {
	// Get retrieves a value from the cache.
	// Returns domain.ErrNotFound if the key does not exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with optional TTL.
	// A TTL of 0 means no expiration.
	Set(ctx context.Context, key string, value []byte, ttlSeconds int) error

	// Delete removes a value from the cache.
	// Does not return an error if the key does not exist.
	Delete(ctx context.Context, key string) error
}
