package usecase

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// ReportSBIUseCase handles implementation report submission from AI agents
type ReportSBIUseCase struct {
	sbiRepo     repository.SBIRepository
	journalRepo repository.JournalRepository
}

// NewReportSBIUseCase creates a new ReportSBIUseCase
func NewReportSBIUseCase(sbiRepo repository.SBIRepository, journalRepo repository.JournalRepository) *ReportSBIUseCase {
	return &ReportSBIUseCase{
		sbiRepo:     sbiRepo,
		journalRepo: journalRepo,
	}
}

// Execute processes an implementation report and saves it to the reports directory
func (uc *ReportSBIUseCase) Execute(ctx context.Context, sbiID string, turn int, reportType string, content string) error {
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
	var reportDir string
	var filename string

	switch reportType {
	case "implement":
		reportDir = filepath.Join(".deespec", "reports", "sbi", sbiID)
		filename = fmt.Sprintf("implement_%d.md", turn)
	default:
		return fmt.Errorf("unsupported report type: %s", reportType)
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

	// 6. Write journal entry for audit trail
	journalRecord := &repository.JournalRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		SBIID:     sbiID,
		Turn:      turn,
		Step:      "report_command",
		Status:    string(sbi.Status()),
		Attempt:   execState.CurrentAttempt.Value(),
		Decision:  "",
		ElapsedMs: 0,
		Error:     "",
		Artifacts: []interface{}{filename},
	}

	if err := uc.journalRepo.Append(ctx, journalRecord); err != nil {
		// Log warning but don't fail - journal is for auditing
		fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to append journal entry\n")
		fmt.Fprintf(os.Stderr, "   Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "   SBI ID: %s, Turn: %d, Type: %s\n", sbiID, turn, reportType)
	}

	// 7. Log success
	fmt.Printf("✅ Report submitted: %s (SBI: %s, Turn: %d, Type: %s)\n",
		reportPath, sbiID, turn, reportType)

	return nil
}
