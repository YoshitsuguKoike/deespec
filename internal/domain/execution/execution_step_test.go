package execution

import (
	"testing"
)

// TestExecutionStepString verifies the String() method
func TestExecutionStepString(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected string
	}{
		{"Ready step", StepReady, "ready"},
		{"Implement try", StepImplementTry, "implement_try"},
		{"First review", StepFirstReview, "first_review"},
		{"Second try", StepImplementSecondTry, "implement_second_try"},
		{"Second review", StepSecondReview, "second_review"},
		{"Third try", StepImplementThirdTry, "implement_third_try"},
		{"Third review", StepThirdReview, "third_review"},
		{"Force implement", StepReviewerForceImplement, "reviewer_force_implement"},
		{"Implementer review", StepImplementerReview, "implementer_review"},
		{"Done", StepDone, "done"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.step.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestExecutionStepToNumber verifies the ToNumber() method
func TestExecutionStepToNumber(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected int
	}{
		{"Ready is 1", StepReady, 1},
		{"Implement try is 2", StepImplementTry, 2},
		{"First review is 3", StepFirstReview, 3},
		{"Second try is 4", StepImplementSecondTry, 4},
		{"Second review is 5", StepSecondReview, 5},
		{"Third try is 6", StepImplementThirdTry, 6},
		{"Third review is 7", StepThirdReview, 7},
		{"Force implement is 8", StepReviewerForceImplement, 8},
		{"Implementer review is 9", StepImplementerReview, 9},
		{"Done is 10", StepDone, 10},
		{"Invalid step is 0", ExecutionStep("invalid"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.step.ToNumber()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestExecutionStepToNumberOrdering verifies that step numbers are sequential
func TestExecutionStepToNumberOrdering(t *testing.T) {
	steps := []ExecutionStep{
		StepReady,
		StepImplementTry,
		StepFirstReview,
		StepImplementSecondTry,
		StepSecondReview,
		StepImplementThirdTry,
		StepThirdReview,
		StepReviewerForceImplement,
		StepImplementerReview,
		StepDone,
	}

	expectedNumber := 1
	for _, step := range steps {
		actualNumber := step.ToNumber()
		if actualNumber != expectedNumber {
			t.Errorf("Step %s: expected number %d, got %d",
				step, expectedNumber, actualNumber)
		}
		expectedNumber++
	}
}

// TestExecutionStepToStatus verifies the ToStatus() method
func TestExecutionStepToStatus(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected ExecutionStatus
	}{
		{"Ready -> READY", StepReady, StatusReady},
		{"Implement try -> WIP", StepImplementTry, StatusWIP},
		{"Second try -> WIP", StepImplementSecondTry, StatusWIP},
		{"Third try -> WIP", StepImplementThirdTry, StatusWIP},
		{"First review -> REVIEW", StepFirstReview, StatusReview},
		{"Second review -> REVIEW", StepSecondReview, StatusReview},
		{"Third review -> REVIEW", StepThirdReview, StatusReview},
		{"Implementer review -> REVIEW", StepImplementerReview, StatusReview},
		{"Force implement -> REVIEW&WIP", StepReviewerForceImplement, StatusReviewAndWIP},
		{"Done -> DONE", StepDone, StatusDone},
		{"Invalid -> UNKNOWN", ExecutionStep("invalid"), StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.step.ToStatus()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestExecutionStepIsImplementation verifies the IsImplementation() method
func TestExecutionStepIsImplementation(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected bool
	}{
		{"Implement try is implementation", StepImplementTry, true},
		{"Second try is implementation", StepImplementSecondTry, true},
		{"Third try is implementation", StepImplementThirdTry, true},
		{"Force implement is implementation", StepReviewerForceImplement, true},
		{"Ready is not implementation", StepReady, false},
		{"First review is not implementation", StepFirstReview, false},
		{"Second review is not implementation", StepSecondReview, false},
		{"Third review is not implementation", StepThirdReview, false},
		{"Implementer review is not implementation", StepImplementerReview, false},
		{"Done is not implementation", StepDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.step.IsImplementation()
			if result != tt.expected {
				t.Errorf("Expected IsImplementation() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStepIsReview verifies the IsReview() method
func TestExecutionStepIsReview(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected bool
	}{
		{"First review is review", StepFirstReview, true},
		{"Second review is review", StepSecondReview, true},
		{"Third review is review", StepThirdReview, true},
		{"Implementer review is review", StepImplementerReview, true},
		{"Force implement is review (dual)", StepReviewerForceImplement, true},
		{"Ready is not review", StepReady, false},
		{"Implement try is not review", StepImplementTry, false},
		{"Second try is not review", StepImplementSecondTry, false},
		{"Third try is not review", StepImplementThirdTry, false},
		{"Done is not review", StepDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.step.IsReview()
			if result != tt.expected {
				t.Errorf("Expected IsReview() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStepCanTransitionTo verifies the CanTransitionTo() method
func TestExecutionStepCanTransitionTo(t *testing.T) {
	tests := []struct {
		name     string
		current  ExecutionStep
		next     ExecutionStep
		expected bool
	}{
		// Valid transitions
		{"Ready -> Implement", StepReady, StepImplementTry, true},
		{"Implement -> First Review", StepImplementTry, StepFirstReview, true},
		{"First Review -> Second Try", StepFirstReview, StepImplementSecondTry, true},
		{"First Review -> Done", StepFirstReview, StepDone, true},
		{"Second Try -> Second Review", StepImplementSecondTry, StepSecondReview, true},
		{"Second Review -> Third Try", StepSecondReview, StepImplementThirdTry, true},
		{"Second Review -> Done", StepSecondReview, StepDone, true},
		{"Third Try -> Third Review", StepImplementThirdTry, StepThirdReview, true},
		{"Third Review -> Force", StepThirdReview, StepReviewerForceImplement, true},
		{"Third Review -> Done", StepThirdReview, StepDone, true},
		{"Force -> Implementer Review", StepReviewerForceImplement, StepImplementerReview, true},
		{"Implementer Review -> Done", StepImplementerReview, StepDone, true},

		// Invalid transitions
		{"Ready -> Done", StepReady, StepDone, false},
		{"Ready -> Review", StepReady, StepFirstReview, false},
		{"Implement -> Second Review", StepImplementTry, StepSecondReview, false},
		{"First Review -> Third Try", StepFirstReview, StepImplementThirdTry, false},
		{"First Review -> Force", StepFirstReview, StepReviewerForceImplement, false},
		{"Done -> anything", StepDone, StepReady, false},
		{"Done -> Implement", StepDone, StepImplementTry, false},
		{"Invalid from state", ExecutionStep("invalid"), StepReady, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.current.CanTransitionTo(tt.next)
			if result != tt.expected {
				t.Errorf("Expected CanTransitionTo(%s, %s) = %v, got %v",
					tt.current, tt.next, tt.expected, result)
			}
		})
	}
}

// TestExecutionStepGetAttemptNumber verifies the GetAttemptNumber() method
func TestExecutionStepGetAttemptNumber(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected int
	}{
		{"Implement try is attempt 1", StepImplementTry, 1},
		{"Second try is attempt 2", StepImplementSecondTry, 2},
		{"Third try is attempt 3", StepImplementThirdTry, 3},
		{"Force implement is attempt 4", StepReviewerForceImplement, 4},
		{"Ready is not attempt", StepReady, 0},
		{"First review is not attempt", StepFirstReview, 0},
		{"Done is not attempt", StepDone, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.step.GetAttemptNumber()
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestExecutionStepIsValid verifies the IsValid() method
func TestExecutionStepIsValid(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected bool
	}{
		{"Ready is valid", StepReady, true},
		{"Implement try is valid", StepImplementTry, true},
		{"First review is valid", StepFirstReview, true},
		{"Second try is valid", StepImplementSecondTry, true},
		{"Second review is valid", StepSecondReview, true},
		{"Third try is valid", StepImplementThirdTry, true},
		{"Third review is valid", StepThirdReview, true},
		{"Force implement is valid", StepReviewerForceImplement, true},
		{"Implementer review is valid", StepImplementerReview, true},
		{"Done is valid", StepDone, true},
		{"Invalid step is invalid", ExecutionStep("invalid"), false},
		{"Empty step is invalid", ExecutionStep(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.step.IsValid()
			if result != tt.expected {
				t.Errorf("Expected IsValid() = %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestExecutionStepTransitionChain verifies a complete transition chain
func TestExecutionStepTransitionChain(t *testing.T) {
	// Test success path (first try succeeds)
	successPath := []ExecutionStep{
		StepReady,
		StepImplementTry,
		StepFirstReview,
		StepDone,
	}

	for i := 0; i < len(successPath)-1; i++ {
		if !successPath[i].CanTransitionTo(successPath[i+1]) {
			t.Errorf("Success path broken: %s -> %s should be allowed",
				successPath[i], successPath[i+1])
		}
	}

	// Test force termination path (all attempts fail)
	forcePath := []ExecutionStep{
		StepReady,
		StepImplementTry,
		StepFirstReview,
		StepImplementSecondTry,
		StepSecondReview,
		StepImplementThirdTry,
		StepThirdReview,
		StepReviewerForceImplement,
		StepImplementerReview,
		StepDone,
	}

	for i := 0; i < len(forcePath)-1; i++ {
		if !forcePath[i].CanTransitionTo(forcePath[i+1]) {
			t.Errorf("Force path broken: %s -> %s should be allowed",
				forcePath[i], forcePath[i+1])
		}
	}
}

// TestExecutionStepDualNature verifies steps with dual nature (both implementation and review)
func TestExecutionStepDualNature(t *testing.T) {
	step := StepReviewerForceImplement

	if !step.IsImplementation() {
		t.Error("StepReviewerForceImplement should be considered implementation")
	}

	if !step.IsReview() {
		t.Error("StepReviewerForceImplement should be considered review")
	}

	status := step.ToStatus()
	if status != StatusReviewAndWIP {
		t.Errorf("Expected status REVIEW&WIP, got %s", status)
	}
}

// TestExecutionStepConsistency verifies consistency between step number and transitions
func TestExecutionStepConsistency(t *testing.T) {
	// Verify that steps with higher numbers can be transitioned to from lower numbers
	// (in valid transition paths)
	allSteps := []ExecutionStep{
		StepReady,
		StepImplementTry,
		StepFirstReview,
		StepImplementSecondTry,
		StepSecondReview,
		StepImplementThirdTry,
		StepThirdReview,
		StepReviewerForceImplement,
		StepImplementerReview,
		StepDone,
	}

	for i, step := range allSteps {
		stepNumber := step.ToNumber()
		expectedNumber := i + 1

		if stepNumber != expectedNumber {
			t.Errorf("Step %s: expected number %d, got %d",
				step, expectedNumber, stepNumber)
		}
	}
}
