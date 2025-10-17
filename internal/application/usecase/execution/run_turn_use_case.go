package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// RunTurnUseCase orchestrates a single workflow turn execution
type RunTurnUseCase struct {
	journalRepo  repository.JournalRepository
	sbiRepo      repository.SBIRepository
	lockService  service.LockService
	agentGateway output.AgentGateway
	// TODO: Add PromptBuilder interface
	maxTurns int
	leaseTTL time.Duration
}

// NewRunTurnUseCase creates a new RunTurnUseCase
func NewRunTurnUseCase(
	journalRepo repository.JournalRepository,
	sbiRepo repository.SBIRepository,
	lockService service.LockService,
	agentGateway output.AgentGateway,
	maxTurns int,
	leaseTTL time.Duration,
) *RunTurnUseCase {
	if maxTurns <= 0 {
		maxTurns = 8 // Default
	}
	if leaseTTL == 0 {
		leaseTTL = 10 * time.Minute // Default
	}

	return &RunTurnUseCase{
		journalRepo:  journalRepo,
		sbiRepo:      sbiRepo,
		lockService:  lockService,
		agentGateway: agentGateway,
		maxTurns:     maxTurns,
		leaseTTL:     leaseTTL,
	}
}

// ExecuteForSBI executes a turn for a specific SBI (for parallel execution)
// This method skips RunLock acquisition and SBI picking, assuming the SBI is already locked
func (uc *RunTurnUseCase) ExecuteForSBI(ctx context.Context, sbiID string, input dto.RunTurnInput) (*dto.RunTurnOutput, error) {
	startTime := time.Now()

	// Load the specified SBI from repository
	currentSBI, err := uc.sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return nil, fmt.Errorf("failed to find SBI %s: %w", sbiID, err)
	}
	if currentSBI == nil {
		return nil, fmt.Errorf("SBI %s not found", sbiID)
	}

	// Get execution state from SBI entity
	execState := currentSBI.ExecutionState()
	if execState == nil {
		return nil, fmt.Errorf("SBI %s has no execution state", currentSBI.ID())
	}

	currentTurn := execState.CurrentTurn.Value()
	currentAttempt := execState.CurrentAttempt.Value()
	prevStatus := currentSBI.Status()

	// Increment turn for this execution
	currentTurn++

	// Check turn limit
	if currentTurn > uc.maxTurns {
		// Force termination - transition to DONE status
		if err := currentSBI.UpdateStatus(model.StatusDone); err != nil {
			return nil, fmt.Errorf("failed to mark SBI as done: %w", err)
		}
		// Record work completion time for force termination
		currentSBI.MarkAsCompleted()
		if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
			return nil, fmt.Errorf("failed to save SBI after force termination: %w", err)
		}

		// Write journal entry for force termination
		journalRecord := &repository.JournalRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			SBIID:     currentSBI.ID().String(),
			Turn:      currentTurn,
			Step:      "force_terminated",
			Status:    "DONE",
			Attempt:   currentAttempt,
			Decision:  "FORCE_TERMINATED",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     fmt.Sprintf("Exceeded max turns (%d)", uc.maxTurns),
			Artifacts: []interface{}{},
		}

		if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry (force termination)\n")
			fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Step: force_terminated\n",
				currentSBI.ID().String(), currentTurn)
		}

		return &dto.RunTurnOutput{
			Turn:          currentTurn,
			SBIID:         currentSBI.ID().String(),
			NoOp:          false,
			PrevStatus:    uc.mapDomainStatusToString(prevStatus),
			NextStatus:    "DONE",
			Decision:      "FORCE_TERMINATED",
			ElapsedMs:     time.Since(startTime).Milliseconds(),
			CompletedAt:   time.Now(),
			TaskCompleted: true,
		}, nil
	}

	// CRITICAL FIX: Handle status-only transitions without calling AI agent
	// These are O(1) database updates, not O(AI_call) operations

	// Case 1: PENDING → PICKED (task selection)
	if prevStatus == model.StatusPending {
		if err := currentSBI.UpdateStatus(model.StatusPicked); err != nil {
			return nil, fmt.Errorf("failed to update SBI status to PICKED: %w", err)
		}
		currentSBI.MarkAsStarted()
		currentSBI.IncrementTurn()

		if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
			return nil, fmt.Errorf("failed to save SBI to DB: %w", err)
		}

		// Write journal entry for PICK
		journalRecord := &repository.JournalRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			SBIID:     currentSBI.ID().String(),
			Turn:      currentTurn,
			Step:      "pick",
			Status:    "WIP",
			Attempt:   currentAttempt,
			Decision:  "PICKED",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     "",
			Artifacts: []interface{}{},
		}

		if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry (pick)\n")
			fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		}

		return &dto.RunTurnOutput{
			Turn:          currentTurn,
			SBIID:         currentSBI.ID().String(),
			NoOp:          false,
			PrevStatus:    uc.mapDomainStatusToString(prevStatus),
			NextStatus:    "WIP",
			Decision:      "PICKED",
			Attempt:       currentAttempt,
			ArtifactPath:  "",
			ErrorMsg:      "",
			ElapsedMs:     time.Since(startTime).Milliseconds(),
			CompletedAt:   time.Now(),
			TaskCompleted: false,
		}, nil
	}

	// Case 2: PICKED → IMPLEMENTING (status initialization)
	// This happens when a PICKED task is selected for execution
	if prevStatus == model.StatusPicked {
		if err := currentSBI.UpdateStatus(model.StatusImplementing); err != nil {
			return nil, fmt.Errorf("failed to update SBI status to IMPLEMENTING: %w", err)
		}
		currentSBI.IncrementTurn()

		if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
			return nil, fmt.Errorf("failed to save SBI to DB: %w", err)
		}

		// Write journal entry for status transition
		journalRecord := &repository.JournalRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			SBIID:     currentSBI.ID().String(),
			Turn:      currentTurn,
			Step:      "status_init",
			Status:    "WIP",
			Attempt:   currentAttempt,
			Decision:  "INITIALIZED",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     "",
			Artifacts: []interface{}{},
		}

		if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry (status_init)\n")
			fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		}

		return &dto.RunTurnOutput{
			Turn:          currentTurn,
			SBIID:         currentSBI.ID().String(),
			NoOp:          false,
			PrevStatus:    uc.mapDomainStatusToString(prevStatus),
			NextStatus:    "WIP",
			Decision:      "INITIALIZED",
			Attempt:       currentAttempt,
			ArtifactPath:  "",
			ErrorMsg:      "",
			ElapsedMs:     time.Since(startTime).Milliseconds(),
			CompletedAt:   time.Now(),
			TaskCompleted: false,
		}, nil
	}

	// Execute workflow step (for IMPLEMENTING, REVIEWING, etc.)
	stepOutput, err := uc.executeStepForSBI(ctx, currentSBI, currentTurn, currentAttempt)
	if err != nil {
		stepOutput = &dto.ExecuteStepOutput{
			Success:   false,
			ErrorMsg:  err.Error(),
			Decision:  "NEEDS_CHANGES",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}
	}

	// Check if this is a REVIEW step
	// Since v0.2.13, review decisions are handled by `deespec sbi review` command
	// which updates the status directly. We only need to wait for AI execution to complete.
	currentStatus := currentSBI.Status()
	isReviewStep := (currentStatus == model.StatusReviewing)

	var nextStatus model.Status
	var shouldIncrementAttempt bool

	if isReviewStep {
		// For REVIEW steps: AI agent executes `deespec sbi review --decision X --stdin` command
		// The command updates the status (DONE or IMPLEMENTING) directly in the database
		// We don't need to update status here - just reload the SBI to get the updated status
		reloadedSBI, err := uc.sbiRepo.Find(ctx, repository.SBIID(currentSBI.ID().String()))
		if err != nil {
			return nil, fmt.Errorf("failed to reload SBI after review: %w", err)
		}
		if reloadedSBI == nil {
			return nil, fmt.Errorf("SBI disappeared after review: %s", currentSBI.ID().String())
		}

		// Use the reloaded status - it was already updated by the review command
		nextStatus = reloadedSBI.Status()
		currentSBI = reloadedSBI
		shouldIncrementAttempt = false
	} else {
		// For non-REVIEW steps: determine next status based on current status and decision
		nextStatus, shouldIncrementAttempt = uc.determineNextStatusForSBI(
			currentSBI.Status(),
			stepOutput.Decision,
			currentAttempt,
		)

		if shouldIncrementAttempt {
			currentAttempt++
		}

		// Update SBI entity with new status and execution state
		// Handle PENDING → PICKED transition if needed
		if currentSBI.Status() == model.StatusPending && nextStatus != model.StatusPicked {
			if err := currentSBI.UpdateStatus(model.StatusPicked); err != nil {
				return nil, fmt.Errorf("failed to update SBI status to PICKED: %w", err)
			}
			// Record work start time when task is picked
			currentSBI.MarkAsStarted()
		}

		// Now transition to the target status
		if err := currentSBI.UpdateStatus(nextStatus); err != nil {
			return nil, fmt.Errorf("failed to update SBI status: %w", err)
		}

		// Record work completion time when task is done or failed
		if nextStatus == model.StatusDone || nextStatus == model.StatusFailed {
			currentSBI.MarkAsCompleted()
		}

		// Update turn in execution state
		currentSBI.IncrementTurn()
	}

	// NOTE: done.md generation is commented out due to performance concerns
	//
	// BACKGROUND: Generating done.md takes 2-5 minutes per task, blocking the next task from starting.
	// This significantly slows down workflow throughput, especially in parallel execution mode.
	//
	// OPTIONS TO CONSIDER (when you read this comment):
	// 1. 動作効率を犠牲にしてまでdoneを出すか？
	//    - Uncomment the code below to restore done.md generation
	//    - Accept 2-5 minute delay per task completion
	//    - Benefit: Comprehensive completion report for each task
	//
	// 2. 安定性を犠牲にして非同期でdoneを出すか？
	//    - Implement goroutine-based async generation
	//    - Complexity: Error handling, goroutine lifecycle management, potential race conditions
	//    - Benefit: No blocking, but errors may go unnoticed
	//
	// 3. このままコメントアウトし続けるか？
	//    - Current state: Maximum throughput
	//    - Loss: No done.md summary (review.md still contains final decision)
	//
	// DECISION DATE: 2025-10-12
	//
	// var doneArtifactPath string
	// if nextStatus == model.StatusDone {
	// 	doneArtifactPath = fmt.Sprintf(".deespec/specs/sbi/%s/done.md", currentSBI.ID().String())
	// 	doneStepOutput, err := uc.executeStepForSBI(ctx, currentSBI, currentTurn, currentAttempt)
	// 	if err != nil {
	// 		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to generate done.md\n")
	// 		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
	// 	} else if doneStepOutput.Success {
	// 		doneArtifactPath = doneStepOutput.ArtifactPath
	// 	}
	// }
	var doneArtifactPath string // Keep variable for journal compatibility

	// Save SBI to DB
	if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
		return nil, fmt.Errorf("failed to save SBI to DB: %w", err)
	}

	// Write journal entry
	artifacts := []interface{}{stepOutput.ArtifactPath}
	if doneArtifactPath != "" {
		artifacts = append(artifacts, doneArtifactPath)
	}

	journalRecord := &repository.JournalRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		SBIID:     currentSBI.ID().String(),
		Turn:      currentTurn,
		Step:      uc.statusToStep(uc.mapDomainStatusToString(nextStatus)),
		Status:    uc.mapDomainStatusToString(nextStatus),
		Attempt:   currentAttempt,
		Decision:  stepOutput.Decision,
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Error:     stepOutput.ErrorMsg,
		Artifacts: artifacts,
	}

	if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Step: %s, Status: %s\n",
			currentSBI.ID().String(), currentTurn,
			uc.statusToStep(uc.mapDomainStatusToString(nextStatus)),
			uc.mapDomainStatusToString(nextStatus))
	}

	// Build output
	taskCompleted := (nextStatus == model.StatusDone)

	return &dto.RunTurnOutput{
		Turn:          currentTurn,
		SBIID:         currentSBI.ID().String(),
		NoOp:          false,
		PrevStatus:    uc.mapDomainStatusToString(prevStatus),
		NextStatus:    uc.mapDomainStatusToString(nextStatus),
		Decision:      stepOutput.Decision,
		Attempt:       currentAttempt,
		ArtifactPath:  stepOutput.ArtifactPath,
		ErrorMsg:      stepOutput.ErrorMsg,
		ElapsedMs:     time.Since(startTime).Milliseconds(),
		CompletedAt:   time.Now(),
		TaskCompleted: taskCompleted,
	}, nil
}

