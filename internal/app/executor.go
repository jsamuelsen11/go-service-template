package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jsamuelsen/go-service-template/internal/platform/logging"
)

// Transactional Service Pattern: Validate → Perform → Verify → Archive → Respond
//
// This pattern prevents data corruption when downstream dependencies fail by
// ensuring operations are verified before state is persisted.
//
// The 5 Steps:
//   1. VALIDATE  - Check all inputs and preconditions before any state changes
//   2. PERFORM   - Execute the operation (call external service, process data)
//   3. VERIFY    - Confirm the operation succeeded (never trust, always verify)
//   4. ARCHIVE   - Persist the verified state (only after verification)
//   5. RESPOND   - Return success (only after full completion)
//
// Key Benefits:
//   - Prevents partial failures from corrupting state
//   - Creates clear audit trail at each step
//   - Enables retry and recovery patterns
//   - Separates concerns for easier testing

// ExecutionStep represents a step in the transactional pattern.
type ExecutionStep string

const (
	StepValidate ExecutionStep = "validate"
	StepPerform  ExecutionStep = "perform"
	StepVerify   ExecutionStep = "verify"
	StepArchive  ExecutionStep = "archive"
	StepRespond  ExecutionStep = "respond"
)

// ExecutionError wraps errors with the step where they occurred.
type ExecutionError struct {
	Step    ExecutionStep
	Message string
	Cause   error
}

// Error implements the error interface.
func (e *ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s failed: %s: %v", e.Step, e.Message, e.Cause)
	}

	return fmt.Sprintf("%s failed: %s", e.Step, e.Message)
}

// Unwrap returns the underlying cause for errors.Is/As support.
func (e *ExecutionError) Unwrap() error {
	return e.Cause
}

// NewExecutionValidationError creates an error for the validate step.
func NewExecutionValidationError(message string, cause error) error {
	return &ExecutionError{Step: StepValidate, Message: message, Cause: cause}
}

// NewPerformError creates an error for the perform step.
func NewPerformError(message string, cause error) error {
	return &ExecutionError{Step: StepPerform, Message: message, Cause: cause}
}

// NewVerifyError creates an error for the verify step.
func NewVerifyError(message string, cause error) error {
	return &ExecutionError{Step: StepVerify, Message: message, Cause: cause}
}

// NewArchiveError creates an error for the archive step.
func NewArchiveError(message string, cause error) error {
	return &ExecutionError{Step: StepArchive, Message: message, Cause: cause}
}

// Executor runs operations using the transactional pattern.
// It provides logging and error handling at each step.
type Executor struct {
	logger *slog.Logger
}

// NewExecutor creates a new executor with the given logger.
func NewExecutor(logger *slog.Logger) *Executor {
	if logger == nil {
		logger = slog.Default()
	}

	return &Executor{logger: logger}
}

// ExecuteFunc is the signature for operation functions in the transactional pattern.
type ExecuteFunc[I, O any] func(ctx context.Context, input I) (O, error)

// Operation defines the functions for each step of the transactional pattern.
type Operation[I, P, V, O any] struct {
	// Name identifies this operation for logging.
	Name string

	// Validate checks inputs and preconditions.
	// Return an error to abort before any state changes.
	Validate func(ctx context.Context, input I) error

	// Perform executes the main operation.
	// This might call an external service, process data, etc.
	Perform func(ctx context.Context, input I) (P, error)

	// Verify confirms the operation succeeded.
	// Never trust Perform's return value - always verify independently.
	Verify func(ctx context.Context, input I, performed P) (V, error)

	// Archive persists the verified state.
	// Only called after successful verification.
	Archive func(ctx context.Context, input I, verified V) error

	// Respond transforms the result for the caller.
	// Called only after successful completion of all steps.
	Respond func(ctx context.Context, input I, verified V) (O, error)
}

// executionContext holds state during operation execution.
type executionContext[I, P, V, O any] struct {
	logger *slog.Logger
	op     Operation[I, P, V, O]
	input  I
}

