package execution

import "fmt"

// ExecutionError represents domain-specific errors for execution
type ExecutionError struct {
	Code    string
	Message string
	Details map[string]interface{}
}

// Error implements the error interface
func (e ExecutionError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Common execution errors
var (
	// ErrExecutionNotFound indicates the execution was not found
	ErrExecutionNotFound = ExecutionError{
		Code:    "EXEC_NOT_FOUND",
		Message: "Execution not found",
	}

	// ErrExecutionAlreadyExists indicates an execution already exists for the SBI
	ErrExecutionAlreadyExists = ExecutionError{
		Code:    "EXEC_ALREADY_EXISTS",
		Message: "An active execution already exists for this SBI",
	}

	// ErrInvalidTransition indicates an invalid state transition
	ErrInvalidTransition = ExecutionError{
		Code:    "EXEC_INVALID_TRANSITION",
		Message: "Invalid state transition",
	}

	// ErrExecutionCompleted indicates operation on completed execution
	ErrExecutionCompleted = ExecutionError{
		Code:    "EXEC_ALREADY_COMPLETED",
		Message: "Execution is already completed",
	}

	// ErrMaxAttemptsReached indicates maximum attempts have been reached
	ErrMaxAttemptsReached = ExecutionError{
		Code:    "EXEC_MAX_ATTEMPTS",
		Message: "Maximum implementation attempts reached",
	}

	// ErrInvalidDecision indicates an invalid decision value
	ErrInvalidDecision = ExecutionError{
		Code:    "EXEC_INVALID_DECISION",
		Message: "Invalid decision value",
	}

	// ErrExecutionStuck indicates the execution is stuck
	ErrExecutionStuck = ExecutionError{
		Code:    "EXEC_STUCK",
		Message: "Execution is stuck and requires intervention",
	}
)

// NewExecutionError creates a new execution error with details
func NewExecutionError(code, message string, details map[string]interface{}) ExecutionError {
	return ExecutionError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// WithDetails adds details to an existing error
func (e ExecutionError) WithDetails(details map[string]interface{}) ExecutionError {
	e.Details = details
	return e
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	execErr, ok := err.(ExecutionError)
	return ok && execErr.Code == ErrExecutionNotFound.Code
}

// IsAlreadyExists checks if the error is an already exists error
func IsAlreadyExists(err error) bool {
	execErr, ok := err.(ExecutionError)
	return ok && execErr.Code == ErrExecutionAlreadyExists.Code
}

// IsCompleted checks if the error is a completed execution error
func IsCompleted(err error) bool {
	execErr, ok := err.(ExecutionError)
	return ok && execErr.Code == ErrExecutionCompleted.Code
}

// IsMaxAttempts checks if the error is a max attempts error
func IsMaxAttempts(err error) bool {
	execErr, ok := err.(ExecutionError)
	return ok && execErr.Code == ErrMaxAttemptsReached.Code
}

// IsInvalidTransition checks if the error is an invalid transition error
func IsInvalidTransition(err error) bool {
	execErr, ok := err.(ExecutionError)
	return ok && execErr.Code == ErrInvalidTransition.Code
}

// IsInvalidDecision checks if the error is an invalid decision error
func IsInvalidDecision(err error) bool {
	execErr, ok := err.(ExecutionError)
	return ok && execErr.Code == ErrInvalidDecision.Code
}

// IsExecutionStuck checks if the error is an execution stuck error
func IsExecutionStuck(err error) bool {
	execErr, ok := err.(ExecutionError)
	return ok && execErr.Code == ErrExecutionStuck.Code
}