// Execute runs a single workflow turn using DB-based state management
// This implementation eliminates state.json dependency and uses SQLite as the single source of truth
// Note: RunLock should be acquired by the caller (CLI layer) before calling this method
func (uc *RunTurnUseCase) Execute(ctx context.Context, input dto.RunTurnInput) (*dto.RunTurnOutput, error) {
	startTime := time.Now()

	// 1. Pick or continue SBI from DB (not from state.json)
	// Note: RunLock is managed by CLI layer, not by UseCase layer
	sbiExecService := service.NewSBIExecutionService(uc.sbiRepo, uc.lockService)

	// Try to pick next SBI with lock
	var currentSBI *sbi.SBI
	currentSBI, sbiLock, err := sbiExecService.PickAndLockNextSBI(ctx, uc.leaseTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to pick and lock SBI: %w", err)
	}

	if currentSBI == nil {
		// No tasks available
		return &dto.RunTurnOutput{
			NoOp:        true,
			NoOpReason:  "no_tasks",
			ElapsedMs:   time.Since(startTime).Milliseconds(),
			CompletedAt: time.Now(),
		}, nil
	}

	// Release lock when done
	defer func() {
		if sbiLock != nil {
			if err := sbiExecService.ReleaseSBILock(ctx, currentSBI.ID().String()); err != nil {
				// Log error but don't fail
			}
		}
	}()

	// 3. Get execution state from SBI entity (not from state.json)
	execState := currentSBI.ExecutionState()
	if execState == nil {
		return nil, fmt.Errorf("SBI %s has no execution state", currentSBI.ID())
	}

	currentTurn := execState.CurrentTurn.Value()
	currentAttempt := execState.CurrentAttempt.Value()
	prevStatus := currentSBI.Status()

	// Increment turn for this execution
	currentTurn++

	// 4. Check turn limit
	if currentTurn > uc.maxTurns {
		// Force termination - transition to DONE status
		if err := currentSBI.UpdateStatus(model.StatusDone); err != nil {
			return nil, fmt.Errorf("failed to mark SBI as done: %w", err)
		}
		// Record work completion time for force termination
		currentSBI.MarkAsCompleted()
		if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
			return nil, fmt.Errorf("failed to save SBI after force termination: %w", err)
		}

		// Write journal entry for force termination
		journalRecord := &repository.JournalRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			SBIID:     currentSBI.ID().String(),
			Turn:      currentTurn,
			Step:      "force_terminated",
			Status:    "DONE",
			Attempt:   currentAttempt,
			Decision:  "FORCE_TERMINATED",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     fmt.Sprintf("Exceeded max turns (%d)", uc.maxTurns),
			Artifacts: []interface{}{},
		}

		if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
			// Log warning to stderr but don't fail the operation
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry (force termination)\n")
			fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
			fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Step: force_terminated\n",
				currentSBI.ID().String(), currentTurn)
		}

		// Note: State sync removed - DB is single source of truth

		return &dto.RunTurnOutput{
			Turn:          currentTurn,
			SBIID:         currentSBI.ID().String(),
			NoOp:          false,
			PrevStatus:    uc.mapDomainStatusToString(prevStatus),
			NextStatus:    "DONE",
			Decision:      "FORCE_TERMINATED",
			ElapsedMs:     time.Since(startTime).Milliseconds(),
			CompletedAt:   time.Now(),
			TaskCompleted: true,
		}, nil
	}

	// CRITICAL FIX: Handle status-only transitions without calling AI agent
	// These are O(1) database updates, not O(AI_call) operations

	// Case 1: PENDING → PICKED (task selection)
	if prevStatus == model.StatusPending {
		if err := currentSBI.UpdateStatus(model.StatusPicked); err != nil {
			return nil, fmt.Errorf("failed to update SBI status to PICKED: %w", err)
		}
		currentSBI.MarkAsStarted()
		currentSBI.IncrementTurn()

		if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
			return nil, fmt.Errorf("failed to save SBI to DB: %w", err)
		}

		// Write journal entry for PICK
		journalRecord := &repository.JournalRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			SBIID:     currentSBI.ID().String(),
			Turn:      currentTurn,
			Step:      "pick",
			Status:    "WIP",
			Attempt:   currentAttempt,
			Decision:  "PICKED",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     "",
			Artifacts: []interface{}{},
		}

		if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry (pick)\n")
			fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		}

		return &dto.RunTurnOutput{
			Turn:          currentTurn,
			SBIID:         currentSBI.ID().String(),
			NoOp:          false,
			PrevStatus:    uc.mapDomainStatusToString(prevStatus),
			NextStatus:    "WIP",
			Decision:      "PICKED",
			Attempt:       currentAttempt,
			ArtifactPath:  "",
			ErrorMsg:      "",
			ElapsedMs:     time.Since(startTime).Milliseconds(),
			CompletedAt:   time.Now(),
			TaskCompleted: false,
		}, nil
	}

	// Case 2: PICKED → IMPLEMENTING (status initialization)
	// This happens when PickNextSBI() selects a PICKED task that was picked in previous turn
	if prevStatus == model.StatusPicked {
		if err := currentSBI.UpdateStatus(model.StatusImplementing); err != nil {
			return nil, fmt.Errorf("failed to update SBI status to IMPLEMENTING: %w", err)
		}
		currentSBI.IncrementTurn()

		if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
			return nil, fmt.Errorf("failed to save SBI to DB: %w", err)
		}

		// Write journal entry for status transition
		journalRecord := &repository.JournalRecord{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
			SBIID:     currentSBI.ID().String(),
			Turn:      currentTurn,
			Step:      "status_init",
			Status:    "WIP",
			Attempt:   currentAttempt,
			Decision:  "INITIALIZED",
			ElapsedMs: time.Since(startTime).Milliseconds(),
			Error:     "",
			Artifacts: []interface{}{},
		}

		if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry (status_init)\n")
			fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		}

		return &dto.RunTurnOutput{
			Turn:          currentTurn,
			SBIID:         currentSBI.ID().String(),
			NoOp:          false,
			PrevStatus:    uc.mapDomainStatusToString(prevStatus),
			NextStatus:    "WIP",
			Decision:      "INITIALIZED",
			Attempt:       currentAttempt,
			ArtifactPath:  "",
			ErrorMsg:      "",
			ElapsedMs:     time.Since(startTime).Milliseconds(),
			CompletedAt:   time.Now(),
			TaskCompleted: false,
		}, nil
	}

	// 5. Execute workflow step (for IMPLEMENTING, REVIEWING, etc.)
	stepOutput, err := uc.executeStepForSBI(ctx, currentSBI, currentTurn, currentAttempt)
	if err != nil {
		stepOutput = &dto.ExecuteStepOutput{
			Success:   false,
			ErrorMsg:  err.Error(),
			Decision:  "NEEDS_CHANGES",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}
	}

	// 6. Check if this is a REVIEW step
	// Since v0.2.13, review decisions are handled by `deespec sbi review` command
	// which updates the status directly. We only need to wait for AI execution to complete.
	currentStatus := currentSBI.Status()
	isReviewStep := (currentStatus == model.StatusReviewing)

	var nextStatus model.Status
	var shouldIncrementAttempt bool

	if isReviewStep {
		// For REVIEW steps: AI agent executes `deespec sbi review --decision X --stdin` command
		// The command updates the status (DONE or IMPLEMENTING) directly in the database
		// We don't need to update status here - just reload the SBI to get the updated status
		reloadedSBI, err := uc.sbiRepo.Find(ctx, repository.SBIID(currentSBI.ID().String()))
		if err != nil {
			return nil, fmt.Errorf("failed to reload SBI after review: %w", err)
		}
		if reloadedSBI == nil {
			return nil, fmt.Errorf("SBI disappeared after review: %s", currentSBI.ID().String())
		}

		// Use the reloaded status - it was already updated by the review command
		nextStatus = reloadedSBI.Status()
		currentSBI = reloadedSBI
		shouldIncrementAttempt = false
	} else {
		// 7. For non-REVIEW steps: determine next status based on current status and decision
		nextStatus, shouldIncrementAttempt = uc.determineNextStatusForSBI(
			currentSBI.Status(),
			stepOutput.Decision,
			currentAttempt,
		)

		if shouldIncrementAttempt {
			currentAttempt++
		}

		// 8. Update SBI entity with new status and execution state
		// Handle PENDING → PICKED transition if needed (state machine requires this intermediate step)
		if currentSBI.Status() == model.StatusPending && nextStatus != model.StatusPicked {
			// First transition: PENDING → PICKED
			if err := currentSBI.UpdateStatus(model.StatusPicked); err != nil {
				return nil, fmt.Errorf("failed to update SBI status to PICKED: %w", err)
			}
			// Record work start time when task is picked
			currentSBI.MarkAsStarted()
		}

		// Now transition to the target status
		if err := currentSBI.UpdateStatus(nextStatus); err != nil {
			return nil, fmt.Errorf("failed to update SBI status: %w", err)
		}

		// Record work completion time when task is done or failed
		if nextStatus == model.StatusDone || nextStatus == model.StatusFailed {
			currentSBI.MarkAsCompleted()
		}

		// Update turn and attempt in execution state
		currentSBI.IncrementTurn()
		// TODO: Add method to update attempt if needed
	}

	// NOTE: done.md generation is commented out due to performance concerns
	//
	// BACKGROUND: Generating done.md takes 2-5 minutes per task, blocking the next task from starting.
	// This significantly slows down workflow throughput, especially in parallel execution mode.
	//
	// OPTIONS TO CONSIDER (when you read this comment):
	// 1. 動作効率を犠牲にしてまでdoneを出すか？
	//    - Uncomment the code below to restore done.md generation
	//    - Accept 2-5 minute delay per task completion
	//    - Benefit: Comprehensive completion report for each task
	//
	// 2. 安定性を犠牲にして非同期でdoneを出すか？
	//    - Implement goroutine-based async generation
	//    - Complexity: Error handling, goroutine lifecycle management, potential race conditions
	//    - Benefit: No blocking, but errors may go unnoticed
	//
	// 3. このままコメントアウトし続けるか？
	//    - Current state: Maximum throughput
	//    - Loss: No done.md summary (review.md still contains final decision)
	//
	// DECISION DATE: 2025-10-12
	//
	// var doneArtifactPath string
	// if nextStatus == model.StatusDone {
	// 	doneArtifactPath = fmt.Sprintf(".deespec/specs/sbi/%s/done.md", currentSBI.ID().String())
	// 	doneStepOutput, err := uc.executeStepForSBI(ctx, currentSBI, currentTurn, currentAttempt)
	// 	if err != nil {
	// 		// Log warning but don't fail - done.md is optional
	// 		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to generate done.md\n")
	// 		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
	// 	} else if doneStepOutput.Success {
	// 		doneArtifactPath = doneStepOutput.ArtifactPath
	// 	}
	// }
	var doneArtifactPath string // Keep variable for journal compatibility

	// 8. Save SBI to DB
	if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
		return nil, fmt.Errorf("failed to save SBI to DB: %w", err)
	}

	// 9. Write journal entry
	artifacts := []interface{}{stepOutput.ArtifactPath}
	if doneArtifactPath != "" {
		artifacts = append(artifacts, doneArtifactPath)
	}

	journalRecord := &repository.JournalRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		SBIID:     currentSBI.ID().String(),
		Turn:      currentTurn,
		Step:      uc.statusToStep(uc.mapDomainStatusToString(nextStatus)),
		Status:    uc.mapDomainStatusToString(nextStatus),
		Attempt:   currentAttempt,
		Decision:  stepOutput.Decision,
		ElapsedMs: time.Since(startTime).Milliseconds(),
		Error:     stepOutput.ErrorMsg,
		Artifacts: artifacts,
	}

	if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
		// Log warning to stderr but don't fail the operation
		// Journal is for auditing purposes and shouldn't block execution
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Step: %s, Status: %s\n",
			currentSBI.ID().String(), currentTurn,
			uc.statusToStep(uc.mapDomainStatusToString(nextStatus)),
			uc.mapDomainStatusToString(nextStatus))
		fmt.Fprintf(os.Stderr, "   Journal Record: Timestamp=%s, Attempt=%d, Decision=%s\n",
			journalRecord.Timestamp, currentAttempt, stepOutput.Decision)
	}

	// 10. Note: State sync removed - DB is single source of truth

	// 11. Build output
	taskCompleted := (nextStatus == currentSBI.Status()) // Check if status is DONE

	return &dto.RunTurnOutput{
		Turn:          currentTurn,
		SBIID:         currentSBI.ID().String(),
		NoOp:          false,
		PrevStatus:    uc.mapDomainStatusToString(prevStatus),
		NextStatus:    uc.mapDomainStatusToString(nextStatus),
		Decision:      stepOutput.Decision,
		Attempt:       currentAttempt,
		ArtifactPath:  stepOutput.ArtifactPath,
		ErrorMsg:      stepOutput.ErrorMsg,
		ElapsedMs:     time.Since(startTime).Milliseconds(),
		CompletedAt:   time.Now(),
		TaskCompleted: taskCompleted,
	}, nil
}

