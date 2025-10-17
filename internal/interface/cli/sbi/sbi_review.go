package sbi

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/application/usecase"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	infrarepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/spf13/cobra"

	_ "github.com/mattn/go-sqlite3"
)

// NewSBIReviewCommand creates the sbi review command
func NewSBIReviewCommand() *cobra.Command {
	var turn int
	var decision string

	cmd := &cobra.Command{
		Use:   "review <SBI_ID>",
		Short: "Report review decision for an SBI",
		Long: `Report the review decision for an SBI after completing code review.

This command is intended to be executed by AI agents after writing review reports,
providing a reliable way to communicate review decisions without file parsing.

Example:
  deespec sbi review 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --decision SUCCEEDED
  deespec sbi review 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --decision NEEDS_CHANGES
  deespec sbi review 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --decision FAILED`,
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

			return nil
		},
	}

	cmd.Flags().IntVar(&turn, "turn", 0, "Turn number of the review (required)")
	cmd.Flags().StringVar(&decision, "decision", "", "Review decision: SUCCEEDED, NEEDS_CHANGES, or FAILED (required)")
	cmd.MarkFlagRequired("turn")
	cmd.MarkFlagRequired("decision")

	return cmd
}
