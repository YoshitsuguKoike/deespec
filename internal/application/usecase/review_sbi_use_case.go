package usecase

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// ReviewSBIUseCase handles review decision reporting from AI agents
type ReviewSBIUseCase struct {
	sbiRepo     repository.SBIRepository
	journalRepo repository.JournalRepository
}

// NewReviewSBIUseCase creates a new ReviewSBIUseCase
func NewReviewSBIUseCase(sbiRepo repository.SBIRepository, journalRepo repository.JournalRepository) *ReviewSBIUseCase {
	return &ReviewSBIUseCase{
		sbiRepo:     sbiRepo,
		journalRepo: journalRepo,
	}
}

// Execute processes a review decision and updates SBI status accordingly
func (uc *ReviewSBIUseCase) Execute(ctx context.Context, sbiID string, turn int, decision string) error {
	// 1. Load SBI from database
	sbi, err := uc.sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return fmt.Errorf("failed to find SBI: %w", err)
	}
	if sbi == nil {
		return fmt.Errorf("SBI not found: %s", sbiID)
	}

	// 2. Validate turn number (prevent stale review application)
	execState := sbi.ExecutionState()
	if execState == nil {
		return fmt.Errorf("SBI %s has no execution state", sbiID)
	}

	currentTurn := execState.CurrentTurn.Value()
	if turn != currentTurn {
		return fmt.Errorf(
			"turn mismatch: SBI is at turn %d, but review is for turn %d (ignoring stale review)",
			currentTurn, turn,
		)
	}

	// 3. Validate status (only accept REVIEWING status)
	if sbi.Status() != model.StatusReviewing {
		return fmt.Errorf(
			"invalid status: expected REVIEWING, got %s (current turn: %d)",
			sbi.Status(), currentTurn,
		)
	}

	// 4. Update status based on decision
	previousStatus := sbi.Status()
	var nextStatus model.Status
	var shouldIncrementTurn bool

	switch decision {
	case "SUCCEEDED":
		// REVIEWING → DONE (review passed)
		nextStatus = model.StatusDone
		shouldIncrementTurn = false
		if err := sbi.UpdateStatus(model.StatusDone); err != nil {
			return fmt.Errorf("failed to update status to DONE: %w", err)
		}
		// Record work completion time
		sbi.MarkAsCompleted()
		fmt.Printf("✅ SBI %s marked as DONE (turn %d review: SUCCEEDED)\n", sbiID, turn)

	case "NEEDS_CHANGES", "FAILED":
		// REVIEWING → IMPLEMENTING (needs another turn)
		nextStatus = model.StatusImplementing
		shouldIncrementTurn = true
		if err := sbi.UpdateStatus(model.StatusImplementing); err != nil {
			return fmt.Errorf("failed to update status to IMPLEMENTING: %w", err)
		}
		// Increment turn for next implementation cycle
		sbi.IncrementTurn()
		fmt.Printf("🔄 SBI %s moved to next turn (turn %d → %d, review: %s)\n",
			sbiID, turn, turn+1, decision)

	default:
		return fmt.Errorf("invalid decision: %s (must be SUCCEEDED, NEEDS_CHANGES, or FAILED)", decision)
	}

	// 5. Save SBI to database
	if err := uc.sbiRepo.Save(ctx, sbi); err != nil {
		return fmt.Errorf("failed to save SBI: %w", err)
	}

	// 6. Write journal entry for audit trail
	journalRecord := &repository.JournalRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		SBIID:     sbiID,
		Turn:      turn,
		Step:      "review_command",
		Status:    string(nextStatus),
		Attempt:   execState.CurrentAttempt.Value(),
		Decision:  decision,
		ElapsedMs: 0, // Command execution, not agent execution
		Error:     "",
		Artifacts: []interface{}{fmt.Sprintf("review_%d.md", turn)},
	}

	if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
		// Log warning but don't fail - journal is for auditing
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Decision: %s\n", sbiID, turn, decision)
	}

	// 7. Log status transition
	fmt.Fprintf(os.Stderr, "[review_command] SBI=%s, Turn=%d, Decision=%s, Transition=%s→%s, IncrementTurn=%v\n",
		sbiID, turn, decision, previousStatus, nextStatus, shouldIncrementTurn)

	return nil
}