// Helper functions

// executeStepForSBI executes a workflow step for an SBI entity
func (uc *RunTurnUseCase) executeStepForSBI(ctx context.Context, sbiEntity *sbi.SBI, turn int, attempt int) (*dto.ExecuteStepOutput, error) {
	// Extract SBI ID and status
	sbiID := sbiEntity.ID().String()
	currentStatus := uc.mapDomainStatusToString(sbiEntity.Status())
	step := uc.statusToStep(currentStatus)

	// Determine artifact path
	// Since v0.2.13, reports are saved to .deespec/reports/sbi/ via commands
	// This path is used for journal records and fallback file creation
	var artifactPath string
	if step == "done" {
		artifactPath = fmt.Sprintf(".deespec/reports/sbi/%s/done.md", sbiID)
	} else {
		artifactPath = fmt.Sprintf(".deespec/reports/sbi/%s/%s_%d.md", sbiID, step, turn)
	}

	// Build prompt with artifact generation instruction
	prompt := uc.buildPromptWithArtifact(sbiEntity, step, turn, attempt, artifactPath)

	// Execute agent
	startTime := time.Now()
	agentResult, err := uc.agentGateway.Execute(ctx, output.AgentRequest{
		Prompt:  prompt,
		Timeout: 10 * time.Minute,
	})
	if err != nil {
		return &dto.ExecuteStepOutput{
			Success:     false,
			ErrorMsg:    err.Error(),
			ElapsedMs:   time.Since(startTime).Milliseconds(),
			StartedAt:   startTime,
			CompletedAt: time.Now(),
		}, err
	}

	// Note: Decision extraction for review steps is no longer needed here
	// Since v0.2.13, AI agents execute `deespec sbi review --decision SUCCEEDED --stdin` command
	// which updates the status directly in ReviewSBIUseCase.Execute()
	// The decision extraction logic is only kept for backward compatibility with old workflow
	decision := "PENDING"

	// Check if artifact file was created by Claude
	artifactCreated := false
	if _, err := os.Stat(artifactPath); err == nil {
		artifactCreated = true
	}

	// If Claude didn't create the artifact, save the output ourselves as fallback
	if !artifactCreated {
		artifactDir := filepath.Dir(artifactPath)
		if err := os.MkdirAll(artifactDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create artifact directory: %w", err)
		}

		if err := os.WriteFile(artifactPath, []byte(agentResult.Output), 0644); err != nil {
			return nil, fmt.Errorf("failed to write artifact file: %w", err)
		}
	}

	return &dto.ExecuteStepOutput{
		Success:      true,
		Output:       agentResult.Output,
		Decision:     decision,
		ArtifactPath: artifactPath,
		ElapsedMs:    time.Since(startTime).Milliseconds(),
		StartedAt:    startTime,
		CompletedAt:  time.Now(),
	}, nil
}

