package execution

import (
	"fmt"
	"time"
)

// SBIExecution represents the execution state of an SBI task
type SBIExecution struct {
	ID          ExecutionID
	SBIID       string
	Step        ExecutionStep
	Status      ExecutionStatus
	Decision    Decision
	Attempt     int // Number of implementation attempts (1-3)
	StartedAt   time.Time
	UpdatedAt   time.Time
	CompletedAt *time.Time
}

// ExecutionID is a value object for execution identifier
type ExecutionID string

// NewExecutionID creates a new ExecutionID
func NewExecutionID(sbiID string, startedAt time.Time) ExecutionID {
	return ExecutionID(fmt.Sprintf("%s_%d", sbiID, startedAt.Unix()))
}

// NewSBIExecution creates a new SBI execution
func NewSBIExecution(sbiID string) *SBIExecution {
	now := time.Now()
	return &SBIExecution{
		ID:        NewExecutionID(sbiID, now),
		SBIID:     sbiID,
		Step:      StepReady,
		Status:    StatusReady,
		Decision:  DecisionPending,
		Attempt:   0,
		StartedAt: now,
		UpdatedAt: now,
	}
}

// TransitionTo transitions the execution to the next step based on current state and decision
func (e *SBIExecution) TransitionTo(nextStep ExecutionStep) error {
	// Validate transition
	if !e.Step.CanTransitionTo(nextStep) {
		return fmt.Errorf("invalid transition from %s to %s", e.Step, nextStep)
	}

	// Update step and related fields
	e.Step = nextStep
	e.Status = nextStep.ToStatus()
	e.UpdatedAt = time.Now()

	// Handle attempt counter for implementation steps
	if nextStep.IsImplementation() {
		if e.Step != StepImplementTry && e.Step != StepImplementSecondTry {
			e.Attempt++
		}
	}

	// Mark as completed if done
	if nextStep == StepDone {
		now := time.Now()
		e.CompletedAt = &now
	}

	return nil
}

// ApplyDecision applies a review decision to the execution
func (e *SBIExecution) ApplyDecision(decision Decision) error {
	if !e.Status.IsReview() {
		return fmt.Errorf("can only apply decision in review status, current: %s", e.Status)
	}

	if !decision.IsValid() {
		return fmt.Errorf("invalid decision: %s", decision)
	}

	e.Decision = decision
	e.UpdatedAt = time.Now()
	return nil
}

// IsCompleted returns true if the execution is completed
func (e *SBIExecution) IsCompleted() bool {
	return e.Step == StepDone
}

// ShouldForceTerminate returns true if the execution should be forcefully terminated
func (e *SBIExecution) ShouldForceTerminate() bool {
	// Force terminate after 3 failed attempts
	return e.Attempt >= 3 && e.Decision == DecisionNeedsChanges
}

// NextStep determines the next step based on current state and decision
func (e *SBIExecution) NextStep() (ExecutionStep, error) {
	switch e.Step {
	case StepReady:
		return StepImplementTry, nil

	case StepImplementTry:
		return StepFirstReview, nil

	case StepFirstReview:
		if e.Decision == DecisionSucceeded {
			return StepDone, nil
		}
		return StepImplementSecondTry, nil

	case StepImplementSecondTry:
		return StepSecondReview, nil

	case StepSecondReview:
		if e.Decision == DecisionSucceeded {
			return StepDone, nil
		}
		return StepImplementThirdTry, nil

	case StepImplementThirdTry:
		return StepThirdReview, nil

	case StepThirdReview:
		if e.Decision == DecisionSucceeded {
			return StepDone, nil
		}
		// Force termination path
		return StepReviewerForceImplement, nil

	case StepReviewerForceImplement:
		return StepImplementerReview, nil

	case StepImplementerReview:
		return StepDone, nil

	case StepDone:
		return StepDone, nil // No transition from done

	default:
		return "", fmt.Errorf("unknown step: %s", e.Step)
	}
}

// GetFinalDecision returns the final decision for completed executions
func (e *SBIExecution) GetFinalDecision() Decision {
	if !e.IsCompleted() {
		return DecisionPending
	}

	// Return the actual decision if it's final
	if e.Decision.IsFinal() {
		return e.Decision
	}

	// If went through force termination, it's failed
	if e.Step == StepDone && e.Attempt >= 3 {
		return DecisionFailed
	}

	return e.Decision
}
