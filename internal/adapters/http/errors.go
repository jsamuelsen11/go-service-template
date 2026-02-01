package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"github.com/jsamuelsen/go-service-template/internal/adapters/http/dto"
	"github.com/jsamuelsen/go-service-template/internal/domain"
	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

// MapDomainError maps a domain error to an HTTP status code and error response.
// Unknown errors are mapped to 500 Internal Server Error with a generic message.
func MapDomainError(err error) (int, *dto.ErrorResponse) {
	if err == nil {
		return http.StatusOK, nil
	}

	switch {
	case domain.IsNotFound(err):
		return http.StatusNotFound, dto.NewErrorResponse(
			dto.ErrorCodeNotFound,
			err.Error(),
		)

	case domain.IsConflict(err):
		return http.StatusConflict, dto.NewErrorResponse(
			dto.ErrorCodeConflict,
			err.Error(),
		)

	case domain.IsValidation(err):
		resp := dto.NewErrorResponse(
			dto.ErrorCodeValidation,
			err.Error(),
		)
		// Extract field details if available
		var validationErr *domain.ValidationError
		if errors.As(err, &validationErr) && validationErr.Field != "" {
			resp.Error.Details = map[string]string{
				validationErr.Field: validationErr.Message,
			}
		}

		return http.StatusBadRequest, resp

	case domain.IsForbidden(err):
		return http.StatusForbidden, dto.NewErrorResponse(
			dto.ErrorCodeForbidden,
			err.Error(),
		)

	case domain.IsUnavailable(err):
		return http.StatusServiceUnavailable, dto.NewErrorResponse(
			dto.ErrorCodeUnavailable,
			err.Error(),
		)

	default:
		// Unknown errors get a generic message to avoid leaking internals
		return http.StatusInternalServerError, dto.NewErrorResponse(
			dto.ErrorCodeInternal,
			"an internal error occurred",
		)
	}
}

// RespondWithError writes an error response to the gin.Context.
// It maps domain errors to HTTP responses and includes the trace ID if available.
func RespondWithError(c *gin.Context, err error) {
	status, errResp := MapDomainError(err)

	// Add trace ID if available from OpenTelemetry
	if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
		errResp.TraceID = span.SpanContext().TraceID().String()
	}

	// Log internal errors with full details
	if status == http.StatusInternalServerError {
		logger := logging.FromContext(c.Request.Context())
		logger.Error("internal error",
			"error", err.Error(),
			"trace_id", errResp.TraceID,
		)
	}

	c.JSON(status, errResp)
}

// RespondWithErrorCode writes an error response with a specific error code.
// Use this for adapter-level errors (e.g., validation, bad request) that
// don't originate from domain errors.
func RespondWithErrorCode(c *gin.Context, code, message string) {
	errResp := dto.NewErrorResponse(code, message)

	// Add trace ID if available
	if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
		errResp.TraceID = span.SpanContext().TraceID().String()
	}

	status := dto.HTTPStatusFromCode(code)
	c.JSON(status, errResp)
}

// RespondWithValidationErrors writes a 400 response with field-level validation errors.
func RespondWithValidationErrors(c *gin.Context, fieldErrors map[string]string) {
	errResp := dto.NewErrorResponseWithDetails(
		dto.ErrorCodeValidation,
		"request validation failed",
		fieldErrors,
	)

	// Add trace ID if available
	if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
		errResp.TraceID = span.SpanContext().TraceID().String()
	}

	c.JSON(http.StatusBadRequest, errResp)
}

// AbortWithError aborts the request chain and writes an error response.
// Use this in middleware when you want to stop further processing.
func AbortWithError(c *gin.Context, err error) {
	status, errResp := MapDomainError(err)

	// Add trace ID if available
	if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
		errResp.TraceID = span.SpanContext().TraceID().String()
	}

	c.AbortWithStatusJSON(status, errResp)
}

// AbortWithErrorCode aborts the request chain with a specific error code.
func AbortWithErrorCode(c *gin.Context, code, message string) {
	errResp := dto.NewErrorResponse(code, message)

	// Add trace ID if available
	if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().HasTraceID() {
		errResp.TraceID = span.SpanContext().TraceID().String()
	}

	status := dto.HTTPStatusFromCode(code)
	c.AbortWithStatusJSON(status, errResp)
}
