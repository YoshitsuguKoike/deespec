package execution

import (
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
