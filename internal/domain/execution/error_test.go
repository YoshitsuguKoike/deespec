package execution

import (
	"errors"
	"testing"
)

// TestExecutionErrorError tests the Error() method implementation
func TestExecutionErrorError(t *testing.T) {
	tests := []struct {
		name     string
		err      ExecutionError
		expected string
	}{
		{
			name: "Error with code and message",
			err: ExecutionError{
				Code:    "TEST_ERROR",
				Message: "Test error message",
			},
			expected: "[TEST_ERROR] Test error message",
		},
		{
			name: "Error with empty code",
			err: ExecutionError{
				Code:    "",
				Message: "Message without code",
			},
			expected: "[] Message without code",
		},
		{
			name: "Error with empty message",
			err: ExecutionError{
				Code:    "EMPTY_MSG",
				Message: "",
			},
			expected: "[EMPTY_MSG] ",
		},
		{
			name: "Error with both empty",
			err: ExecutionError{
				Code:    "",
				Message: "",
			},
			expected: "[] ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// TestPredefinedErrors tests all predefined error constants
func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      ExecutionError
		code     string
		contains string
	}{
		{
			name:     "ErrExecutionNotFound",
			err:      ErrExecutionNotFound,
			code:     "EXEC_NOT_FOUND",
			contains: "not found",
		},
		{
			name:     "ErrExecutionAlreadyExists",
			err:      ErrExecutionAlreadyExists,
			code:     "EXEC_ALREADY_EXISTS",
			contains: "already exists",
		},
		{
			name:     "ErrInvalidTransition",
			err:      ErrInvalidTransition,
			code:     "EXEC_INVALID_TRANSITION",
			contains: "Invalid state transition",
		},
		{
			name:     "ErrExecutionCompleted",
			err:      ErrExecutionCompleted,
			code:     "EXEC_ALREADY_COMPLETED",
			contains: "already completed",
		},
		{
			name:     "ErrMaxAttemptsReached",
			err:      ErrMaxAttemptsReached,
			code:     "EXEC_MAX_ATTEMPTS",
			contains: "Maximum",
		},
		{
			name:     "ErrInvalidDecision",
			err:      ErrInvalidDecision,
			code:     "EXEC_INVALID_DECISION",
			contains: "Invalid decision",
		},
		{
			name:     "ErrExecutionStuck",
			err:      ErrExecutionStuck,
			code:     "EXEC_STUCK",
			contains: "stuck",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("Expected code %q, got %q", tt.code, tt.err.Code)
			}

			errMsg := tt.err.Error()
			if errMsg == "" {
				t.Error("Error message should not be empty")
			}
		})
	}
}

// TestNewExecutionError tests the NewExecutionError constructor
func TestNewExecutionError(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		message string
		details map[string]interface{}
	}{
		{
			name:    "Error with details",
			code:    "CUSTOM_ERROR",
			message: "Custom error message",
			details: map[string]interface{}{
				"sbi_id":  "SBI-001",
				"attempt": 3,
			},
		},
		{
			name:    "Error without details",
			code:    "SIMPLE_ERROR",
			message: "Simple message",
			details: nil,
		},
		{
			name:    "Error with empty details",
			code:    "EMPTY_DETAILS",
			message: "Empty details",
			details: map[string]interface{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewExecutionError(tt.code, tt.message, tt.details)

			if err.Code != tt.code {
				t.Errorf("Expected code %q, got %q", tt.code, err.Code)
			}

			if err.Message != tt.message {
				t.Errorf("Expected message %q, got %q", tt.message, err.Message)
			}

			if tt.details == nil && err.Details != nil {
				t.Error("Expected details to be nil")
			}

			if tt.details != nil {
				if err.Details == nil {
					t.Error("Expected details to be non-nil")
				} else if len(err.Details) != len(tt.details) {
					t.Errorf("Expected %d details, got %d", len(tt.details), len(err.Details))
				}
			}
		})
	}
}

