package execution

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewSBIExecution(t *testing.T) {
	sbiID := "SBI-TEST-001"
	exec := NewSBIExecution(sbiID)

	if exec.SBIID != sbiID {
		t.Errorf("Expected SBIID %s, got %s", sbiID, exec.SBIID)
	}

	if exec.Step != StepReady {
		t.Errorf("Expected initial step to be %s, got %s", StepReady, exec.Step)
	}

	if exec.Status != StatusReady {
		t.Errorf("Expected initial status to be %s, got %s", StatusReady, exec.Status)
	}

	if exec.Decision != DecisionPending {
		t.Errorf("Expected initial decision to be %s, got %s", DecisionPending, exec.Decision)
	}

	if exec.Attempt != 0 {
		t.Errorf("Expected initial attempt to be 0, got %d", exec.Attempt)
	}
}

func TestExecutionTransitions(t *testing.T) {
	tests := []struct {
		name          string
		currentStep   ExecutionStep
		nextStep      ExecutionStep
		shouldSucceed bool
	}{
		{"Ready to Implement", StepReady, StepImplementTry, true},
		{"Implement to Review", StepImplementTry, StepFirstReview, true},
		{"Review to Implement (retry)", StepFirstReview, StepImplementSecondTry, true},
		{"Review to Done (success)", StepFirstReview, StepDone, true},
		{"Invalid: Ready to Done", StepReady, StepDone, false},
		{"Invalid: Implement to Implement", StepImplementTry, StepImplementSecondTry, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &SBIExecution{
				Step:   tt.currentStep,
				Status: tt.currentStep.ToStatus(),
			}

			err := exec.TransitionTo(tt.nextStep)
			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected transition to succeed, got error: %v", err)
			}
			if !tt.shouldSucceed && err == nil {
				t.Errorf("Expected transition to fail, but it succeeded")
			}

			if tt.shouldSucceed && exec.Step != tt.nextStep {
				t.Errorf("Expected step to be %s, got %s", tt.nextStep, exec.Step)
			}
		})
	}
}

func TestApplyDecision(t *testing.T) {
	tests := []struct {
		name          string
		status        ExecutionStatus
		decision      Decision
		shouldSucceed bool
	}{
		{"Apply Succeeded in Review", StatusReview, DecisionSucceeded, true},
		{"Apply NeedsChanges in Review", StatusReview, DecisionNeedsChanges, true},
		{"Cannot apply in WIP", StatusWIP, DecisionSucceeded, false},
		{"Cannot apply in Ready", StatusReady, DecisionSucceeded, false},
		{"Invalid decision", StatusReview, Decision("INVALID"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &SBIExecution{
				Status: tt.status,
			}

			err := exec.ApplyDecision(tt.decision)
			if tt.shouldSucceed && err != nil {
				t.Errorf("Expected decision to be applied, got error: %v", err)
			}
			if !tt.shouldSucceed && err == nil {
				t.Errorf("Expected decision application to fail, but it succeeded")
			}

			if tt.shouldSucceed && exec.Decision != tt.decision {
				t.Errorf("Expected decision to be %s, got %s", tt.decision, exec.Decision)
			}
		})
	}
}

func TestNextStepLogic(t *testing.T) {
	tests := []struct {
		name         string
		currentStep  ExecutionStep
		decision     Decision
		expectedNext ExecutionStep
	}{
		{"Ready -> Implement", StepReady, DecisionPending, StepImplementTry},
		{"First Review Success -> Done", StepFirstReview, DecisionSucceeded, StepDone},
		{"First Review Fail -> Second Try", StepFirstReview, DecisionNeedsChanges, StepImplementSecondTry},
		{"Second Review Success -> Done", StepSecondReview, DecisionSucceeded, StepDone},
		{"Second Review Fail -> Third Try", StepSecondReview, DecisionNeedsChanges, StepImplementThirdTry},
		{"Third Review Success -> Done", StepThirdReview, DecisionSucceeded, StepDone},
		{"Third Review Fail -> Force", StepThirdReview, DecisionNeedsChanges, StepReviewerForceImplement},
		{"Force -> Implementer Review", StepReviewerForceImplement, DecisionPending, StepImplementerReview},
		{"Implementer Review -> Done", StepImplementerReview, DecisionPending, StepDone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &SBIExecution{
				Step:     tt.currentStep,
				Decision: tt.decision,
			}

			next, err := exec.NextStep()
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if next != tt.expectedNext {
				t.Errorf("Expected next step to be %s, got %s", tt.expectedNext, next)
			}
		})
	}
}

