package acl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/domain"
)

// BaseAdapter provides common functionality for ACL adapters.
// Embed this in your service-specific adapters.
type BaseAdapter struct {
	client      *clients.Client
	serviceName string
}

// NewBaseAdapter creates a new base adapter with the given client and service name.
func NewBaseAdapter(client *clients.Client, serviceName string) BaseAdapter {
	return BaseAdapter{
		client:      client,
		serviceName: serviceName,
	}
}

// Client returns the underlying HTTP client.
func (a *BaseAdapter) Client() *clients.Client {
	return a.client
}

// ServiceName returns the name of the external service.
func (a *BaseAdapter) ServiceName() string {
	return a.serviceName
}

// DoRequest executes an HTTP request and handles error mapping.
// On success, returns the response body reader (caller must close).
// On failure, returns a mapped domain error.
func (a *BaseAdapter) DoRequest(ctx context.Context, req *http.Request, operation string) (io.ReadCloser, error) {
	resp, err := a.client.Do(ctx, req)
	if err != nil {
		return nil, MapHTTPError(nil, err, a.serviceName, operation)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer func() { _ = resp.Body.Close() }()

		return nil, MapHTTPError(resp, nil, a.serviceName, operation)
	}

	return resp.Body, nil
}

// Get performs a GET request and returns the response body.
// The path should be an absolute path starting with "/".
func (a *BaseAdapter) Get(ctx context.Context, path, operation string) (io.ReadCloser, error) {
	resp, err := a.client.Get(ctx, path)
	if err != nil {
		return nil, MapHTTPError(nil, err, a.serviceName, operation)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer func() { _ = resp.Body.Close() }()

		return nil, MapHTTPError(resp, nil, a.serviceName, operation)
	}

	return resp.Body, nil
}

// Post performs a POST request and returns the response body.
func (a *BaseAdapter) Post(ctx context.Context, path string, body io.Reader, operation string) (io.ReadCloser, error) {
	resp, err := a.client.Post(ctx, path, body)
	if err != nil {
		return nil, MapHTTPError(nil, err, a.serviceName, operation)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer func() { _ = resp.Body.Close() }()

		return nil, MapHTTPError(resp, nil, a.serviceName, operation)
	}

	return resp.Body, nil
}

// DecodeResponse reads and decodes a JSON response body into the target type.
// Closes the body after reading.
func DecodeResponse[T any](body io.ReadCloser) (*T, error) {
	if body == nil {
		return nil, fmt.Errorf("response body is nil")
	}
	defer func() { _ = body.Close() }()

	var result T
	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	return &result, nil
}

// ValidateRequired checks that a required field is not empty.
// Returns a domain.ValidationError if the field is empty.
func ValidateRequired(value, fieldName string) error {
	if value == "" {
		return domain.NewValidationError(fieldName, "is required")
	}

	return nil
}

// ValidatePositive checks that a numeric value is positive.
// Returns a domain.ValidationError if the value is not positive.
func ValidatePositive[T ~int | ~int64 | ~float64](value T, fieldName string) error {
	if value <= 0 {
		return domain.NewValidationError(fieldName, "must be positive")
	}

	return nil
}

// Translator is a function type that translates an external DTO to a domain type.
// The function should validate the external data and return a domain error
// if validation fails.
type Translator[External any, Domain any] func(ext *External) (*Domain, error)

// TranslateSlice applies a translator function to a slice of external DTOs.
// If any translation fails, returns the first error encountered.
func TranslateSlice[E any, D any](items []E, translate Translator[E, D]) ([]*D, error) {
	result := make([]*D, 0, len(items))

	for i := range items {
		translated, err := translate(&items[i])
		if err != nil {
			return nil, fmt.Errorf("translating item %d: %w", i, err)
		}

		result = append(result, translated)
	}

	return result, nil
}

// TranslateMap applies a translator function to a map of external DTOs.
func TranslateMap[E any, D any](items map[string]E, translate Translator[E, D]) (map[string]*D, error) {
	result := make(map[string]*D, len(items))

	for key, item := range items {
		translated, err := translate(&item)
		if err != nil {
			return nil, fmt.Errorf("translating key %s: %w", key, err)
		}

		result[key] = translated
	}

	return result, nil
}
