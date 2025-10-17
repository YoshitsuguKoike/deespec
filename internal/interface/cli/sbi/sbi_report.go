package sbi

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/application/usecase"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"

	_ "github.com/mattn/go-sqlite3"
)

// NewSBIReportCommand creates the sbi report command
func NewSBIReportCommand() *cobra.Command {
	var turn int
	var step string
	var decision string
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "report <SBI_ID>",
		Short: "Submit implementation or review report for an SBI",
		Long: `Submit the implementation or review report for an SBI after completing work.

This command is intended to be executed by AI agents after implementation or review,
providing a reliable way to submit reports and update SBI status.

The report content should be provided via stdin using a heredoc:

Example for implementation:
  deespec sbi report 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --step implement --stdin <<'EOF'
  ## Turn 3 Implementation Report

  [Implementation details in Japanese]

  - Files Modified: auth_middleware.go, routes.go
  - Key Achievement: Implemented JWT authentication
  EOF

Example for review:
  deespec sbi report 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --step review --decision SUCCEEDED --stdin <<'EOF'
  ## Turn 3 Review Report
  DECISION: SUCCEEDED

  [Review details in Japanese]

  - All tests passing
  - Code quality: Good
  EOF`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			// Validate step
			step = strings.ToLower(strings.TrimSpace(step))
			validSteps := map[string]bool{
				"implement": true,
				"review":    true,
			}
			if !validSteps[step] {
				return fmt.Errorf("invalid step: %s (must be implement or review)", step)
			}

			// Validate decision for review step
			if step == "review" {
				decision = strings.ToUpper(strings.TrimSpace(decision))
				validDecisions := map[string]bool{
					"SUCCEEDED":     true,
					"NEEDS_CHANGES": true,
					"FAILED":        true,
				}
				if !validDecisions[decision] {
					return fmt.Errorf("invalid decision: %s (must be SUCCEEDED, NEEDS_CHANGES, or FAILED)", decision)
				}
			}

			// Validate turn
			if turn <= 0 {
				return fmt.Errorf("invalid turn: %d (must be positive integer)", turn)
			}

			// Read report content from stdin
			if !useStdin {
				return fmt.Errorf("--stdin flag is required (report content must be provided via stdin)")
			}

			var reportContent strings.Builder
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				reportContent.WriteString(scanner.Text())
				reportContent.WriteString("\n")
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read stdin: %w", err)
			}

			content := strings.TrimSpace(reportContent.String())
			if content == "" {
				return fmt.Errorf("report content is empty")
			}

			// Initialize repository
			db, err := sql.Open("sqlite3", ".deespec/deespec.db")
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer db.Close()

			// Run migrations
			migrator := sqlite.NewMigrator(db)
			if err := migrator.Migrate(); err != nil {
				return fmt.Errorf("failed to run migrations: %w", err)
			}

			sbiRepo := sqlite.NewSBIRepository(db)
			journalRepo := infrarepo.NewJournalRepositoryImpl(".deespec/journal.ndjson")
			execLogRepo := sqlite.NewSBIExecLogRepository(db)

			// Create use case
			reportUseCase := usecase.NewReportSBIUseCase(sbiRepo, journalRepo, execLogRepo)

			// Execute report submission
			ctx := context.Background()
			if err := reportUseCase.Execute(ctx, sbiID, turn, step, decision, content); err != nil {
				return fmt.Errorf("failed to submit report: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&turn, "turn", 0, "Turn number (required)")
	cmd.Flags().StringVar(&step, "step", "", "Step type: implement or review (required)")
	cmd.Flags().StringVar(&decision, "decision", "", "Review decision: SUCCEEDED, NEEDS_CHANGES, or FAILED (required for review step)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read report content from stdin (required)")
	cmd.MarkFlagRequired("turn")
	cmd.MarkFlagRequired("step")
	cmd.MarkFlagRequired("stdin")

	return cmd
}