// buildPromptWithArtifact builds a prompt that instructs Claude to create an artifact file
func (uc *RunTurnUseCase) buildPromptWithArtifact(sbiEntity *sbi.SBI, step string, turn int, attempt int, artifactPath string) string {
	sbiID := sbiEntity.ID().String()
	title := sbiEntity.Title()
	description := sbiEntity.Description()

	// Get current working directory
	workDir, err := os.Getwd()
	if err != nil {
		workDir = "."
	}

	// Generate prior context instructions
	priorContext := uc.buildPriorContextInstructions(sbiID, turn)

	// Prepare template data
	data := PromptTemplateData{
		WorkDir:         workDir,
		SBIID:           sbiID,
		Title:           title,
		Description:     description,
		Turn:            turn,
		Attempt:         attempt,
		Step:            step,
		SBIDir:          fmt.Sprintf(".deespec/specs/sbi/%s", sbiID),
		ArtifactPath:    artifactPath,
		PriorContext:    priorContext,
		TaskDescription: description,
	}

	// Determine template path based on step
	var templatePath string
	switch step {
	case "implement":
		templatePath = ".deespec/prompts/WIP.md"
	case "review":
		templatePath = ".deespec/prompts/REVIEW.md"
		// Since v0.2.13, reports are in .deespec/reports/sbi/
		data.ImplementPath = fmt.Sprintf(".deespec/reports/sbi/%s/implement_%d.md", sbiID, turn-1)
	case "force_implement":
		templatePath = ".deespec/prompts/REVIEW_AND_WIP.md"
	case "done":
		templatePath = ".deespec/prompts/DONE.md"
		// Collect all implement and review paths
		data.AllImplementPaths = uc.collectImplementPaths(sbiID, turn)
		data.AllReviewPaths = uc.collectReviewPaths(sbiID, turn)
	default:
		// Fallback to simple prompt if no template found
		return fmt.Sprintf("Execute step %s for SBI %s (turn %d, attempt %d)", step, sbiID, turn, attempt)
	}

	// Try to expand template
	prompt, err := uc.expandTemplate(templatePath, data)
	if err != nil {
		// Fallback to old-style hardcoded prompts if template fails
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to load template %s: %v\n", templatePath, err)
		fmt.Fprintf(os.Stderr, "   Falling back to built-in prompt\n")
		return uc.buildFallbackPrompt(sbiEntity, step, turn, attempt, artifactPath, priorContext)
	}

	return prompt
}