func TestShouldForceTerminate(t *testing.T) {
	tests := []struct {
		name     string
		attempt  int
		decision Decision
		expected bool
	}{
		{"Not enough attempts", 2, DecisionNeedsChanges, false},
		{"Three attempts with failure", 3, DecisionNeedsChanges, true},
		{"Three attempts but success", 3, DecisionSucceeded, false},
		{"Four attempts", 4, DecisionNeedsChanges, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &SBIExecution{
				Attempt:  tt.attempt,
				Decision: tt.decision,
			}

			if exec.ShouldForceTerminate() != tt.expected {
				t.Errorf("Expected force terminate to be %v, got %v", tt.expected, exec.ShouldForceTerminate())
			}
		})
	}
}

func TestGetFinalDecision(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		decision Decision
		attempt  int
		expected Decision
	}{
		{"Not completed", StepImplementTry, DecisionPending, 1, DecisionPending},
		{"Completed with Success", StepDone, DecisionSucceeded, 1, DecisionSucceeded},
		{"Completed with Failure", StepDone, DecisionFailed, 3, DecisionFailed},
		{"Completed after force termination", StepDone, DecisionFailed, 4, DecisionFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			exec := &SBIExecution{
				Step:     tt.step,
				Decision: tt.decision,
				Attempt:  tt.attempt,
			}
			if tt.step == StepDone {
				exec.CompletedAt = &now
			}

			final := exec.GetFinalDecision()
			if final != tt.expected {
				t.Errorf("Expected final decision to be %s, got %s", tt.expected, final)
			}
		})
	}
}

func TestIsCompleted(t *testing.T) {
	tests := []struct {
		name     string
		step     ExecutionStep
		expected bool
	}{
		{"Not completed - Ready", StepReady, false},
		{"Not completed - Implementation", StepImplementTry, false},
		{"Not completed - Review", StepFirstReview, false},
		{"Completed - Done", StepDone, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now()
			exec := &SBIExecution{
				Step: tt.step,
			}
			if tt.step == StepDone {
				exec.CompletedAt = &now
			}

			if exec.IsCompleted() != tt.expected {
				t.Errorf("IsCompleted() = %v, want %v", exec.IsCompleted(), tt.expected)
			}
		})
	}
}

func TestAttemptIncrement(t *testing.T) {
	exec := &SBIExecution{
		Attempt: 0,
	}

	// Manually increment attempt field
	exec.Attempt++
	if exec.Attempt != 1 {
		t.Errorf("Expected attempt 1, got %d", exec.Attempt)
	}

	exec.Attempt++
	if exec.Attempt != 2 {
		t.Errorf("Expected attempt 2, got %d", exec.Attempt)
	}
}

func TestTimingFields(t *testing.T) {
	exec := &SBIExecution{}

	// Test setting start time
	startTime := time.Now()
	exec.StartedAt = startTime

	if !exec.StartedAt.Equal(startTime) {
		t.Error("Start time not set correctly")
	}

	// Test setting end time
	time.Sleep(50 * time.Millisecond)
	endTime := time.Now()
	exec.CompletedAt = &endTime

	if exec.CompletedAt == nil || !exec.CompletedAt.Equal(endTime) {
		t.Error("End time not set correctly")
	}

	duration := exec.CompletedAt.Sub(exec.StartedAt)
	if duration < 50*time.Millisecond {
		t.Errorf("Duration calculation incorrect: %v", duration)
	}
}

// TestNewExecutionID verifies ExecutionID creation with proper formatting
func TestNewExecutionID(t *testing.T) {
	tests := []struct {
		name  string
		sbiID string
	}{
		{"Standard SBI ID", "SBI-TEST-001"},
		{"UUID-like SBI ID", "15e8ba80-36fc-402e-afba-afa3ca94fc2e"},
		{"Short ID", "TEST"},
		{"ID with special chars", "SBI_TEST_001"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			startTime := time.Now()
			execID := NewExecutionID(tt.sbiID, startTime)

			// Verify format: sbiID_timestamp
			expected := fmt.Sprintf("%s_%d", tt.sbiID, startTime.Unix())
			if string(execID) != expected {
				t.Errorf("Expected ExecutionID %s, got %s", expected, execID)
			}

			// Verify ExecutionID is not empty
			if execID == "" {
				t.Error("ExecutionID should not be empty")
			}
		})
	}
}

// TestNewExecutionIDUniqueness verifies that different timestamps produce different IDs
func TestNewExecutionIDUniqueness(t *testing.T) {
	sbiID := "SBI-001"
	time1 := time.Now()
	time.Sleep(1 * time.Second)
	time2 := time.Now()

	id1 := NewExecutionID(sbiID, time1)
	id2 := NewExecutionID(sbiID, time2)

	if id1 == id2 {
		t.Errorf("Expected different ExecutionIDs, but got same: %s", id1)
	}
}

