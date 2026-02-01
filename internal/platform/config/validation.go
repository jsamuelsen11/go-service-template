package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// validate is the package-level validator instance.
var validate = validator.New(validator.WithRequiredStructEnabled())

// Validate validates the configuration and returns an error if invalid.
// Validation fails fast - the service should not start with invalid config.
func (c *Config) Validate() error {
	err := validate.Struct(c)
	if err != nil {
		return formatValidationErrors(err)
	}

	return nil
}

// formatValidationErrors converts validator errors to a readable format.
func formatValidationErrors(err error) error {
	var validationErrors validator.ValidationErrors

	ok := errors.As(err, &validationErrors)
	if !ok {
		return err
	}

	errs := make([]string, 0, len(validationErrors))
	for _, e := range validationErrors {
		errs = append(errs, formatFieldError(e))
	}

	return fmt.Errorf("config validation failed:\n  %s", strings.Join(errs, "\n  "))
}

// formatFieldError formats a single field validation error.
func formatFieldError(e validator.FieldError) string {
	field := formatFieldPath(e.Namespace())

	switch e.Tag() {
	case "required":
		return field + " is required"
	case "required_if":
		return fmt.Sprintf("%s is required when %s", field, e.Param())
	case "min":
		return fmt.Sprintf("%s must be at least %s", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", field, e.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, e.Param())
	case "url":
		return field + " must be a valid URL"
	default:
		return fmt.Sprintf("%s failed validation: %s", field, e.Tag())
	}
}

// formatFieldPath converts "Config.Server.Port" to "server.port".
func formatFieldPath(namespace string) string {
	// Remove the root struct name (Config.)
	parts := strings.Split(namespace, ".")
	if len(parts) > 1 {
		parts = parts[1:]
	}

	// Convert to lowercase
	for i, part := range parts {
		parts[i] = strings.ToLower(part)
	}

	return strings.Join(parts, ".")
}