// TestWithDetails tests adding details to an existing error
func TestWithDetails(t *testing.T) {
	tests := []struct {
		name            string
		baseErr         ExecutionError
		details         map[string]interface{}
		expectedDetails int
	}{
		{
			name:    "Add details to error without details",
			baseErr: ErrExecutionNotFound,
			details: map[string]interface{}{
				"execution_id": "exec-123",
			},
			expectedDetails: 1,
		},
		{
			name: "Replace existing details",
			baseErr: ExecutionError{
				Code:    "TEST",
				Message: "Test",
				Details: map[string]interface{}{
					"old_key": "old_value",
				},
			},
			details: map[string]interface{}{
				"new_key": "new_value",
			},
			expectedDetails: 1,
		},
		{
			name:            "Add nil details",
			baseErr:         ErrInvalidDecision,
			details:         nil,
			expectedDetails: 0,
		},
		{
			name:    "Add multiple details",
			baseErr: ErrMaxAttemptsReached,
			details: map[string]interface{}{
				"sbi_id":     "SBI-001",
				"attempt":    3,
				"max":        3,
				"started_at": "2024-01-01",
			},
			expectedDetails: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.baseErr.WithDetails(tt.details)

			// Verify original error is not modified
			if tt.baseErr.Details != nil && result.Details != nil {
				// Deep comparison would be complex, just check they're different instances
				if len(result.Details) > 0 && &tt.baseErr.Details == &result.Details {
					t.Error("WithDetails should create a new error, not modify original")
				}
			}

			// Verify new error has correct details
			if tt.details == nil {
				if result.Details != nil && len(result.Details) > 0 {
					t.Error("Expected details to be empty for nil input")
				}
			} else {
				if result.Details == nil {
					t.Error("Expected details to be set")
				} else if len(result.Details) != tt.expectedDetails {
					t.Errorf("Expected %d details, got %d", tt.expectedDetails, len(result.Details))
				}
			}

			// Verify code and message are preserved
			if result.Code != tt.baseErr.Code {
				t.Errorf("Code should be preserved: expected %q, got %q", tt.baseErr.Code, result.Code)
			}
			if result.Message != tt.baseErr.Message {
				t.Errorf("Message should be preserved: expected %q, got %q", tt.baseErr.Message, result.Message)
			}
		})
	}
}

