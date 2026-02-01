// Package acl provides Anti-Corruption Layer patterns for translating between
// external service DTOs and domain types.
//
// # What is an Anti-Corruption Layer?
//
// The Anti-Corruption Layer (ACL) is a pattern from Domain-Driven Design that
// protects your domain model from external service representations. It acts as
// a translation boundary, ensuring that:
//
//   - External DTOs never leak into your domain
//   - External error codes map to domain errors
//   - External data is validated before creating domain objects
//   - Changes to external APIs don't ripple through your codebase
//
// # When to Use
//
// Create an ACL adapter when:
//   - Integrating with an external/downstream service
//   - The external service uses different naming conventions
//   - External data formats differ from your domain model
//   - You want to isolate your domain from external API changes
//
// # Package Components
//
// This package provides reusable patterns:
//
//   - [BaseAdapter]: Embeddable struct with common functionality
//   - [ErrorResponse]: Standard external error response parsing
//   - [MapHTTPError]: HTTP status code to domain error mapping
//   - [ParseErrorResponse]: JSON error body parsing
//   - [DecodeResponse]: Generic JSON response decoder
//   - [TranslateSlice]: Batch translation helper
//
// # Creating an Adapter
//
// Follow these steps to create a new service adapter:
//
//  1. Define external DTOs (unexported, in your adapter file)
//  2. Embed [BaseAdapter] for common functionality
//  3. Implement translation methods that validate and convert
//  4. Use [MapHTTPError] for consistent error handling
//
// Example adapter structure:
//
//	type MyServiceAdapter struct {
//	    acl.BaseAdapter
//	}
//
//	func NewMyServiceAdapter(client *clients.Client) *MyServiceAdapter {
//	    return &MyServiceAdapter{
//	        BaseAdapter: acl.NewBaseAdapter(client, "my-service"),
//	    }
//	}
//
//	// externalResponse is the DTO from the external service (unexported)
//	type externalResponse struct {
//	    ID   string `json:"id"`
//	    Name string `json:"name"`
//	}
//
//	func (a *MyServiceAdapter) Get(ctx context.Context, id string) (*domain.Entity, error) {
//	    body, err := a.BaseAdapter.Get(ctx, "/api/v1/items/"+id, "get item")
//	    if err != nil {
//	        return nil, err // Already a domain error
//	    }
//
//	    ext, err := acl.DecodeResponse[externalResponse](body)
//	    if err != nil {
//	        return nil, domain.NewUnavailableError(a.ServiceName(), err.Error())
//	    }
//
//	    return a.translate(ext)
//	}
//
// See [UserServiceAdapter] for a complete working example.
//
// # Error Handling Strategy
//
// External services return errors in various formats:
//   - HTTP status codes (4xx, 5xx)
//   - Error response bodies with codes and messages
//   - Network/transport errors
//
// The ACL translates all of these to domain errors:
//   - 404 Not Found → [domain.ErrNotFound]
//   - 409 Conflict → [domain.ErrConflict]
//   - 400/422 Validation → [domain.ErrValidation]
//   - 401/403 Forbidden → [domain.ErrForbidden]
//   - 5xx/Network → [domain.ErrUnavailable]
//
// Client-level errors ([clients.ErrCircuitOpen], [clients.ErrMaxRetriesExceeded])
// are also translated to [domain.ErrUnavailable] with appropriate context.
package acl
