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
	var reportType string
	var useStdin bool

	cmd := &cobra.Command{
		Use:   "report <SBI_ID>",
		Short: "Submit implementation report for an SBI",
		Long: `Submit the implementation report for an SBI after completing implementation work.

This command is intended to be executed by AI agents after implementation,
providing a reliable way to submit reports without creating report files.

The report content should be provided via stdin using a heredoc:

Example:
  deespec sbi report 01K7P4N123EQAB57FA5E5ZG6A3 --turn 3 --type implement --stdin <<'EOF'
  ## Turn 3 Implementation Report

  [Report content in Japanese]

  - Status: Completed
  - Files Modified: 5
  - Key Achievement: Implemented authentication middleware
  EOF`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			// Validate report type
			reportType = strings.ToLower(strings.TrimSpace(reportType))
			validTypes := map[string]bool{
				"implement": true,
			}
			if !validTypes[reportType] {
				return fmt.Errorf("invalid report type: %s (must be implement)", reportType)
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

			// Create use case
			reportUseCase := usecase.NewReportSBIUseCase(sbiRepo, journalRepo)

			// Execute report submission
			ctx := context.Background()
			if err := reportUseCase.Execute(ctx, sbiID, turn, reportType, content); err != nil {
				return fmt.Errorf("failed to submit report: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&turn, "turn", 0, "Turn number of the implementation (required)")
	cmd.Flags().StringVar(&reportType, "type", "", "Report type: implement (required)")
	cmd.Flags().BoolVar(&useStdin, "stdin", false, "Read report content from stdin (required)")
	cmd.MarkFlagRequired("turn")
	cmd.MarkFlagRequired("type")
	cmd.MarkFlagRequired("stdin")

	return cmd
}