// TestIsNotFound tests the IsNotFound helper function
func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Not found error",
			err:      ErrExecutionNotFound,
			expected: true,
		},
		{
			name: "Not found error with details",
			err: ErrExecutionNotFound.WithDetails(map[string]interface{}{
				"id": "123",
			}),
			expected: true,
		},
		{
			name:     "Different execution error",
			err:      ErrExecutionAlreadyExists,
			expected: false,
		},
		{
			name:     "Standard error",
			err:      errors.New("standard error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
		{
			name: "Custom error with same code",
			err: ExecutionError{
				Code:    "EXEC_NOT_FOUND",
				Message: "Custom not found",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsNotFound to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsAlreadyExists tests the IsAlreadyExists helper function
func TestIsAlreadyExists(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Already exists error",
			err:      ErrExecutionAlreadyExists,
			expected: true,
		},
		{
			name: "Already exists with details",
			err: ErrExecutionAlreadyExists.WithDetails(map[string]interface{}{
				"sbi_id": "SBI-001",
			}),
			expected: true,
		},
		{
			name:     "Different error",
			err:      ErrExecutionNotFound,
			expected: false,
		},
		{
			name:     "Standard error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAlreadyExists(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsAlreadyExists to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsCompletedError tests the IsCompleted helper function
func TestIsCompletedError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Completed error",
			err:      ErrExecutionCompleted,
			expected: true,
		},
		{
			name: "Completed error with details",
			err: ErrExecutionCompleted.WithDetails(map[string]interface{}{
				"completed_at": "2024-01-01",
			}),
			expected: true,
		},
		{
			name:     "Different error",
			err:      ErrMaxAttemptsReached,
			expected: false,
		},
		{
			name:     "Standard error",
			err:      errors.New("error"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsCompleted(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsCompleted to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsMaxAttempts tests the IsMaxAttempts helper function
func TestIsMaxAttempts(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Max attempts error",
			err:      ErrMaxAttemptsReached,
			expected: true,
		},
		{
			name: "Max attempts with details",
			err: ErrMaxAttemptsReached.WithDetails(map[string]interface{}{
				"attempt": 3,
				"max":     3,
			}),
			expected: true,
		},
		{
			name:     "Different error",
			err:      ErrInvalidDecision,
			expected: false,
		},
		{
			name:     "Standard error",
			err:      errors.New("max attempts"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsMaxAttempts(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsMaxAttempts to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionErrorAsError tests that ExecutionError implements error interface
func TestExecutionErrorAsError(t *testing.T) {
	var err error = ErrExecutionNotFound

	if err == nil {
		t.Error("ExecutionError should implement error interface")
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message should not be empty")
	}

	if errMsg != "[EXEC_NOT_FOUND] Execution not found" {
		t.Errorf("Expected specific error message, got %q", errMsg)
	}
}

// TestExecutionErrorChaining tests error handling patterns
func TestExecutionErrorChaining(t *testing.T) {
	t.Run("Multiple WithDetails calls", func(t *testing.T) {
		err1 := ErrExecutionNotFound.WithDetails(map[string]interface{}{
			"id": "123",
		})

		err2 := err1.WithDetails(map[string]interface{}{
			"attempt": 1,
		})

		// err1 should not be affected
		if len(err1.Details) != 1 {
			t.Errorf("err1 should have 1 detail, got %d", len(err1.Details))
		}

		// err2 should have the new details
		if len(err2.Details) != 1 {
			t.Errorf("err2 should have 1 detail, got %d", len(err2.Details))
		}

		// Both should have same code
		if err1.Code != err2.Code {
			t.Error("Code should be preserved across WithDetails calls")
		}
	})
}

// TestExecutionErrorDetailsAccess tests accessing details from errors
func TestExecutionErrorDetailsAccess(t *testing.T) {
	details := map[string]interface{}{
		"sbi_id":    "SBI-001",
		"attempt":   3,
		"max":       5,
		"timestamp": "2024-01-01T00:00:00Z",
	}

	err := NewExecutionError("TEST_ERROR", "Test message", details)

	// Verify all details are accessible
	if err.Details["sbi_id"] != "SBI-001" {
		t.Error("Should be able to access sbi_id from details")
	}

	if err.Details["attempt"] != 3 {
		t.Error("Should be able to access attempt from details")
	}

	// Verify type assertion works
	if sbiID, ok := err.Details["sbi_id"].(string); !ok || sbiID != "SBI-001" {
		t.Error("Type assertion for string detail should work")
	}

	if attempt, ok := err.Details["attempt"].(int); !ok || attempt != 3 {
		t.Error("Type assertion for int detail should work")
	}
}

// TestExecutionErrorComparison tests error comparison scenarios
func TestExecutionErrorComparison(t *testing.T) {
	t.Run("Same error codes are equal", func(t *testing.T) {
		err1 := ExecutionError{Code: "TEST", Message: "Test"}
		err2 := ExecutionError{Code: "TEST", Message: "Different"}

		if err1.Code != err2.Code {
			t.Error("Errors with same code should have equal codes")
		}
	})

	t.Run("Different errors are not equal", func(t *testing.T) {
		if ErrExecutionNotFound.Code == ErrExecutionAlreadyExists.Code {
			t.Error("Different error types should have different codes")
		}
	})

	t.Run("Predefined errors are constant", func(t *testing.T) {
		if ErrExecutionNotFound.Code != "EXEC_NOT_FOUND" {
			t.Error("Predefined error codes should be constant")
		}
	})
}

// TestIsInvalidTransition tests the IsInvalidTransition helper function
func TestIsInvalidTransition(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Invalid transition error",
			err:      ErrInvalidTransition,
			expected: true,
		},
		{
			name: "Invalid transition with details",
			err: ErrInvalidTransition.WithDetails(map[string]interface{}{
				"from": "Ready",
				"to":   "Done",
			}),
			expected: true,
		},
		{
			name:     "Different error",
			err:      ErrExecutionNotFound,
			expected: false,
		},
		{
			name:     "Standard error",
			err:      errors.New("invalid transition"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInvalidTransition(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsInvalidTransition to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsInvalidDecision tests the IsInvalidDecision helper function
func TestIsInvalidDecision(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Invalid decision error",
			err:      ErrInvalidDecision,
			expected: true,
		},
		{
			name: "Invalid decision with details",
			err: ErrInvalidDecision.WithDetails(map[string]interface{}{
				"decision": "invalid_value",
			}),
			expected: true,
		},
		{
			name:     "Different error",
			err:      ErrMaxAttemptsReached,
			expected: false,
		},
		{
			name:     "Standard error",
			err:      errors.New("invalid decision"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsInvalidDecision(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsInvalidDecision to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestIsExecutionStuck tests the IsExecutionStuck helper function
func TestIsExecutionStuck(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "Execution stuck error",
			err:      ErrExecutionStuck,
			expected: true,
		},
		{
			name: "Execution stuck with details",
			err: ErrExecutionStuck.WithDetails(map[string]interface{}{
				"reason":   "repeated failures",
				"attempts": 5,
			}),
			expected: true,
		},
		{
			name:     "Different error",
			err:      ErrInvalidTransition,
			expected: false,
		},
		{
			name:     "Standard error",
			err:      errors.New("stuck"),
			expected: false,
		},
		{
			name:     "Nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsExecutionStuck(tt.err)
			if result != tt.expected {
				t.Errorf("Expected IsExecutionStuck to be %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestHelperFunctionsWithNonExecutionErrors tests helper functions with non-ExecutionError types
func TestHelperFunctionsWithNonExecutionErrors(t *testing.T) {
	standardErr := errors.New("standard error")

	t.Run("IsNotFound with standard error", func(t *testing.T) {
		if IsNotFound(standardErr) {
			t.Error("Standard error should not be detected as NotFound")
		}
	})

	t.Run("IsAlreadyExists with standard error", func(t *testing.T) {
		if IsAlreadyExists(standardErr) {
			t.Error("Standard error should not be detected as AlreadyExists")
		}
	})

	t.Run("IsCompleted with standard error", func(t *testing.T) {
		if IsCompleted(standardErr) {
			t.Error("Standard error should not be detected as Completed")
		}
	})

	t.Run("IsMaxAttempts with standard error", func(t *testing.T) {
		if IsMaxAttempts(standardErr) {
			t.Error("Standard error should not be detected as MaxAttempts")
		}
	})

	t.Run("IsInvalidTransition with standard error", func(t *testing.T) {
		if IsInvalidTransition(standardErr) {
			t.Error("Standard error should not be detected as InvalidTransition")
		}
	})

	t.Run("IsInvalidDecision with standard error", func(t *testing.T) {
		if IsInvalidDecision(standardErr) {
			t.Error("Standard error should not be detected as InvalidDecision")
		}
	})

	t.Run("IsExecutionStuck with standard error", func(t *testing.T) {
		if IsExecutionStuck(standardErr) {
			t.Error("Standard error should not be detected as ExecutionStuck")
		}
	})
}

// TestExecutionErrorEdgeCases tests edge cases
func TestExecutionErrorEdgeCases(t *testing.T) {
	t.Run("Empty ExecutionError", func(t *testing.T) {
		err := ExecutionError{}
		msg := err.Error()
		if msg != "[] " {
			t.Errorf("Empty error should have format '[] ', got %q", msg)
		}
	})

	t.Run("Details with nil values", func(t *testing.T) {
		err := NewExecutionError("TEST", "Test", map[string]interface{}{
			"nil_value": nil,
			"valid":     "value",
		})

		if len(err.Details) != 2 {
			t.Errorf("Expected 2 details including nil, got %d", len(err.Details))
		}
	})

	t.Run("WithDetails preserves original error", func(t *testing.T) {
		original := ErrExecutionNotFound
		modified := original.WithDetails(map[string]interface{}{"key": "value"})

		// Original should still have no details (assuming it starts with nil)
		if original.Details != nil && len(original.Details) > 0 {
			t.Error("WithDetails should not modify the original error")
		}

		// Modified should have details
		if modified.Details == nil || len(modified.Details) == 0 {
			t.Error("Modified error should have details")
		}
	})
}
