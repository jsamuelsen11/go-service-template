package dto

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// jsonTagParts is the number of parts when splitting a JSON tag by comma.
// The first part is the field name, subsequent parts are options like "omitempty".
const jsonTagParts = 2

// Validation errors.
var (
	// ErrValidation indicates a validation failure occurred.
	ErrValidation = errors.New("validation failed")

	// ErrBinding indicates JSON or query binding failed.
	ErrBinding = errors.New("binding failed")
)

var (
	// validate is the singleton validator instance.
	validate     *validator.Validate
	validateOnce sync.Once
)

// Validator returns the singleton validator instance.
// It initializes the validator with custom validations on first call.
func Validator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New()

		// Use JSON tag names in error messages
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", jsonTagParts)[0]
			if name == "-" {
				return ""
			}

			return name
		})

		// Register custom validators
		_ = validate.RegisterValidation("uuid", validateUUID)
		_ = validate.RegisterValidation("notempty", validateNotEmpty)
	})

	return validate
}

// Validate validates a struct using the validator instance.
// Returns nil if valid, or an error containing validation failures.
func Validate(v any) error {
	err := Validator().Struct(v)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrValidation, err)
	}

	return nil
}

// BindAndValidate binds JSON body to the struct and validates it.
// Returns nil on success, or an error for binding/validation failures.
func BindAndValidate(c *gin.Context, v any) error {
	err := c.ShouldBindJSON(v)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBinding, err)
	}

	return Validate(v)
}

// BindQueryAndValidate binds query parameters and validates.
func BindQueryAndValidate(c *gin.Context, v any) error {
	err := c.ShouldBindQuery(v)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBinding, err)
	}

	return Validate(v)
}

// ValidationErrors extracts field-level error messages from a validator error.
// Returns a map of field names to error messages suitable for API responses.
func ValidationErrors(err error) map[string]string {
	fieldErrors := make(map[string]string)

	var validationErrs validator.ValidationErrors
	if errors.As(err, &validationErrs) {
		for _, fieldErr := range validationErrs {
			fieldName := fieldErr.Field()
			fieldErrors[fieldName] = validationMessage(fieldErr)
		}
	}

	return fieldErrors
}

// IsValidationError checks if the error is a validation error.
func IsValidationError(err error) bool {
	var validationErrs validator.ValidationErrors
	return errors.As(err, &validationErrs)
}

// validationMessages maps validation tags to message templates.
// Use {param} as placeholder for the validation parameter.
var validationMessages = map[string]string{
	"required": "this field is required",
	"email":    "must be a valid email address",
	"uuid":     "must be a valid UUID",
	"url":      "must be a valid URL",
	"notempty": "must not be empty",
	"gte":      "must be greater than or equal to {param}",
	"lte":      "must be less than or equal to {param}",
	"gt":       "must be greater than {param}",
	"lt":       "must be less than {param}",
	"oneof":    "must be one of: {param}",
}

// validationMessage returns a human-readable message for a validation error.
func validationMessage(fe validator.FieldError) string {
	tag := fe.Tag()
	param := fe.Param()

	// Handle min/max with type-aware messages
	if tag == "min" || tag == "max" {
		return minMaxMessage(tag, param, fe.Type().Kind())
	}

	// Look up in message map
	if msg, ok := validationMessages[tag]; ok {
		return strings.ReplaceAll(msg, "{param}", param)
	}

	return "failed validation: " + tag
}

// minMaxMessage returns the appropriate message for min/max validation.
func minMaxMessage(tag, param string, kind reflect.Kind) string {
	suffix := ""
	if kind == reflect.String {
		suffix = " characters"
	}

	if tag == "min" {
		return "must be at least " + param + suffix
	}

	return "must be at most " + param + suffix
}

// validateUUID validates that a string is a valid UUID.
func validateUUID(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	if value == "" {
		return true // Empty is ok, use 'required' tag if needed
	}

	_, err := uuid.Parse(value)

	return err == nil
}

// validateNotEmpty validates that a string is not empty after trimming whitespace.
func validateNotEmpty(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return strings.TrimSpace(value) != ""
}

// Validatable is an interface for types that can perform custom validation.
// Implement this for business rule validation beyond struct tags.
type Validatable interface {
	Validate() error
}

// ValidateAll validates struct tags and calls custom Validate() if implemented.
func ValidateAll(v any) error {
	// First validate struct tags
	err := Validate(v)
	if err != nil {
		return err
	}

	// Then call custom validation if implemented
	if validatable, ok := v.(Validatable); ok {
		err = validatable.Validate()
		if err != nil {
			return fmt.Errorf("%w: %w", ErrValidation, err)
		}
	}

	return nil
}
