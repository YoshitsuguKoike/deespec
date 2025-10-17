package sbi

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/usecase"
	"github.com/YoshitsuguKoike/deespec/internal/buildinfo"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"

	_ "github.com/mattn/go-sqlite3"
)

// NewSBIReviewCommand creates the sbi review command
func NewSBIReviewCommand() *cobra.Command {
	var turn int
	var decision string
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "review <SBI_ID>",
		Short: "Report review decision for an SBI",
		Long: `Report the review decision for an SBI after completing code review.

This command is intended to be executed by AI agents after code review,
providing a reliable way to communicate review decisions and submit reports.

The review report content should be provided via stdin using a heredoc:

Example with stdin:
  deespec sbi review 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --decision SUCCEEDED --stdin <<'EOF'
  ## Summary
  DECISION: SUCCEEDED

  [Review content in Japanese]

  ## Review Details
  [Details...]
  EOF

Legacy example (without report):
  deespec sbi review 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --decision SUCCEEDED`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			// Validate decision
			decision = strings.ToUpper(strings.TrimSpace(decision))
			validDecisions := map[string]bool{
				"SUCCEEDED":     true,
				"NEEDS_CHANGES": true,
				"FAILED":        true,
			}
			if !validDecisions[decision] {
				return fmt.Errorf("invalid decision: %s (must be SUCCEEDED, NEEDS_CHANGES, or FAILED)", decision)
			}

			// Validate turn
			if turn <= 0 {
				return fmt.Errorf("invalid turn: %d (must be positive integer)", turn)
			}

			// Read review content from stdin if --stdin flag is provided
			var reviewContent string
			if useStdin {
				var contentBuilder strings.Builder
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					contentBuilder.WriteString(scanner.Text())
					contentBuilder.WriteString("\n")
				}
				if err := scanner.Err(); err != nil {
					return fmt.Errorf("failed to read stdin: %w", err)
				}
				reviewContent = strings.TrimSpace(contentBuilder.String())
				if reviewContent == "" {
					return fmt.Errorf("review content from stdin is empty")
				}
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

			// Create use case
			reviewUseCase := usecase.NewReviewSBIUseCase(sbiRepo, journalRepo)

			// Execute review
			ctx := context.Background()
			if err := reviewUseCase.Execute(ctx, sbiID, turn, decision); err != nil {
				return fmt.Errorf("failed to execute review: %w", err)
			}

			// Log review execution
			currentTime := time.Now().Format("2006-01-02 15:04:05")
			version := buildinfo.GetVersion()
			fmt.Fprintf(os.Stderr, "[review] SBI=%s, DECISION=%s, Turn=%d, Time=%s, Version=%s\n",
				sbiID, decision, turn, currentTime, version)

			// If stdin content is provided, save review report to file
			if useStdin && reviewContent != "" {
				reportDir := filepath.Join(".deespec", "reports", "sbi", sbiID)
				if err := os.MkdirAll(reportDir, 0755); err != nil {
					return fmt.Errorf("failed to create report directory: %w", err)
				}

				reportPath := filepath.Join(reportDir, fmt.Sprintf("review_%d.md", turn))
				if err := os.WriteFile(reportPath, []byte(reviewContent), 0644); err != nil {
					return fmt.Errorf("failed to write review report: %w", err)
				}

				fmt.Printf("✅ Review report saved: %s\n", reportPath)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&turn, "turn", 0, "Turn number of the review (required)")
	cmd.Flags().StringVar(&decision, "decision", "", "Review decision: SUCCEEDED, NEEDS_CHANGES, or FAILED (required)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read review report content from stdin (optional)")
	cmd.MarkFlagRequired("turn")
	cmd.MarkFlagRequired("decision")

	return cmd
}