// PromptTemplateData holds data for template expansion
type PromptTemplateData struct {
	WorkDir           string
	SBIID             string
	Title             string
	Description       string
	Turn              int
	Attempt           int
	Step              string
	SBIDir            string
	ArtifactPath      string
	ImplementPath     string
	AllImplementPaths []string
	AllReviewPaths    []string
	PriorContext      string
	TaskDescription   string
}

// expandTemplate reads a template file and expands it with given data
func (uc *RunTurnUseCase) expandTemplate(templatePath string, data PromptTemplateData) (string, error) {
	// Read template file
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	// Parse template
	tmpl, err := template.New(filepath.Base(templatePath)).Parse(string(templateContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", templatePath, err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", templatePath, err)
	}

	return buf.String(), nil
}

// buildFallbackPrompt generates prompts using hardcoded templates (fallback when template files are not available)
func (uc *RunTurnUseCase) buildFallbackPrompt(sbiEntity *sbi.SBI, step string, turn int, attempt int, artifactPath string, priorContext string) string {
	sbiID := sbiEntity.ID().String()
	title := sbiEntity.Title()
	description := sbiEntity.Description()

	switch step {
	case "implement":
		return fmt.Sprintf(`%s# Implementation Task

**SBI ID**: %s
**Title**: %s
**Description**: %s
**Turn**: %d
**Attempt**: %d

## Your Task

Please implement the above specification. Write your complete implementation report to the file:

**Output File**: %s

The report should include:
1. Overview of what you implemented
2. Key design decisions
3. Code changes made
4. Any challenges or considerations
5. Testing notes (if applicable)

Use the Write tool to create this file with your full implementation report.
`, priorContext, sbiID, title, description, turn, attempt, artifactPath)

	case "review":
		// Since v0.2.13, reports are in .deespec/reports/sbi/
		implementPath := fmt.Sprintf(".deespec/reports/sbi/%s/implement_%d.md", sbiID, turn-1)
		return fmt.Sprintf(`%s# Code Review Task

**SBI ID**: %s
**Title**: %s
**Turn**: %d
**Attempt**: %d

## Your Task

Please review the implementation at: %s

Write your complete review report to the file:

**Output File**: %s

The review report should include:
1. Overview of the implementation quality
2. Specific issues found (if any)
3. Suggestions for improvement
4. **DECISION**: SUCCEEDED | NEEDS_CHANGES | FAILED

**Important**: End your report with a clear DECISION line:
- DECISION: SUCCEEDED (if implementation is correct and complete)
- DECISION: NEEDS_CHANGES (if minor fixes needed)
- DECISION: FAILED (if major issues found)

Use the Write tool to create this file with your full review report.
`, priorContext, sbiID, title, turn, attempt, implementPath, artifactPath)

	case "force_implement":
		return fmt.Sprintf(`%s# Force Implementation Task (Final Attempt)

**SBI ID**: %s
**Title**: %s
**Description**: %s
**Turn**: %d

## Context

This is the final implementation attempt after %d previous attempts. As the reviewer, please implement the solution directly.

Write your complete implementation report to the file:

**Output File**: %s

The report should include:
1. Final implementation approach
2. Code changes made
3. Why this approach resolves previous issues
4. Verification steps

Use the Write tool to create this file with your full implementation report.
`, priorContext, sbiID, title, description, turn, attempt, artifactPath)

	case "done":
		return fmt.Sprintf(`%s# Task Completion Report

**SBI ID**: %s
**Title**: %s
**Description**: %s
**Final Turn**: %d

## Context

This task has been completed. Please create a comprehensive completion report.

Write your complete completion report to the file:

**Output File**: %s

The report should include:
1. Task overview and what was accomplished
2. Implementation approach and key decisions
3. Files modified and major changes
4. Challenges encountered and solutions
5. Testing approach and results
6. Technical debt or follow-up items

Use the Write tool to create this file with your full completion report.
`, priorContext, sbiID, title, description, turn, artifactPath)

	default:
		return fmt.Sprintf("Execute step %s for SBI %s (turn %d, attempt %d)", step, sbiID, turn, attempt)
	}
}

