// Package domain contains business logic types and errors.
// Domain errors represent business-level failures, NOT HTTP errors.
// They are infrastructure-agnostic and can be mapped to HTTP/gRPC/etc by adapters.
package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors for use with errors.Is().
var (
	// ErrNotFound indicates the requested entity does not exist.
	ErrNotFound = errors.New("not found")

	// ErrConflict indicates a state conflict such as duplicate entry or version mismatch.
	ErrConflict = errors.New("conflict")

	// ErrValidation indicates business rule validation failed.
	ErrValidation = errors.New("validation failed")

	// ErrForbidden indicates the operation is not permitted by business rules.
	ErrForbidden = errors.New("forbidden")

	// ErrUnavailable indicates a required dependency is unavailable.
	ErrUnavailable = errors.New("unavailable")
)

// NotFoundError provides context for not found errors.
type NotFoundError struct {
	Entity string
	ID     string
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("%s with id %q not found", e.Entity, e.ID)
	}

	return e.Entity + " not found"
}

// Unwrap returns the sentinel error for errors.Is() support.
func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// NewNotFoundError creates a not found error with context.
func NewNotFoundError(entity, id string) error {
	return &NotFoundError{Entity: entity, ID: id}
}

// ConflictError provides context for conflict errors.
type ConflictError struct {
	Entity  string
	Reason  string
	Details string
}

// Error implements the error interface.
func (e *ConflictError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s conflict: %s (%s)", e.Entity, e.Reason, e.Details)
	}

	return fmt.Sprintf("%s conflict: %s", e.Entity, e.Reason)
}

// Unwrap returns the sentinel error for errors.Is() support.
func (e *ConflictError) Unwrap() error {
	return ErrConflict
}

// NewConflictError creates a conflict error with context.
func NewConflictError(entity, reason string) error {
	return &ConflictError{Entity: entity, Reason: reason}
}

// NewConflictErrorWithDetails creates a conflict error with additional details.
func NewConflictErrorWithDetails(entity, reason, details string) error {
	return &ConflictError{Entity: entity, Reason: reason, Details: details}
}

// ValidationError provides context for validation errors.
type ValidationError struct {
	Field   string
	Message string
	Value   any
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
	}

	return "validation failed: " + e.Message
}

// Unwrap returns the sentinel error for errors.Is() support.
func (e *ValidationError) Unwrap() error {
	return ErrValidation
}

// NewValidationError creates a validation error with context.
func NewValidationError(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// NewValidationErrorWithValue creates a validation error including the invalid value.
func NewValidationErrorWithValue(field, message string, value any) error {
	return &ValidationError{Field: field, Message: message, Value: value}
}

// ForbiddenError provides context for forbidden errors.
type ForbiddenError struct {
	Operation string
	Reason    string
}

// Error implements the error interface.
func (e *ForbiddenError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("operation %q forbidden: %s", e.Operation, e.Reason)
	}

	return fmt.Sprintf("operation %q forbidden", e.Operation)
}

// Unwrap returns the sentinel error for errors.Is() support.
func (e *ForbiddenError) Unwrap() error {
	return ErrForbidden
}

// NewForbiddenError creates a forbidden error with context.
func NewForbiddenError(operation, reason string) error {
	return &ForbiddenError{Operation: operation, Reason: reason}
}

// UnavailableError provides context for unavailable errors.
type UnavailableError struct {
	Service string
	Reason  string
}

// Error implements the error interface.
func (e *UnavailableError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("service %q unavailable: %s", e.Service, e.Reason)
	}

	return fmt.Sprintf("service %q unavailable", e.Service)
}

// Unwrap returns the sentinel error for errors.Is() support.
func (e *UnavailableError) Unwrap() error {
	return ErrUnavailable
}

// NewUnavailableError creates an unavailable error with context.
func NewUnavailableError(service, reason string) error {
	return &UnavailableError{Service: service, Reason: reason}
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsConflict checks if an error is a conflict error.
func IsConflict(err error) bool {
	return errors.Is(err, ErrConflict)
}

// IsValidation checks if an error is a validation error.
func IsValidation(err error) bool {
	return errors.Is(err, ErrValidation)
}

// IsForbidden checks if an error is a forbidden error.
func IsForbidden(err error) bool {
	return errors.Is(err, ErrForbidden)
}

// IsUnavailable checks if an error is an unavailable error.
func IsUnavailable(err error) bool {
	return errors.Is(err, ErrUnavailable)
}
