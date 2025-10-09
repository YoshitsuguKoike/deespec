package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// RunTurnUseCase orchestrates a single workflow turn execution
type RunTurnUseCase struct {
	stateRepo    repository.StateRepository
	journalRepo  repository.JournalRepository
	lockService  service.LockService
	agentGateway output.AgentGateway
	// TODO: Add PromptBuilder interface
	// TODO: Add TaskPickerService interface
	maxTurns int
	leaseTTL time.Duration
}

// NewRunTurnUseCase creates a new RunTurnUseCase
func NewRunTurnUseCase(
	stateRepo repository.StateRepository,
	journalRepo repository.JournalRepository,
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
		stateRepo:    stateRepo,
		journalRepo:  journalRepo,
		lockService:  lockService,
		agentGateway: agentGateway,
		maxTurns:     maxTurns,
		leaseTTL:     leaseTTL,
	}
}

// Execute runs a single workflow turn
func (uc *RunTurnUseCase) Execute(ctx context.Context, input dto.RunTurnInput) (*dto.RunTurnOutput, error) {
	startTime := time.Now()

	// 1. Acquire run lock
	lockID, err := lock.NewLockID("system-runlock")
	if err != nil {
		return nil, fmt.Errorf("failed to create lock ID: %w", err)
	}

	runLock, err := uc.lockService.AcquireRunLock(ctx, lockID, uc.leaseTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire run lock: %w", err)
	}

	if runLock == nil {
		// Another instance is running
		return &dto.RunTurnOutput{
			NoOp:        true,
			ElapsedMs:   time.Since(startTime).Milliseconds(),
			CompletedAt: time.Now(),
		}, nil
	}

	defer func() {
		if err := uc.lockService.ReleaseRunLock(ctx, lockID); err != nil {
			// Log warning but don't fail the operation
		}
	}()

	// 2. Load current state
	state, err := uc.stateRepo.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load state: %w", err)
	}
	prevVersion := state.Version

	// 3. Turn management
	currentTurn := state.Turn + 1
	state.Turn = currentTurn

	// Check turn limit
	if currentTurn > uc.maxTurns {
		state.Status = "DONE"
		state.Decision = "FORCE_TERMINATED"
		state.WIP = ""
		state.LeaseExpiresAt = ""
		state.Turn = 0
		state.Attempt = 0

		if err := uc.stateRepo.Save(ctx, state); err != nil {
			return nil, fmt.Errorf("failed to save state after force termination: %w", err)
		}

		return &dto.RunTurnOutput{
			Turn:          currentTurn,
			NoOp:          false,
			PrevStatus:    state.Status,
			NextStatus:    "DONE",
			Decision:      "FORCE_TERMINATED",
			ElapsedMs:     time.Since(startTime).Milliseconds(),
			CompletedAt:   time.Now(),
			TaskCompleted: true,
		}, nil
	}

	// 4. Lease management
	if state.WIP != "" {
		if uc.isLeaseExpired(state) {
			// Lease expired, we can take over
		}
		uc.renewLease(state)
	}

	// 5. Pick task if no WIP
	if state.WIP == "" {
		picked, err := uc.pickTask(ctx, input.AutoFB, currentTurn, state)
		if err != nil {
			return nil, fmt.Errorf("failed to pick task: %w", err)
		}

		if !picked {
			// No task to pick
			return &dto.RunTurnOutput{
				Turn:        currentTurn,
				NoOp:        true,
				ElapsedMs:   time.Since(startTime).Milliseconds(),
				CompletedAt: time.Now(),
			}, nil
		}

		// Task picked, save state and exit (implementation starts next turn)
		if err := uc.stateRepo.Save(ctx, state); err != nil {
			return nil, fmt.Errorf("failed to save state after pick: %w", err)
		}

		return &dto.RunTurnOutput{
			Turn:        currentTurn,
			SBIID:       state.WIP,
			NoOp:        false,
			PrevStatus:  "PENDING",
			NextStatus:  state.Status,
			TaskPicked:  true,
			ElapsedMs:   time.Since(startTime).Milliseconds(),
			CompletedAt: time.Now(),
		}, nil
	}

	// 6. Execute workflow step based on current status
	prevStatus := state.Status
	prevStep := state.Current

	stepOutput, err := uc.executeStep(ctx, state, currentTurn)
	if err != nil {
		// Record error but continue to save state
		stepOutput = &dto.ExecuteStepOutput{
			Success:   false,
			ErrorMsg:  err.Error(),
			Decision:  "NEEDS_CHANGES",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}
	}

	// 7. Determine next status
	// TODO(human): Implement status transition logic
	// This is the core business logic that determines workflow progression
	// based on current status, decision, and attempt count.
	//
	// Input:
	//   - state.Status: current status (READY, WIP, REVIEW, REVIEW&WIP, DONE)
	//   - stepOutput.Decision: review decision (SUCCEEDED, NEEDS_CHANGES, FAILED)
	//   - state.Attempt: current attempt number (1-3)
	//
	// Output:
	//   - nextStatus: string (READY, WIP, REVIEW, REVIEW&WIP, DONE)
	//   - shouldIncrementAttempt: bool
	//
	// Business rules:
	//   - READY/WIP -> REVIEW (after implementation)
	//   - REVIEW -> DONE (if SUCCEEDED)
	//   - REVIEW -> WIP (if NEEDS_CHANGES, increment attempt)
	//   - REVIEW -> REVIEW&WIP (if 3 attempts failed)
	//   - REVIEW&WIP -> DONE (reviewer force implements)
	//
	// See run.go:nextStatusTransition() for reference implementation
	nextStatus, shouldIncrementAttempt := uc.determineNextStatus(state.Status, stepOutput.Decision, state.Attempt)

	if shouldIncrementAttempt {
		state.Attempt++
	}

	// 8. Update state
	state.Status = nextStatus
	state.Decision = stepOutput.Decision
	state.Current = uc.statusToStep(nextStatus)

	// Store artifact path
	if stepOutput.ArtifactPath != "" {
		if state.LastArtifacts == nil {
			state.LastArtifacts = make(map[string]string)
		}
		state.LastArtifacts[state.Current] = stepOutput.ArtifactPath
	}

	// Clear WIP when done
	taskCompleted := false
	if nextStatus == "DONE" {
		state.WIP = ""
		state.LeaseExpiresAt = ""
		state.Attempt = 0
		state.Turn = 0
		taskCompleted = true
	}

	// 9. Save state and journal atomically
	journalRec := uc.buildJournalRecord(state, stepOutput, currentTurn, startTime)
	if err := uc.stateRepo.SaveAtomic(ctx, state, journalRec); err != nil {
		return nil, fmt.Errorf("failed to save state and journal: %w", err)
	}

	_ = prevVersion // Used for optimistic locking in SaveAtomic

	// 10. Build output
	return &dto.RunTurnOutput{
		Turn:          currentTurn,
		SBIID:         state.WIP,
		NoOp:          false,
		PrevStatus:    prevStatus,
		NextStatus:    nextStatus,
		PrevStep:      prevStep,
		NextStep:      state.Current,
		Decision:      stepOutput.Decision,
		Attempt:       state.Attempt,
		ArtifactPath:  stepOutput.ArtifactPath,
		ErrorMsg:      stepOutput.ErrorMsg,
		ElapsedMs:     time.Since(startTime).Milliseconds(),
		CompletedAt:   time.Now(),
		TaskCompleted: taskCompleted,
	}, nil
}

