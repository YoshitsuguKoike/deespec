package service

import (
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

// WorkflowDecisionService manages all workflow decision logic for SBI execution
// This service centralizes the decision-making process for determining next actions
type WorkflowDecisionService struct {
	maxAttempts int
}

// NewWorkflowDecisionService creates a new workflow decision service
func NewWorkflowDecisionService(maxAttempts int) *WorkflowDecisionService {
	if maxAttempts <= 0 {
		maxAttempts = 3 // Default
	}
	return &WorkflowDecisionService{
		maxAttempts: maxAttempts,
	}
}

// WorkflowAction represents the decided next action for an SBI
type WorkflowAction struct {
	NextStatus             model.Status // Next status to transition to
	NextStep               model.Step   // Next step to execute
	ShouldIncrementTurn    bool         // Whether to increment turn counter
	ShouldIncrementAttempt bool         // Whether to increment attempt counter
	NeedsReload            bool         // Whether to reload SBI from DB before applying
	SkipStepExecution      bool         // Whether to skip executeStepForSBI (for status-only transitions)
	Reason                 string       // Decision reason for debugging
}

// DecideNextAction determines the next workflow action based on SBI state and execution result
// This is the central decision point for all workflow logic
func (s *WorkflowDecisionService) DecideNextAction(
	sbiEntity *sbi.SBI,
	stepResult *dto.ExecuteStepOutput,
) *WorkflowAction {
	currentStatus := sbiEntity.Status()

	// Priority 1: Check only_implement flag first
	if sbiEntity.OnlyImplement() {
		return s.decideForImplementOnly(sbiEntity, stepResult, currentStatus)
	}

	// Priority 2: Handle REVIEW step (special case - AI updates DB directly)
	if currentStatus == model.StatusReviewing {
		return s.decideForReviewStep(sbiEntity, stepResult)
	}

	// Priority 3: Normal full workflow
	return s.decideForFullWorkflow(sbiEntity, stepResult, currentStatus)
}

// decideForImplementOnly handles decision logic for only_implement=true SBIs
func (s *WorkflowDecisionService) decideForImplementOnly(
	sbiEntity *sbi.SBI,
	stepResult *dto.ExecuteStepOutput,
	currentStatus model.Status,
) *WorkflowAction {
	switch currentStatus {
	case model.StatusPending:
		// PENDING → PICKED (task selection)
		return &WorkflowAction{
			NextStatus:          model.StatusPicked,
			NextStep:            model.StepImplement,
			ShouldIncrementTurn: true,
			SkipStepExecution:   true, // No AI execution needed
			Reason:              "only_implement: PENDING→PICKED (task selection)",
		}

	case model.StatusPicked:
		// PICKED → IMPLEMENTING (initialization)
		return &WorkflowAction{
			NextStatus:          model.StatusImplementing,
			NextStep:            model.StepImplement,
			ShouldIncrementTurn: true,
			SkipStepExecution:   true, // No AI execution needed
			Reason:              "only_implement: PICKED→IMPLEMENTING (init)",
		}

	case model.StatusImplementing:
		if stepResult == nil {
			// Step not yet executed, continue with current status
			return &WorkflowAction{
				NextStatus: model.StatusImplementing,
				NextStep:   model.StepImplement,
				Reason:     "only_implement: IMPLEMENTING (awaiting execution)",
			}
		}

		if stepResult.Success {
			// IMPLEMENTING (success) → DONE (skip REVIEW)
			return &WorkflowAction{
				NextStatus:          model.StatusDone,
				NextStep:            model.StepDone,
				ShouldIncrementTurn: true,
				Reason:              "only_implement: IMPLEMENTING→DONE (skip REVIEW)",
			}
		} else {
			// IMPLEMENTING (failure) → FAILED
			return &WorkflowAction{
				NextStatus: model.StatusFailed,
				NextStep:   model.StepImplement,
				Reason:     "only_implement: IMPLEMENTING→FAILED",
			}
		}

	case model.StatusReviewing:
		// Abnormal state: SBI was in REVIEW before only_implement was set to true
		// Auto-complete to DONE
		return &WorkflowAction{
			NextStatus:        model.StatusDone,
			NextStep:          model.StepDone,
			SkipStepExecution: true, // No AI execution needed
			Reason:            "only_implement: REVIEWING→DONE (auto-complete stuck review)",
		}

	case model.StatusDone, model.StatusFailed:
		// Terminal states - no change
		return &WorkflowAction{
			NextStatus: currentStatus,
			NextStep:   sbiEntity.CurrentStep(),
			Reason:     "only_implement: terminal state (no change)",
		}

	default:
		// Unknown status - treat as pending
		return &WorkflowAction{
			NextStatus: model.StatusPending,
			NextStep:   model.StepImplement,
			Reason:     "only_implement: unknown status, reset to PENDING",
		}
	}
}

// decideForReviewStep handles decision logic for REVIEWING status
// Special case: AI agent executes `deespec sbi report` which updates DB directly
func (s *WorkflowDecisionService) decideForReviewStep(
	sbiEntity *sbi.SBI,
	stepResult *dto.ExecuteStepOutput,
) *WorkflowAction {
	// For REVIEW steps: AI agent executes `deespec sbi report --decision X --stdin` command
	// The command updates the status (DONE or IMPLEMENTING) directly in the database
	// We need to reload the SBI to get the updated status
	return &WorkflowAction{
		NextStatus:  model.StatusReviewing, // Will be overridden by reloaded status
		NextStep:    model.StepReview,
		NeedsReload: true, // Signal to reload SBI from DB
		Reason:      "full_workflow: REVIEWING (reload for AI decision)",
	}
}

// decideForFullWorkflow handles decision logic for full workflow (only_implement=false)
func (s *WorkflowDecisionService) decideForFullWorkflow(
	sbiEntity *sbi.SBI,
	stepResult *dto.ExecuteStepOutput,
	currentStatus model.Status,
) *WorkflowAction {

	switch currentStatus {
	case model.StatusPending:
		// PENDING → PICKED (task selection)
		return &WorkflowAction{
			NextStatus:          model.StatusPicked,
			NextStep:            model.StepImplement,
			ShouldIncrementTurn: true,
			SkipStepExecution:   true,
			Reason:              "full_workflow: PENDING→PICKED (task selection)",
		}

	case model.StatusPicked:
		// PICKED → IMPLEMENTING (initialization)
		return &WorkflowAction{
			NextStatus:          model.StatusImplementing,
			NextStep:            model.StepImplement,
			ShouldIncrementTurn: true,
			SkipStepExecution:   true,
			Reason:              "full_workflow: PICKED→IMPLEMENTING (init)",
		}

	case model.StatusImplementing:
		if stepResult == nil {
			// Step not yet executed
			return &WorkflowAction{
				NextStatus: model.StatusImplementing,
				NextStep:   model.StepImplement,
				Reason:     "full_workflow: IMPLEMENTING (awaiting execution)",
			}
		}

		if stepResult.Success {
			// IMPLEMENTING (success) → REVIEWING
			return &WorkflowAction{
				NextStatus:          model.StatusReviewing,
				NextStep:            model.StepReview,
				ShouldIncrementTurn: true,
				Reason:              "full_workflow: IMPLEMENTING→REVIEWING",
			}
		} else {
			// IMPLEMENTING (failure) → FAILED
			return &WorkflowAction{
				NextStatus: model.StatusFailed,
				NextStep:   model.StepImplement,
				Reason:     "full_workflow: IMPLEMENTING→FAILED",
			}
		}

	case model.StatusReviewing:
		// This case is handled by decideForReviewStep()
		// This is a fallback in case it's called directly
		return s.decideForReviewStep(sbiEntity, stepResult)

	case model.StatusDone:
		// Terminal state - no change
		return &WorkflowAction{
			NextStatus: model.StatusDone,
			NextStep:   model.StepDone,
			Reason:     "full_workflow: DONE (terminal state)",
		}

	case model.StatusFailed:
		// Terminal state - no change
		return &WorkflowAction{
			NextStatus: model.StatusFailed,
			NextStep:   model.StepImplement,
			Reason:     "full_workflow: FAILED (terminal state)",
		}

	default:
		// Unknown status - reset to pending
		return &WorkflowAction{
			NextStatus: model.StatusPending,
			NextStep:   model.StepImplement,
			Reason:     "full_workflow: unknown status, reset to PENDING",
		}
	}
}

// DecideNextActionForReviewDecision determines action after review decision is made
// This is called after AI has executed `deespec sbi report --decision X`
func (s *WorkflowDecisionService) DecideNextActionForReviewDecision(
	sbiEntity *sbi.SBI,
	decision string,
) *WorkflowAction {
	execState := sbiEntity.ExecutionState()
	currentAttempt := execState.CurrentAttempt.Value()

	switch decision {
	case "SUCCEEDED":
		// REVIEWING → DONE (approved)
		return &WorkflowAction{
			NextStatus:          model.StatusDone,
			NextStep:            model.StepDone,
			ShouldIncrementTurn: true,
			Reason:              "review_decision: SUCCEEDED→DONE",
		}

	case "NEEDS_CHANGES", "FAILED":
		if currentAttempt >= s.maxAttempts {
			// Max attempts reached - force implementation
			return &WorkflowAction{
				NextStatus:          model.StatusImplementing,
				NextStep:            model.StepImplement,
				ShouldIncrementTurn: true,
				Reason:              "review_decision: max attempts reached, force implement",
			}
		} else {
			// REVIEWING → IMPLEMENTING (retry with incremented attempt)
			return &WorkflowAction{
				NextStatus:             model.StatusImplementing,
				NextStep:               model.StepImplement,
				ShouldIncrementTurn:    true,
				ShouldIncrementAttempt: true,
				Reason:                 "review_decision: " + decision + "→IMPLEMENTING (retry)",
			}
		}

	default:
		// Unknown decision - treat as NEEDS_CHANGES
		return &WorkflowAction{
			NextStatus:             model.StatusImplementing,
			NextStep:               model.StepImplement,
			ShouldIncrementTurn:    true,
			ShouldIncrementAttempt: true,
			Reason:                 "review_decision: unknown decision, retry",
		}
	}
}