// TestTransitionToUpdatesTimestamp verifies that TransitionTo updates the UpdatedAt field
func TestTransitionToUpdatesTimestamp(t *testing.T) {
	exec := NewSBIExecution("SBI-001")
	originalUpdatedAt := exec.UpdatedAt

	// Wait a bit to ensure timestamp difference
	time.Sleep(10 * time.Millisecond)

	err := exec.TransitionTo(StepImplementTry)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !exec.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after transition")
	}
}

// TestTransitionToSetsCompletedAt verifies that transitioning to Done sets CompletedAt
func TestTransitionToSetsCompletedAt(t *testing.T) {
	exec := &SBIExecution{
		Step:   StepFirstReview,
		Status: StatusReview,
	}

	if exec.CompletedAt != nil {
		t.Error("CompletedAt should be nil initially")
	}

	err := exec.TransitionTo(StepDone)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if exec.CompletedAt == nil {
		t.Error("CompletedAt should be set when transitioning to Done")
	}

	if exec.CompletedAt.Before(exec.UpdatedAt) {
		t.Error("CompletedAt should not be before UpdatedAt")
	}
}

// TestTransitionToIncrementsAttempt verifies attempt counter incrementation
func TestTransitionToIncrementsAttempt(t *testing.T) {
	tests := []struct {
		name            string
		initialStep     ExecutionStep
		nextStep        ExecutionStep
		initialAttempt  int
		expectedAttempt int
	}{
		{"Ready to First Try", StepReady, StepImplementTry, 0, 1},
		{"First Review to Second Try", StepFirstReview, StepImplementSecondTry, 1, 2},
		{"Second Review to Third Try", StepSecondReview, StepImplementThirdTry, 2, 3},
		{"Review to Done (no increment)", StepFirstReview, StepDone, 1, 1},
		{"First Try to Review (no increment)", StepImplementTry, StepFirstReview, 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &SBIExecution{
				Step:    tt.initialStep,
				Status:  tt.initialStep.ToStatus(),
				Attempt: tt.initialAttempt,
			}

			err := exec.TransitionTo(tt.nextStep)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if exec.Attempt != tt.expectedAttempt {
				t.Errorf("Expected attempt %d, got %d", tt.expectedAttempt, exec.Attempt)
			}
		})
	}
}

// TestApplyDecisionUpdatesTimestamp verifies that ApplyDecision updates UpdatedAt
func TestApplyDecisionUpdatesTimestamp(t *testing.T) {
	exec := &SBIExecution{
		Status:    StatusReview,
		UpdatedAt: time.Now(),
	}
	originalUpdatedAt := exec.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	err := exec.ApplyDecision(DecisionSucceeded)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if !exec.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after applying decision")
	}
}

// TestCompleteExecutionLifecycle tests a complete successful execution flow
func TestCompleteExecutionLifecycle(t *testing.T) {
	exec := NewSBIExecution("SBI-LIFECYCLE-001")

	// Verify initial state
	if exec.Step != StepReady || exec.Attempt != 0 {
		t.Fatalf("Invalid initial state: step=%s, attempt=%d", exec.Step, exec.Attempt)
	}

	// Ready -> Implement Try
	if err := exec.TransitionTo(StepImplementTry); err != nil {
		t.Fatalf("Failed to transition to ImplementTry: %v", err)
	}
	if exec.Attempt != 1 {
		t.Errorf("Expected attempt 1, got %d", exec.Attempt)
	}

	// Implement Try -> First Review
	if err := exec.TransitionTo(StepFirstReview); err != nil {
		t.Fatalf("Failed to transition to FirstReview: %v", err)
	}

	// Apply successful decision
	if err := exec.ApplyDecision(DecisionSucceeded); err != nil {
		t.Fatalf("Failed to apply decision: %v", err)
	}

	// First Review -> Done (success path)
	if err := exec.TransitionTo(StepDone); err != nil {
		t.Fatalf("Failed to transition to Done: %v", err)
	}

	// Verify final state
	if !exec.IsCompleted() {
		t.Error("Execution should be completed")
	}
	if exec.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	if exec.GetFinalDecision() != DecisionSucceeded {
		t.Errorf("Expected final decision SUCCEEDED, got %s", exec.GetFinalDecision())
	}
}