// pickTask attempts to pick the next task
func (uc *RunTurnUseCase) pickTask(ctx context.Context, autoFB bool, turn int, state *repository.ExecutionState) (bool, error) {
	// TODO: Implement task picker service integration
	// For now, return no task available
	// This will be implemented when integrating with existing picker.go logic
	return false, nil
}

// executeStep executes a single workflow step
func (uc *RunTurnUseCase) executeStep(ctx context.Context, state *repository.ExecutionState, turn int) (*dto.ExecuteStepOutput, error) {
	// TODO: Build prompt using PromptBuilder interface
	// For now, use a simple placeholder
	prompt := fmt.Sprintf("Execute step %s for SBI %s (attempt %d)", state.Current, state.WIP, state.Attempt)

	// Execute agent
	startTime := time.Now()
	agentResult, err := uc.agentGateway.Execute(ctx, output.AgentRequest{
		Prompt:  prompt,
		Timeout: 5 * time.Minute,
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

	// Extract decision for review steps
	decision := "PENDING"
	if state.Status == "REVIEW" || state.Status == "REVIEW&WIP" {
		decision = uc.extractDecision(agentResult.Output)
	}

	// TODO: Save artifact to filesystem
	// This should be done via an ArtifactRepository
	artifactPath := fmt.Sprintf(".deespec/specs/sbi/%s/%s_%d.md", state.WIP, state.Current, turn)

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

// Helper functions

func (uc *RunTurnUseCase) isLeaseExpired(state *repository.ExecutionState) bool {
	if state.LeaseExpiresAt == "" {
		return true
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, state.LeaseExpiresAt)
	if err != nil {
		return true
	}
	return time.Now().After(expiresAt)
}

func (uc *RunTurnUseCase) renewLease(state *repository.ExecutionState) {
	state.LeaseExpiresAt = time.Now().Add(uc.leaseTTL).UTC().Format(time.RFC3339Nano)
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
	// Look for DECISION: keyword
	// For now, default to NEEDS_CHANGES
	return "NEEDS_CHANGES"
}

func (uc *RunTurnUseCase) buildJournalRecord(state *repository.ExecutionState, stepOutput *dto.ExecuteStepOutput, turn int, startTime time.Time) map[string]interface{} {
	artifacts := []interface{}{stepOutput.ArtifactPath}

	return map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       turn,
		"step":       state.Current,
		"status":     state.Status,
		"attempt":    state.Attempt,
		"decision":   state.Decision,
		"elapsed_ms": time.Since(startTime).Milliseconds(),
		"error":      stepOutput.ErrorMsg,
		"artifacts":  artifacts,
	}
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
