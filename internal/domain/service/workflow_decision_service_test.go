package service

import (
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

// Helper to create test SBI
func createTestSBI(status model.Status, onlyImplement bool, currentAttempt int) *sbi.SBI {
	metadata := sbi.SBIMetadata{
		OnlyImplement: onlyImplement,
	}

	attempt := model.NewAttempt()
	for i := 1; i < currentAttempt; i++ {
		attempt = attempt.Increment()
	}

	execution := &sbi.ExecutionState{
		CurrentTurn:    model.NewTurn(),
		CurrentAttempt: attempt,
		MaxTurns:       10,
		MaxAttempts:    3,
	}

	taskID := model.NewTaskID()
	return sbi.ReconstructSBI(
		taskID,
		"Test SBI",
		"Test description",
		status,
		model.StepImplement,
		nil,
		metadata,
		execution,
		time.Now(),
		time.Now(),
	)
}

// Test only_implement=true workflow
func TestWorkflowDecisionService_OnlyImplement_PENDING_to_PICKED(t *testing.T) {
	service := NewWorkflowDecisionService(3)
	testSBI := createTestSBI(model.StatusPending, true, 1)

	action := service.DecideNextAction(testSBI, nil)

	if action.NextStatus != model.StatusPicked {
		t.Errorf("Expected PICKED, got %v", action.NextStatus)
	}
	if !action.ShouldIncrementTurn {
		t.Error("Expected turn increment")
	}
	if !action.SkipStepExecution {
		t.Error("Expected skip step execution")
	}
}

func TestWorkflowDecisionService_OnlyImplement_IMPLEMENTING_Success_to_DONE(t *testing.T) {
	service := NewWorkflowDecisionService(3)
	testSBI := createTestSBI(model.StatusImplementing, true, 1)

	stepResult := &dto.ExecuteStepOutput{
		Success: true,
	}

	action := service.DecideNextAction(testSBI, stepResult)

	if action.NextStatus != model.StatusDone {
		t.Errorf("Expected DONE, got %v", action.NextStatus)
	}
	if !action.ShouldIncrementTurn {
		t.Error("Expected turn increment")
	}
	if action.SkipStepExecution {
		t.Error("Should not skip step execution")
	}
}

func TestWorkflowDecisionService_OnlyImplement_REVIEWING_to_DONE(t *testing.T) {
	service := NewWorkflowDecisionService(3)
	testSBI := createTestSBI(model.StatusReviewing, true, 1)

	action := service.DecideNextAction(testSBI, nil)

	if action.NextStatus != model.StatusDone {
		t.Errorf("Expected DONE (auto-complete), got %v", action.NextStatus)
	}
	if action.ShouldIncrementTurn {
		t.Error("Should not increment turn for auto-complete")
	}
}

// Test only_implement=false workflow
func TestWorkflowDecisionService_FullWorkflow_IMPLEMENTING_Success_to_REVIEWING(t *testing.T) {
	service := NewWorkflowDecisionService(3)
	testSBI := createTestSBI(model.StatusImplementing, false, 1)

	stepResult := &dto.ExecuteStepOutput{
		Success: true,
	}

	action := service.DecideNextAction(testSBI, stepResult)

	if action.NextStatus != model.StatusReviewing {
		t.Errorf("Expected REVIEWING, got %v", action.NextStatus)
	}
	if !action.ShouldIncrementTurn {
		t.Error("Expected turn increment")
	}
}

func TestWorkflowDecisionService_FullWorkflow_REVIEWING_NeedsReload(t *testing.T) {
	service := NewWorkflowDecisionService(3)
	testSBI := createTestSBI(model.StatusReviewing, false, 1)

	stepResult := &dto.ExecuteStepOutput{
		Success: true,
	}

	action := service.DecideNextAction(testSBI, stepResult)

	if !action.NeedsReload {
		t.Error("Expected NeedsReload for REVIEWING status")
	}
	if action.NextStatus != model.StatusReviewing {
		t.Errorf("Expected REVIEWING (before reload), got %v", action.NextStatus)
	}
}

func TestWorkflowDecisionService_OnlyImplement_IMPLEMENTING_Failure_to_FAILED(t *testing.T) {
	service := NewWorkflowDecisionService(3)
	testSBI := createTestSBI(model.StatusImplementing, true, 1)

	stepResult := &dto.ExecuteStepOutput{
		Success:  false,
		ErrorMsg: "Test error",
	}

	action := service.DecideNextAction(testSBI, stepResult)

	if action.NextStatus != model.StatusFailed {
		t.Errorf("Expected FAILED, got %v", action.NextStatus)
	}
}