// buildPriorContextInstructions generates instructions to read prior artifacts
func (uc *RunTurnUseCase) buildPriorContextInstructions(sbiID string, currentTurn int) string {
	var context strings.Builder

	context.WriteString("## IMPORTANT: Review Prior Work First\n\n")
	context.WriteString("Before starting your task, you MUST:\n\n")
	context.WriteString("### 1. Read All Existing Artifacts\n\n")
	context.WriteString("Check and read files in the following locations:\n")
	context.WriteString(fmt.Sprintf("- `.deespec/specs/sbi/%s/` - Specification and old reports\n", sbiID))
	context.WriteString(fmt.Sprintf("- `.deespec/reports/sbi/%s/` - New implementation and review reports\n\n", sbiID))
	context.WriteString("Expected files:\n")
	context.WriteString("- `spec.md`: Original specification (in specs directory)\n")

	if currentTurn > 1 {
		context.WriteString("- Previous implementation reports: `implement_*.md` (in reports directory)\n")
		context.WriteString("- Previous review reports: `review_*.md` (in reports directory)\n")
		context.WriteString("- Notes and rollup files if any\n\n")

		context.WriteString("**Why this matters**:\n")
		context.WriteString("- Understand what has been tried before\n")
		context.WriteString("- Avoid repeating failed approaches\n")
		context.WriteString("- Build upon previous progress\n")
		context.WriteString("- Maintain consistency across turns\n\n")

		context.WriteString(fmt.Sprintf("**Action**: Use the Read tool to read `.deespec/specs/sbi/%s/spec.md` and check both `.deespec/specs/sbi/%s/` and `.deespec/reports/sbi/%s/` for any `implement_*.md` or `review_*.md` files from previous turns.\n\n", sbiID, sbiID, sbiID))
	} else {
		context.WriteString(fmt.Sprintf("\n**Action**: Use the Read tool to read `.deespec/specs/sbi/%s/spec.md` to understand the full specification.\n\n", sbiID))
	}

	return context.String()
}

