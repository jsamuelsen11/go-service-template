// Package dto provides Data Transfer Objects for HTTP request/response handling.
package dto

import "net/http"

// ErrorResponse is the standard error envelope for all error responses.
// It provides a consistent structure for API error handling.
type ErrorResponse struct {
	Error   ErrorDetail `json:"error"`
	TraceID string      `json:"traceId,omitempty"`
}

// ErrorDetail contains the error information.
type ErrorDetail struct {
	// Code is a machine-readable error code (e.g., "NOT_FOUND", "VALIDATION_ERROR").
	Code string `json:"code"`

	// Message is a human-readable error message.
	Message string `json:"message"`

	// Details provides additional context about the error.
	// For validation errors, this contains field-level error messages.
	Details map[string]string `json:"details,omitempty"`
}

// Error codes for machine-readable error identification.
const (
	// ErrorCodeNotFound indicates the requested resource was not found.
	ErrorCodeNotFound = "NOT_FOUND"

	// ErrorCodeConflict indicates a state conflict (duplicate, version mismatch).
	ErrorCodeConflict = "CONFLICT"

	// ErrorCodeValidation indicates request validation failed.
	ErrorCodeValidation = "VALIDATION_ERROR"

	// ErrorCodeForbidden indicates the operation is not permitted.
	ErrorCodeForbidden = "FORBIDDEN"

	// ErrorCodeUnauthorized indicates authentication is required.
	ErrorCodeUnauthorized = "UNAUTHORIZED"

	// ErrorCodeUnavailable indicates a dependency is unavailable.
	ErrorCodeUnavailable = "SERVICE_UNAVAILABLE"

	// ErrorCodeInternal indicates an internal server error.
	ErrorCodeInternal = "INTERNAL_ERROR"

	// ErrorCodeTimeout indicates the request timed out.
	ErrorCodeTimeout = "TIMEOUT"

	// ErrorCodeBadRequest indicates the request was malformed.
	ErrorCodeBadRequest = "BAD_REQUEST"
)

// NewErrorResponse creates a new error response with the given code and message.
func NewErrorResponse(code, message string) *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}

// NewErrorResponseWithDetails creates an error response with additional details.
func NewErrorResponseWithDetails(code, message string, details map[string]string) *ErrorResponse {
	return &ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}
}

// WithTraceID adds a trace ID to the error response.
func (e *ErrorResponse) WithTraceID(traceID string) *ErrorResponse {
	e.TraceID = traceID
	return e
}

// HTTPStatusFromCode maps error codes to HTTP status codes.
func HTTPStatusFromCode(code string) int {
	switch code {
	case ErrorCodeNotFound:
		return http.StatusNotFound
	case ErrorCodeConflict:
		return http.StatusConflict
	case ErrorCodeValidation, ErrorCodeBadRequest:
		return http.StatusBadRequest
	case ErrorCodeForbidden:
		return http.StatusForbidden
	case ErrorCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrorCodeUnavailable:
		return http.StatusServiceUnavailable
	case ErrorCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}