// runValidate executes the validate step.
func (e *executionContext[I, P, V, O]) runValidate(ctx context.Context) error {
	if e.op.Validate == nil {
		return nil
	}

	e.logger.DebugContext(ctx, "starting validation")

	err := e.op.Validate(ctx, e.input)
	if err != nil {
		e.logger.WarnContext(ctx, "validation failed", slog.Any("error", err))

		return NewExecutionValidationError("input validation failed", err)
	}

	e.logger.DebugContext(ctx, "validation passed")

	return nil
}

// runPerform executes the perform step.
func (e *executionContext[I, P, V, O]) runPerform(ctx context.Context) (P, error) {
	var zero P

	if e.op.Perform == nil {
		return zero, nil
	}

	e.logger.DebugContext(ctx, "performing operation")

	performed, err := e.op.Perform(ctx, e.input)
	if err != nil {
		e.logger.ErrorContext(ctx, "perform failed", slog.Any("error", err))

		return zero, NewPerformError("operation failed", err)
	}

	e.logger.DebugContext(ctx, "operation performed")

	return performed, nil
}

// runVerify executes the verify step.
func (e *executionContext[I, P, V, O]) runVerify(ctx context.Context, performed P) (V, error) {
	var zero V

	if e.op.Verify == nil {
		return zero, nil
	}

	e.logger.DebugContext(ctx, "verifying result")

	verified, err := e.op.Verify(ctx, e.input, performed)
	if err != nil {
		e.logger.ErrorContext(ctx, "verification failed", slog.Any("error", err))

		return zero, NewVerifyError("verification failed", err)
	}

	e.logger.DebugContext(ctx, "result verified")

	return verified, nil
}

// runArchive executes the archive step.
func (e *executionContext[I, P, V, O]) runArchive(ctx context.Context, verified V) error {
	if e.op.Archive == nil {
		return nil
	}

	e.logger.DebugContext(ctx, "archiving state")

	err := e.op.Archive(ctx, e.input, verified)
	if err != nil {
		e.logger.ErrorContext(ctx, "archive failed", slog.Any("error", err))

		return NewArchiveError("state persistence failed", err)
	}

	e.logger.DebugContext(ctx, "state archived")

	return nil
}

// runRespond executes the respond step.
func (e *executionContext[I, P, V, O]) runRespond(ctx context.Context, verified V) (O, error) {
	var zero O

	if e.op.Respond == nil {
		return zero, nil
	}

	e.logger.DebugContext(ctx, "preparing response")

	result, err := e.op.Respond(ctx, e.input, verified)
	if err != nil {
		e.logger.WarnContext(ctx, "respond formatting failed", slog.Any("error", err))

		return zero, err
	}

	return result, nil
}

// Execute runs an operation through the full transactional pattern.
func Execute[I, P, V, O any](ctx context.Context, exec *Executor, op Operation[I, P, V, O], input I) (O, error) {
	var zero O

	logger := logging.FromContext(ctx)
	if logger == nil {
		logger = exec.logger
	}

	logger = logger.With(slog.String("operation", op.Name))
	start := time.Now()

	ec := &executionContext[I, P, V, O]{
		logger: logger,
		op:     op,
		input:  input,
	}

	// Step 1: Validate.
	err := ec.runValidate(ctx)
	if err != nil {
		return zero, err
	}

	// Step 2: Perform.
	performed, err := ec.runPerform(ctx)
	if err != nil {
		return zero, err
	}

	// Step 3: Verify.
	verified, err := ec.runVerify(ctx, performed)
	if err != nil {
		return zero, err
	}

	// Step 4: Archive.
	err = ec.runArchive(ctx, verified)
	if err != nil {
		return zero, err
	}

	// Step 5: Respond.
	result, err := ec.runRespond(ctx, verified)
	if err != nil {
		return zero, err
	}

	duration := time.Since(start)
	logger.InfoContext(ctx, "operation completed",
		slog.Duration("duration", duration),
	)

	return result, nil
}

// IsExecutionError checks if an error occurred during execution.
func IsExecutionError(err error) bool {
	var execErr *ExecutionError

	return errors.As(err, &execErr)
}

// GetExecutionStep extracts the step from an execution error.
func GetExecutionStep(err error) (ExecutionStep, bool) {
	var execErr *ExecutionError
	if errors.As(err, &execErr) {
		return execErr.Step, true
	}

	return "", false
}