// determineNextStatusForSBI determines next status for SBI entity
func (uc *RunTurnUseCase) determineNextStatusForSBI(currentStatus model.Status, decision string, attempt int) (model.Status, bool) {
	// Map to string for easier handling
	statusStr := uc.mapDomainStatusToString(currentStatus)

	// Use existing logic
	nextStatusStr, shouldIncrement := uc.determineNextStatus(statusStr, decision, attempt)

	// Map back to domain status
	nextStatus := uc.mapStringToDomainStatus(nextStatusStr)

	return nextStatus, shouldIncrement
}

// mapDomainStatusToString converts domain Status to string representation
func (uc *RunTurnUseCase) mapDomainStatusToString(status model.Status) string {
	switch status {
	case model.StatusPending:
		return "READY"
	case model.StatusPicked:
		return "WIP"
	case model.StatusImplementing:
		return "WIP"
	case model.StatusReviewing:
		return "REVIEW"
	case model.StatusDone:
		return "DONE"
	case model.StatusFailed:
		return "FAILED"
	default:
		return fmt.Sprintf("%v", status)
	}
}

// mapStringToDomainStatus converts string status to domain Status
func (uc *RunTurnUseCase) mapStringToDomainStatus(statusStr string) model.Status {
	switch statusStr {
	case "READY":
		return model.StatusPending
	case "WIP":
		return model.StatusImplementing
	case "REVIEW":
		return model.StatusReviewing
	case "REVIEW&WIP":
		return model.StatusImplementing // Force implementation maps to implementing
	case "DONE":
		return model.StatusDone
	case "FAILED":
		return model.StatusFailed
	default:
		return model.StatusPending
	}
}

func (uc *RunTurnUseCase) statusToStep(status string) string {
	switch status {
	case "READY", "WIP":
		return "implement"
	case "REVIEW":
		return "review"
	case "REVIEW&WIP":
		return "force_implement"
	case "DONE":
		return "done"
	default:
		return "unknown"
	}
}

func (uc *RunTurnUseCase) extractDecision(output string) string {
	// Simple extraction - can be improved with regex
	// TODO: Move to a dedicated service
	if len(output) == 0 {
		return "NEEDS_CHANGES"
	}

	// Check for decision keywords in output
	// Look for explicit SUCCEEDED/FAILED/NEEDS_CHANGES markers
	if contains(output, "DECISION: SUCCEEDED") || contains(output, "[SUCCEEDED]") {
		return "SUCCEEDED"
	}
	if contains(output, "DECISION: FAILED") || contains(output, "[FAILED]") {
		return "FAILED"
	}
	if contains(output, "DECISION: NEEDS_CHANGES") || contains(output, "[NEEDS_CHANGES]") {
		return "NEEDS_CHANGES"
	}

	// For mock agents that don't provide explicit decisions, assume success
	// This allows testing and development to proceed smoothly
	if contains(output, "[Gemini Mock]") || contains(output, "[Codex Mock]") || contains(output, "[Mock]") {
		return "SUCCEEDED"
	}

	// Default to NEEDS_CHANGES for real agents without explicit decision
	return "NEEDS_CHANGES"
}

// extractDecisionWithLogging extracts decision from artifact file with metadata validation and logging
// Returns decision string and source indicator for debugging
func (uc *RunTurnUseCase) extractDecisionWithLogging(artifactPath string, agentOutput string, sbiID string) (decision string, source string) {
	// First, try to determine the turn number from artifactPath (e.g., "review_4.md" -> 4)
	// This is needed to check the new reports directory
	var reportPath string
	if strings.Contains(artifactPath, "review_") {
		// Extract turn number from path like ".deespec/specs/sbi/{SBIID}/review_{turn}.md"
		filename := filepath.Base(artifactPath)
		// filename is like "review_4.md"
		parts := strings.Split(filename, "_")
		if len(parts) == 2 {
			turnStr := strings.TrimSuffix(parts[1], ".md")
			// Construct new reports path: .deespec/reports/sbi/{SBIID}/review_{turn}.md
			reportPath = filepath.Join(".deespec", "reports", "sbi", sbiID, fmt.Sprintf("review_%s.md", turnStr))
		}
	}

	// Try to read from new reports location first (priority)
	var content []byte
	var err error
	var actualPath string

	if reportPath != "" {
		content, err = os.ReadFile(reportPath)
		if err == nil {
			actualPath = reportPath
		}
	}

	// Fall back to old artifact path if new path doesn't exist
	if err != nil {
		content, err = os.ReadFile(artifactPath)
		actualPath = artifactPath
	}

	if err != nil {
		// Artifact doesn't exist in either location, use agent output
		decision = uc.extractDecision(agentOutput)
		fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=agent_output (file not found), Decision=%s, CheckedPaths=[%s, %s]\n",
			sbiID, decision, reportPath, artifactPath)
		return decision, "agent_output"
	}

	fmt.Fprintf(os.Stderr, "[decision] SBI=%s, ReadFrom=%s\n", sbiID, actualPath)
	fileContent := string(content)

	// Extract decision from head (first 20 lines, ## Summary section)
	headDecision := uc.extractDecisionFromHead(fileContent)

	// Extract decision from tail JSON (last 5 lines)
	tailDecision := uc.extractDecisionFromTailJSON(fileContent)

	// Check if both metadata and JSON match
	if headDecision != "" && tailDecision != "" && headDecision == tailDecision {
		// Both sources agree - high confidence
		fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=metadata_match, HeadDecision=%s, TailDecision=%s, FinalDecision=%s\n",
			sbiID, headDecision, tailDecision, headDecision)
		return headDecision, "metadata_match"
	}

	// Metadata doesn't match or is missing - use agent output as fallback
	decision = uc.extractDecision(agentOutput)
	fmt.Fprintf(os.Stderr, "[decision] SBI=%s, Source=agent_output (mismatch), HeadDecision=%s, TailDecision=%s, AgentDecision=%s\n",
		sbiID, headDecision, tailDecision, decision)
	return decision, "agent_output"
}