// TestFailureExecutionLifecycle tests execution with multiple retry attempts
func TestFailureExecutionLifecycle(t *testing.T) {
	exec := NewSBIExecution("SBI-FAIL-001")

	// Ready -> Implement Try -> First Review
	exec.TransitionTo(StepImplementTry)
	exec.TransitionTo(StepFirstReview)
	exec.ApplyDecision(DecisionNeedsChanges)

	// First Review -> Second Try -> Second Review
	exec.TransitionTo(StepImplementSecondTry)
	if exec.Attempt != 2 {
		t.Errorf("Expected attempt 2, got %d", exec.Attempt)
	}
	exec.TransitionTo(StepSecondReview)
	exec.ApplyDecision(DecisionNeedsChanges)

	// Second Review -> Third Try -> Third Review
	exec.TransitionTo(StepImplementThirdTry)
	if exec.Attempt != 3 {
		t.Errorf("Expected attempt 3, got %d", exec.Attempt)
	}
	exec.TransitionTo(StepThirdReview)
	exec.ApplyDecision(DecisionNeedsChanges)

	// Verify force termination condition
	if !exec.ShouldForceTerminate() {
		t.Error("Should force terminate after 3 failed attempts")
	}

	// Third Review -> Force Implement -> Implementer Review -> Done
	exec.TransitionTo(StepReviewerForceImplement)
	exec.TransitionTo(StepImplementerReview)
	exec.TransitionTo(StepDone)

	// Verify final state
	if !exec.IsCompleted() {
		t.Error("Execution should be completed")
	}
	if exec.GetFinalDecision() != DecisionFailed {
		t.Errorf("Expected final decision FAILED, got %s", exec.GetFinalDecision())
	}
}

// TestGetFinalDecisionEdgeCases tests edge cases for GetFinalDecision
func TestGetFinalDecisionEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		step        ExecutionStep
		decision    Decision
		attempt     int
		completedAt *time.Time
		expected    Decision
	}{
		{
			name:        "Not completed returns pending",
			step:        StepImplementTry,
			decision:    DecisionSucceeded,
			attempt:     1,
			completedAt: nil,
			expected:    DecisionPending,
		},
		{
			name:        "Completed with final decision",
			step:        StepDone,
			decision:    DecisionSucceeded,
			attempt:     1,
			completedAt: timePtr(time.Now()),
			expected:    DecisionSucceeded,
		},
		{
			name:        "Completed with force termination",
			step:        StepDone,
			decision:    DecisionNeedsChanges,
			attempt:     3,
			completedAt: timePtr(time.Now()),
			expected:    DecisionFailed,
		},
		{
			name:        "Completed with 4 attempts",
			step:        StepDone,
			decision:    DecisionFailed,
			attempt:     4,
			completedAt: timePtr(time.Now()),
			expected:    DecisionFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &SBIExecution{
				Step:        tt.step,
				Decision:    tt.decision,
				Attempt:     tt.attempt,
				CompletedAt: tt.completedAt,
			}

			result := exec.GetFinalDecision()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestNextStepFromDone verifies that Done state returns Done
func TestNextStepFromDone(t *testing.T) {
	exec := &SBIExecution{
		Step: StepDone,
	}

	next, err := exec.NextStep()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if next != StepDone {
		t.Errorf("Expected next step to be Done, got %s", next)
	}
}

// TestTransitionToInvalidStep verifies error handling for invalid transitions
func TestTransitionToInvalidStep(t *testing.T) {
	tests := []struct {
		name        string
		currentStep ExecutionStep
		nextStep    ExecutionStep
	}{
		{"Skip from Ready to Done", StepReady, StepDone},
		{"Skip from Implement to Third Review", StepImplementTry, StepThirdReview},
		{"Backward from Review to Implement", StepFirstReview, StepImplementTry},
		{"From Done to anything", StepDone, StepImplementTry},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &SBIExecution{
				Step:   tt.currentStep,
				Status: tt.currentStep.ToStatus(),
			}

			err := exec.TransitionTo(tt.nextStep)
			if err == nil {
				t.Error("Expected error for invalid transition, but got none")
			}
		})
	}
}

// timePtr is a helper function to create a time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}

// TestNextStepUnknownStep verifies error handling for unknown steps
func TestNextStepUnknownStep(t *testing.T) {
	exec := &SBIExecution{
		Step: ExecutionStep("UNKNOWN_STEP"),
	}

	_, err := exec.NextStep()
	if err == nil {
		t.Error("Expected error for unknown step, but got none")
	}

	expectedErrMsg := "unknown step"
	if err != nil && !strings.Contains(err.Error(), expectedErrMsg) {
		t.Errorf("Expected error message to contain %q, got: %s", expectedErrMsg, err.Error())
	}
}

// TestGetFinalDecisionNonFinalDecision verifies GetFinalDecision with non-final decisions
func TestGetFinalDecisionNonFinalDecision(t *testing.T) {
	now := time.Now()
	exec := &SBIExecution{
		Step:        StepDone,
		Decision:    DecisionNeedsChanges, // Non-final decision
		Attempt:     2,                    // Less than 3 attempts
		CompletedAt: &now,
	}

	result := exec.GetFinalDecision()
	// When completed with non-final decision but < 3 attempts, should return the decision
	if result != DecisionNeedsChanges {
		t.Errorf("Expected %s, got %s", DecisionNeedsChanges, result)
	}
}
