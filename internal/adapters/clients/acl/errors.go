package acl

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/jsamuelsen/go-service-template/internal/adapters/clients"
	"github.com/jsamuelsen/go-service-template/internal/domain"
)

// ErrorResponse represents a standard error response from external services.
// It supports both nested format (error.code/message) and flat format (code/message).
type ErrorResponse struct {
	Error   ErrorDetail `json:"error"`
	Code    string      `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ErrorDetail contains error information from external services.
type ErrorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// GetCode returns the error code from either nested or top-level format.
func (e *ErrorResponse) GetCode() string {
	if e.Error.Code != "" {
		return e.Error.Code
	}

	return e.Code
}

// GetMessage returns the error message from either nested or top-level format.
func (e *ErrorResponse) GetMessage() string {
	if e.Error.Message != "" {
		return e.Error.Message
	}

	return e.Message
}

// Common external error codes that map to domain errors.
// Add service-specific codes as needed.
const (
	// ExternalCodeNotFound indicates the resource was not found.
	ExternalCodeNotFound = "NOT_FOUND"
	// ExternalCodeConflict indicates a state conflict.
	ExternalCodeConflict = "CONFLICT"
	// ExternalCodeValidation indicates validation failed.
	ExternalCodeValidation = "VALIDATION_ERROR"
	// ExternalCodeForbidden indicates the operation is not allowed.
	ExternalCodeForbidden = "FORBIDDEN"
	// ExternalCodeUnauthorized indicates authentication is required.
	ExternalCodeUnauthorized = "UNAUTHORIZED"
)

// ParseErrorResponse attempts to parse an error response body.
// Returns nil if the body is empty or cannot be parsed.
func ParseErrorResponse(body io.Reader) *ErrorResponse {
	if body == nil {
		return nil
	}

	var errResp ErrorResponse
	if err := json.NewDecoder(body).Decode(&errResp); err != nil {
		return nil
	}

	// Check if we got any meaningful data
	if errResp.GetCode() == "" && errResp.GetMessage() == "" {
		return nil
	}

	return &errResp
}

// MapHTTPError maps an HTTP response to a domain error.
// This function handles:
//   - HTTP status codes → domain errors
//   - Error response body parsing for additional context
//   - Client-level errors (circuit breaker, retries exhausted)
//
// Parameters:
//   - resp: The HTTP response (may be nil for transport errors)
//   - clientErr: Any error from the HTTP client (may be nil)
//   - serviceName: Name of the external service for error context
//   - operation: The operation being performed (e.g., "get user", "create order")
//   - entityID: The ID of the entity being operated on (used for NotFoundError)
//
// Returns a domain error appropriate for the failure type.
func MapHTTPError(resp *http.Response, clientErr error, serviceName, operation, entityID string) error {
	// Handle client-level errors first (no response received)
	if clientErr != nil {
		return mapClientError(clientErr, serviceName, operation)
	}

	if resp == nil {
		return domain.NewUnavailableError(serviceName, "no response received")
	}

	// Success responses should not call this function
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}

	// Parse error body for additional context
	var errResp *ErrorResponse
	if resp.Body != nil {
		errResp = ParseErrorResponse(resp.Body)
	}

	return mapStatusCode(resp.StatusCode, errResp, serviceName, operation, entityID)
}

// mapClientError translates client-level errors to domain errors.
func mapClientError(err error, serviceName, operation string) error {
	switch {
	case errors.Is(err, clients.ErrCircuitOpen):
		return domain.NewUnavailableError(serviceName,
			fmt.Sprintf("circuit breaker open during %s", operation))

	case errors.Is(err, clients.ErrMaxRetriesExceeded):
		return domain.NewUnavailableError(serviceName,
			fmt.Sprintf("max retries exceeded during %s", operation))

	default:
		return domain.NewUnavailableError(serviceName,
			fmt.Sprintf("%s failed: %v", operation, err))
	}
}

// mapStatusCode translates HTTP status codes to domain errors.
func mapStatusCode(status int, errResp *ErrorResponse, serviceName, operation, entityID string) error {
	// Get message from error response or use default
	message := defaultMessageForStatus(status, operation)
	if errResp != nil && errResp.GetMessage() != "" {
		message = errResp.GetMessage()
	}

	switch status {
	case http.StatusNotFound:
		return domain.NewNotFoundError(serviceName, entityID)

	case http.StatusConflict:
		return domain.NewConflictError(serviceName, message)

	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		// Check for field-level validation details
		if errResp != nil && errResp.Error.Details != nil {
			// Return first validation error (or aggregate as needed)
			for field, msg := range errResp.Error.Details {
				return domain.NewValidationError(field, msg)
			}
		}

		return domain.NewValidationError("", message)

	case http.StatusForbidden:
		return domain.NewForbiddenError(operation, message)

	case http.StatusUnauthorized:
		return domain.NewForbiddenError(operation, "authentication required")

	case http.StatusTooManyRequests:
		return domain.NewUnavailableError(serviceName, "rate limit exceeded")

	case http.StatusServiceUnavailable, http.StatusBadGateway, http.StatusGatewayTimeout:
		return domain.NewUnavailableError(serviceName, message)

	default:
		if status >= http.StatusInternalServerError {
			return domain.NewUnavailableError(serviceName, message)
		}
		// Unknown 4xx errors default to validation
		return domain.NewValidationError("", message)
	}
}

// defaultMessageForStatus returns a default message for an HTTP status.
func defaultMessageForStatus(status int, operation string) string {
	switch status {
	case http.StatusNotFound:
		return "resource not found"
	case http.StatusConflict:
		return "resource conflict"
	case http.StatusBadRequest:
		return "invalid request"
	case http.StatusForbidden:
		return "access denied"
	case http.StatusUnauthorized:
		return "authentication required"
	case http.StatusTooManyRequests:
		return "rate limit exceeded"
	case http.StatusServiceUnavailable:
		return "service temporarily unavailable"
	default:
		return fmt.Sprintf("%s failed with status %d", operation, status)
	}
}

// MapExternalCode maps an external error code to a domain error.
// Use this when the external service uses specific error codes in its response body.
func MapExternalCode(code, message, serviceName, operation, entityID string) error {
	switch code {
	case ExternalCodeNotFound:
		return domain.NewNotFoundError(serviceName, entityID)
	case ExternalCodeConflict:
		return domain.NewConflictError(serviceName, message)
	case ExternalCodeValidation:
		return domain.NewValidationError("", message)
	case ExternalCodeForbidden:
		return domain.NewForbiddenError(operation, message)
	case ExternalCodeUnauthorized:
		return domain.NewForbiddenError(operation, "authentication required")
	default:
		return domain.NewUnavailableError(serviceName, message)
	}
}
