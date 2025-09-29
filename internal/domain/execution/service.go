package execution

import (
	"fmt"
)

// ExecutionService provides domain services for SBI execution
type ExecutionService struct {
	repository SBIExecutionRepository
}

// NewExecutionService creates a new execution service
func NewExecutionService(repo SBIExecutionRepository) *ExecutionService {
	return &ExecutionService{
		repository: repo,
	}
}

// StartExecution starts a new SBI execution
func (s *ExecutionService) StartExecution(sbiID string) (*SBIExecution, error) {
	// Check if there's already an active execution for this SBI
	existing, err := s.repository.FindBySBIID(sbiID)
	if err == nil && existing != nil && !existing.IsCompleted() {
		return nil, fmt.Errorf("SBI %s already has an active execution", sbiID)
	}

	// Create new execution
	execution := NewSBIExecution(sbiID)

	// Save to repository
	if err := s.repository.Save(execution); err != nil {
		return nil, fmt.Errorf("failed to save execution: %w", err)
	}

	return execution, nil
}

// ProgressExecution progresses the execution to the next step
func (s *ExecutionService) ProgressExecution(executionID ExecutionID, decision Decision) (*SBIExecution, error) {
	// Load execution
	execution, err := s.repository.FindByID(executionID)
	if err != nil {
		return nil, fmt.Errorf("execution not found: %w", err)
	}

	if execution.IsCompleted() {
		return execution, fmt.Errorf("execution is already completed")
	}

	// Apply decision if in review status
	if execution.Status.IsReview() && decision.IsValid() && !decision.IsPending() {
		if err := execution.ApplyDecision(decision); err != nil {
			return nil, fmt.Errorf("failed to apply decision: %w", err)
		}
	}

	// Determine next step
	nextStep, err := execution.NextStep()
	if err != nil {
		return nil, fmt.Errorf("failed to determine next step: %w", err)
	}

	// Transition to next step
	if err := execution.TransitionTo(nextStep); err != nil {
		return nil, fmt.Errorf("failed to transition: %w", err)
	}

	// Check for force termination
	if execution.ShouldForceTerminate() && nextStep != StepDone {
		// Force transition to termination path
		nextStep = StepReviewerForceImplement
		if err := execution.TransitionTo(nextStep); err != nil {
			return nil, fmt.Errorf("failed to force terminate: %w", err)
		}
	}

	// Update in repository
	if err := s.repository.Update(execution); err != nil {
		return nil, fmt.Errorf("failed to update execution: %w", err)
	}

	return execution, nil
}

// GetExecutionPath returns the execution path for an SBI
func (s *ExecutionService) GetExecutionPath(executionID ExecutionID) ([]ExecutionStep, error) {
	execution, err := s.repository.FindByID(executionID)
	if err != nil {
		return nil, fmt.Errorf("execution not found: %w", err)
	}

	// Build the path based on the current step and decision history
	path := []ExecutionStep{StepReady}

	// Always includes first implementation and review
	path = append(path, StepImplementTry, StepFirstReview)

	// Determine the rest based on attempt number and decisions
	if execution.Step.ToNumber() <= 3 && execution.Decision == DecisionSucceeded {
		// Early success
		path = append(path, StepDone)
		return path, nil
	}

	// Second attempt
	if execution.Step.ToNumber() >= 4 {
		path = append(path, StepImplementSecondTry, StepSecondReview)
	}

	if execution.Step.ToNumber() <= 5 && execution.Step == StepDone {
		// Second attempt success
		path = append(path, StepDone)
		return path, nil
	}

	// Third attempt
	if execution.Step.ToNumber() >= 6 {
		path = append(path, StepImplementThirdTry, StepThirdReview)
	}

	if execution.Step.ToNumber() <= 7 && execution.Step == StepDone {
		// Third attempt success
		path = append(path, StepDone)
		return path, nil
	}

	// Force termination path
	if execution.Step.ToNumber() >= 8 {
		path = append(path, StepReviewerForceImplement, StepImplementerReview, StepDone)
	}

	return path, nil
}

// CompleteExecution marks an execution as completed
func (s *ExecutionService) CompleteExecution(executionID ExecutionID, finalDecision Decision) error {
	execution, err := s.repository.FindByID(executionID)
	if err != nil {
		return fmt.Errorf("execution not found: %w", err)
	}

	if execution.IsCompleted() {
		return fmt.Errorf("execution is already completed")
	}

	// Apply final decision
	execution.Decision = finalDecision

	// Transition to done
	if err := execution.TransitionTo(StepDone); err != nil {
		return fmt.Errorf("failed to complete execution: %w", err)
	}

	// Update in repository
	if err := s.repository.Update(execution); err != nil {
		return fmt.Errorf("failed to update execution: %w", err)
	}

	return nil
}

// GetActiveExecutions returns all active executions
func (s *ExecutionService) GetActiveExecutions() ([]*SBIExecution, error) {
	return s.repository.FindActive()
}

// IsExecutionStuck checks if an execution is stuck and needs intervention
func (s *ExecutionService) IsExecutionStuck(executionID ExecutionID) (bool, string) {
	execution, err := s.repository.FindByID(executionID)
	if err != nil {
		return false, ""
	}

	// Check if stuck in review with multiple failures
	if execution.Status == StatusReview &&
		execution.Decision == DecisionNeedsChanges &&
		execution.Attempt >= 3 {
		return true, "Multiple review failures, consider force termination"
	}

	// Check if stuck in implementation for too long
	if execution.Status == StatusWIP && execution.Attempt > 3 {
		return true, "Too many implementation attempts"
	}

	return false, ""
}