// extractDecisionFromHead extracts DECISION from first 20 lines in ## Summary section
func (uc *RunTurnUseCase) extractDecisionFromHead(content string) string {
	lines := strings.Split(content, "\n")
	maxLines := 20
	if len(lines) < maxLines {
		maxLines = len(lines)
	}

	// Look for ## Summary section in first 20 lines
	inSummary := false
	for i := 0; i < maxLines; i++ {
		line := strings.TrimSpace(lines[i])

		// Check if we're entering Summary section
		if strings.HasPrefix(line, "## Summary") {
			inSummary = true
			continue
		}

		// Check if we're leaving Summary section (new ## header)
		if inSummary && strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Summary") {
			break
		}

		// Look for DECISION in Summary section
		if inSummary {
			if contains(line, "DECISION: SUCCEEDED") {
				return "SUCCEEDED"
			}
			if contains(line, "DECISION: FAILED") {
				return "FAILED"
			}
			if contains(line, "DECISION: NEEDS_CHANGES") {
				return "NEEDS_CHANGES"
			}
		}
	}

	return "" // Not found
}

// extractDecisionFromTailJSON extracts decision from JSON in last 5 lines
func (uc *RunTurnUseCase) extractDecisionFromTailJSON(content string) string {
	lines := strings.Split(content, "\n")

	// Check last 5 lines for JSON
	startIdx := len(lines) - 5
	if startIdx < 0 {
		startIdx = 0
	}

	for i := len(lines) - 1; i >= startIdx; i-- {
		line := strings.TrimSpace(lines[i])

		// Skip empty lines
		if line == "" {
			continue
		}

		// Look for JSON with decision field
		if strings.HasPrefix(line, "{") && strings.Contains(line, "decision") {
			// Try to parse as JSON
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(line), &result); err == nil {
				if decisionVal, ok := result["decision"]; ok {
					if decisionStr, ok := decisionVal.(string); ok {
						return strings.ToUpper(decisionStr)
					}
				}
			}
		}
	}

	return "" // Not found
}

// Helper function for case-insensitive string contains
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(findSubstring(s, substr) >= 0))
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// determineNextStatus determines the next status based on current state
func (uc *RunTurnUseCase) determineNextStatus(currentStatus string, decision string, attempt int) (nextStatus string, shouldIncrementAttempt bool) {
	maxAttempts := 3 // Default max attempts
	// TODO: Get maxAttempts from config

	switch currentStatus {
	case "", "READY":
		// READY -> WIP (start implementation)
		return "WIP", false

	case "WIP":
		// WIP -> REVIEW (after implementation)
		return "REVIEW", false

	case "REVIEW":
		if decision == "SUCCEEDED" {
			// REVIEW -> DONE (success)
			return "DONE", false
		} else if attempt >= maxAttempts {
			// REVIEW -> REVIEW&WIP (force termination after max attempts)
			// This prevents infinite loops when AI keeps returning NEEDS_CHANGES or FAILED
			return "REVIEW&WIP", false
		} else {
			// REVIEW -> WIP (needs changes, retry)
			return "WIP", true // shouldIncrementAttempt = true
		}

	case "REVIEW&WIP":
		// REVIEW&WIP -> DONE (after force termination)
		return "DONE", false

	case "DONE":
		// DONE -> DONE (terminal state)
		return "DONE", false

	default:
		// Unknown status, default to READY
		return "READY", false
	}
}

// collectImplementPaths collects all implement_N.md paths for an SBI
func (uc *RunTurnUseCase) collectImplementPaths(sbiID string, maxTurn int) []string {
	var paths []string
	// Since v0.2.13, reports are in .deespec/reports/sbi/ but old reports may be in .deespec/specs/sbi/
	reportsDir := fmt.Sprintf(".deespec/reports/sbi/%s", sbiID)
	specsDir := fmt.Sprintf(".deespec/specs/sbi/%s", sbiID)

	// Check for implement files from turn 1 to maxTurn in both locations
	for turn := 1; turn < maxTurn; turn++ {
		// Try new location first
		implementPath := filepath.Join(reportsDir, fmt.Sprintf("implement_%d.md", turn))
		if _, err := os.Stat(implementPath); err == nil {
			paths = append(paths, implementPath)
			continue
		}
		// Fall back to old location
		implementPath = filepath.Join(specsDir, fmt.Sprintf("implement_%d.md", turn))
		if _, err := os.Stat(implementPath); err == nil {
			paths = append(paths, implementPath)
		}
	}

	return paths
}

// collectReviewPaths collects all review_N.md paths for an SBI
func (uc *RunTurnUseCase) collectReviewPaths(sbiID string, maxTurn int) []string {
	var paths []string
	// Since v0.2.13, reports are in .deespec/reports/sbi/ but old reports may be in .deespec/specs/sbi/
	reportsDir := fmt.Sprintf(".deespec/reports/sbi/%s", sbiID)
	specsDir := fmt.Sprintf(".deespec/specs/sbi/%s", sbiID)

	// Check for review files from turn 1 to maxTurn in both locations
	for turn := 1; turn < maxTurn; turn++ {
		// Try new location first
		reviewPath := filepath.Join(reportsDir, fmt.Sprintf("review_%d.md", turn))
		if _, err := os.Stat(reviewPath); err == nil {
			paths = append(paths, reviewPath)
			continue
		}
		// Fall back to old location
		reviewPath = filepath.Join(specsDir, fmt.Sprintf("review_%d.md", turn))
		if _, err := os.Stat(reviewPath); err == nil {
			paths = append(paths, reviewPath)
		}
	}

	return paths
}
