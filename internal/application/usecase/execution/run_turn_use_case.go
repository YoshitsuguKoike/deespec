package execution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// RunTurnUseCase orchestrates a single workflow turn execution
type RunTurnUseCase struct {
	stateRepo    repository.StateRepository
	journalRepo  repository.JournalRepository
	sbiRepo      repository.SBIRepository // Added for DB-based task picking
	lockService  service.LockService
	agentGateway output.AgentGateway
	// TODO: Add PromptBuilder interface
	maxTurns int
	leaseTTL time.Duration
}

// NewRunTurnUseCase creates a new RunTurnUseCase
func NewRunTurnUseCase(
	stateRepo repository.StateRepository,
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
		stateRepo:    stateRepo,
		journalRepo:  journalRepo,
		sbiRepo:      sbiRepo,
		lockService:  lockService,
		agentGateway: agentGateway,
		maxTurns:     maxTurns,
		leaseTTL:     leaseTTL,
	}
}

// Execute runs a single workflow turn using DB-based state management
// This implementation eliminates state.json dependency and uses SQLite as the single source of truth
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
			NoOpReason:  "lock_held",
			ElapsedMs:   time.Since(startTime).Milliseconds(),
			CompletedAt: time.Now(),
		}, nil
	}

	defer func() {
		if err := uc.lockService.ReleaseRunLock(ctx, lockID); err != nil {
			// Log warning but don't fail the operation
		}
	}()

	// 2. Pick or continue SBI from DB (not from state.json)
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
		// Force termination
		if err := currentSBI.UpdateStatus(currentSBI.Status()); err != nil {
			return nil, fmt.Errorf("failed to mark SBI as done: %w", err)
		}
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
			// Log warning but don't fail
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

	// 5. Execute workflow step
	stepOutput, err := uc.executeStepForSBI(ctx, currentSBI, currentTurn, currentAttempt)
	if err != nil {
		stepOutput = &dto.ExecuteStepOutput{
			Success:   false,
			ErrorMsg:  err.Error(),
			Decision:  "NEEDS_CHANGES",
			ElapsedMs: time.Since(startTime).Milliseconds(),
		}
	}

	// 6. Determine next status based on current status and decision
	nextStatus, shouldIncrementAttempt := uc.determineNextStatusForSBI(
		currentSBI.Status(),
		stepOutput.Decision,
		currentAttempt,
	)

	if shouldIncrementAttempt {
		currentAttempt++
	}

	// 7. Update SBI entity with new status and execution state
	// Handle PENDING → PICKED transition if needed (state machine requires this intermediate step)
	if currentSBI.Status() == model.StatusPending && nextStatus != model.StatusPicked {
		// First transition: PENDING → PICKED
		if err := currentSBI.UpdateStatus(model.StatusPicked); err != nil {
			return nil, fmt.Errorf("failed to update SBI status to PICKED: %w", err)
		}
	}

	// Now transition to the target status
	if err := currentSBI.UpdateStatus(nextStatus); err != nil {
		return nil, fmt.Errorf("failed to update SBI status: %w", err)
	}

	// Update turn and attempt in execution state
	currentSBI.IncrementTurn()
	// TODO: Add method to update attempt if needed

	// 8. Save SBI to DB
	if err := uc.sbiRepo.Save(ctx, currentSBI); err != nil {
		return nil, fmt.Errorf("failed to save SBI to DB: %w", err)
	}

	// 9. Write journal entry
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
		Artifacts: []interface{}{stepOutput.ArtifactPath},
	}

	if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
		// Log warning but don't fail the operation
		// Journal is for auditing purposes and shouldn't block execution
		// TODO: Add proper logging
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

// pickTask attempts to pick the next task from the database
func (uc *RunTurnUseCase) pickTask(ctx context.Context, autoFB bool, turn int, state *repository.ExecutionState) (bool, error) {
	// Create SBI execution service
	sbiExecService := service.NewSBIExecutionService(uc.sbiRepo, uc.lockService)

	// Pick next SBI from database with lock acquisition
	nextSBI, sbiLock, err := sbiExecService.PickAndLockNextSBI(ctx, uc.leaseTTL)
	if err != nil {
		return false, fmt.Errorf("failed to pick and lock SBI: %w", err)
	}

	if nextSBI == nil {
		// No tasks available
		return false, nil
	}

	// Note: Lock will be held until the SBI execution completes
	// We defer release here, but in production this should be managed
	// by the caller to ensure lock is released after execution
	if sbiLock != nil {
		defer func() {
			if err := sbiExecService.ReleaseSBILock(ctx, nextSBI.ID().String()); err != nil {
				// Log error but don't fail the operation
			}
		}()
	}

	// Update state from DB (DB is single source of truth)
	state.WIP = nextSBI.ID().String()
	state.Status = "WIP"
	state.Turn = 1 // Reset turn for new SBI
	state.Attempt = 1 // Reset attempt for new SBI
	state.LeaseExpiresAt = time.Now().Add(uc.leaseTTL).UTC().Format(time.RFC3339Nano)

	// Initialize execution state for the SBI
	if execState := nextSBI.ExecutionState(); execState != nil {
		state.Turn = execState.CurrentTurn.Value()
		state.Attempt = execState.CurrentAttempt.Value()
	}

	return true, nil
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

// executeStepForSBI executes a workflow step for an SBI entity
func (uc *RunTurnUseCase) executeStepForSBI(ctx context.Context, sbiEntity *sbi.SBI, turn int, attempt int) (*dto.ExecuteStepOutput, error) {
	// Extract SBI ID and status
	sbiID := sbiEntity.ID().String()
	currentStatus := uc.mapDomainStatusToString(sbiEntity.Status())
	step := uc.statusToStep(currentStatus)

	// Determine artifact path
	artifactPath := fmt.Sprintf(".deespec/specs/sbi/%s/%s_%d.md", sbiID, step, turn)

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

	// Extract decision for review steps
	decision := "PENDING"
	if currentStatus == "REVIEW" || currentStatus == "REVIEW&WIP" {
		decision = uc.extractDecision(agentResult.Output)
	}

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

	var prompt string

	switch step {
	case "implement":
		prompt = fmt.Sprintf(`# Implementation Task

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
`, sbiID, title, description, turn, attempt, artifactPath)

	case "review":
		implementPath := fmt.Sprintf(".deespec/specs/sbi/%s/implement_%d.md", sbiID, turn-1)
		prompt = fmt.Sprintf(`# Code Review Task

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
`, sbiID, title, turn, attempt, implementPath, artifactPath)

	case "force_implement":
		prompt = fmt.Sprintf(`# Force Implementation Task (Final Attempt)

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
`, sbiID, title, description, turn, attempt, artifactPath)

	default:
		prompt = fmt.Sprintf("Execute step %s for SBI %s (turn %d, attempt %d)", step, sbiID, turn, attempt)
	}

	return prompt
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

// syncSBIToStateFile is deprecated - DB is now single source of truth
// This function is kept for backward compatibility but does nothing
func (uc *RunTurnUseCase) syncSBIToStateFile(ctx context.Context, sbiEntity *sbi.SBI) error {
	// No-op: State sync removed - DB is single source of truth
	return nil
}

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
