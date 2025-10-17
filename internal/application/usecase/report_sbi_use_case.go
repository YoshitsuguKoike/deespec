package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/buildinfo"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// ReportSBIUseCase handles implementation and review report submission from AI agents
type ReportSBIUseCase struct {
	sbiRepo        repository.SBIRepository
	journalRepo    repository.JournalRepository
	execLogRepo    repository.SBIExecLogRepository
}

// NewReportSBIUseCase creates a new ReportSBIUseCase
func NewReportSBIUseCase(
	sbiRepo repository.SBIRepository,
	journalRepo repository.JournalRepository,
	execLogRepo repository.SBIExecLogRepository,
) *ReportSBIUseCase {
	return &ReportSBIUseCase{
		sbiRepo:     sbiRepo,
		journalRepo: journalRepo,
		execLogRepo: execLogRepo,
	}
}

// Execute processes a report (implement or review) and updates SBI status accordingly
func (uc *ReportSBIUseCase) Execute(ctx context.Context, sbiID string, turn int, step string, decision string, content string) error {
	// 1. Load SBI from database
	sbi, err := uc.sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return fmt.Errorf("failed to find SBI: %w", err)
	}
	if sbi == nil {
		return fmt.Errorf("SBI not found: %s", sbiID)
	}

	// 2. Validate turn number
	execState := sbi.ExecutionState()
	if execState == nil {
		return fmt.Errorf("SBI %s has no execution state", sbiID)
	}

	currentTurn := execState.CurrentTurn.Value()
	if turn != currentTurn {
		return fmt.Errorf(
			"turn mismatch: SBI is at turn %d, but report is for turn %d",
			currentTurn, turn,
		)
	}

	// 3. Determine report directory and filename
	reportDir := filepath.Join(".deespec", "reports", "sbi", sbiID)
	var filename string

	switch step {
	case "implement":
		filename = fmt.Sprintf("implement_%d.md", turn)
	case "review":
		filename = fmt.Sprintf("review_%d.md", turn)
	default:
		return fmt.Errorf("unsupported step type: %s (must be 'implement' or 'review')", step)
	}

	// 4. Create report directory if it doesn't exist
	if err := os.MkdirAll(reportDir, 0755); err != nil {
		return fmt.Errorf("failed to create report directory: %w", err)
	}

	// 5. Write report content to file
	reportPath := filepath.Join(reportDir, filename)
	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	// 6. Update SBI status based on step and decision
	previousStatus := sbi.Status()
	var nextStatus model.Status

	switch step {
	case "implement":
		// IMPLEMENTING → REVIEWING (implementation complete, needs review)
		if previousStatus != model.StatusImplementing {
			return fmt.Errorf("invalid status for implement report: expected IMPLEMENTING, got %s", previousStatus)
		}
		nextStatus = model.StatusReviewing
		if err := sbi.UpdateStatus(model.StatusReviewing); err != nil {
			return fmt.Errorf("failed to update status to REVIEWING: %w", err)
		}
		fmt.Printf("✅ Implementation completed, moving to review (SBI: %s, Turn: %d)\n", sbiID, turn)

	case "review":
		// Validate decision for review step
		if decision == "" {
			return fmt.Errorf("decision is required for review step")
		}

		// Validate status
		if previousStatus != model.StatusReviewing {
			return fmt.Errorf("invalid status for review report: expected REVIEWING, got %s", previousStatus)
		}

		switch decision {
		case "SUCCEEDED":
			// REVIEWING → DONE (review passed)
			nextStatus = model.StatusDone
			if err := sbi.UpdateStatus(model.StatusDone); err != nil {
				return fmt.Errorf("failed to update status to DONE: %w", err)
			}
			// Record work completion time
			sbi.MarkAsCompleted()
			fmt.Printf("✅ SBI %s marked as DONE (turn %d review: SUCCEEDED)\n", sbiID, turn)

		case "NEEDS_CHANGES", "FAILED":
			// REVIEWING → IMPLEMENTING (needs another turn)
			nextStatus = model.StatusImplementing
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

	default:
		return fmt.Errorf("unsupported step: %s", step)
	}

	// 7. Save SBI to database
	if err := uc.sbiRepo.Save(ctx, sbi); err != nil {
		return fmt.Errorf("failed to save SBI: %w", err)
	}

	// 8. Record execution log
	execLog := &repository.SBIExecLog{
		SBIID:      sbiID,
		Turn:       turn,
		Step:       strings.ToUpper(step),
		Decision:   nil, // Will be set below for review step
		ReportPath: reportPath,
		ExecutedAt: time.Now(),
	}

	if step == "review" && decision != "" {
		execLog.Decision = &decision
	}

	if err := uc.execLogRepo.Save(ctx, execLog); err != nil {
		// Log warning but don't fail - exec log is for auditing
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to save exec log\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Step: %s\n", sbiID, turn, step)
	}

	// 9. Write journal entry for audit trail
	journalRecord := &repository.JournalRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		SBIID:     sbiID,
		Turn:      turn,
		Step:      fmt.Sprintf("report_%s", step),
		Status:    string(nextStatus),
		Attempt:   execState.CurrentAttempt.Value(),
		Decision:  decision,
		ElapsedMs: 0, // Command execution, not agent execution
		Error:     "",
		Artifacts: []interface{}{filename},
	}

	if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
		// Log warning but don't fail - journal is for auditing
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Step: %s\n", sbiID, turn, step)
	}

	// 10. Log report submission with version info
	version := buildinfo.GetVersion()
	currentTime := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stderr, "[report] SBI=%s, Step=%s, Decision=%s, Turn=%d, Time=%s, Version=%s, Transition=%s→%s\n",
		sbiID, step, decision, turn, currentTime, version, previousStatus, nextStatus)

	fmt.Printf("✅ Report submitted: %s (SBI: %s, Turn: %d, Step: %s)\n",
		reportPath, sbiID, turn, step)

	return nil
}
