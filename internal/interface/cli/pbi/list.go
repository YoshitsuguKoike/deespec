package pbi

import (
	"database/sql"
	"fmt"
	"os"

	pbidomain "github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/migration"
	"github.com/spf13/cobra"
)

// NewListCommand creates a new list command
func NewListCommand() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all PBIs",
		Long:  "Display a list of all Product Backlog Items",
		Example: `  # List all PBIs
  deespec pbi list

  # List PBIs by status
  deespec pbi list --status pending
  deespec pbi list --status in_progress
  deespec pbi list --status done`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(status)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status (pending|planning|planed|in_progress|done)")

	return cmd
}

func runList(statusFilter string) error {
	// Open database
	db, err := sql.Open("sqlite3", ".deespec/var/deespec.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run migrations first
	migrator := migration.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repository
	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	repo := persistence.NewPBISQLiteRepository(db, rootPath)

	// Find PBIs
	var pbis []*pbidomain.PBI
	if statusFilter != "" {
		pbis, err = repo.FindByStatus(pbidomain.Status(statusFilter))
		if err != nil {
			return fmt.Errorf("failed to find PBIs by status: %w", err)
		}
	} else {
		pbis, err = repo.FindAll()
		if err != nil {
			return fmt.Errorf("failed to find all PBIs: %w", err)
		}
	}

	// Display header
	if statusFilter != "" {
		fmt.Printf("PBI一覧（status=%s, %d件）\n", statusFilter, len(pbis))
	} else {
		fmt.Printf("PBI一覧（全%d件）\n", len(pbis))
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	if len(pbis) == 0 {
		fmt.Println("No PBIs found.")
		return nil
	}

	// Display table header
	fmt.Printf("%-10s %-15s %-4s %-8s %s\n", "ID", "STATUS", "SP", "PRIORITY", "TITLE")
	fmt.Println("─────────────────────────────────────────────────────────────")

	// Display PBIs
	for _, p := range pbis {
		sp := "-"
		if p.EstimatedStoryPoints > 0 {
			sp = fmt.Sprintf("%d", p.EstimatedStoryPoints)
		}

		fmt.Printf("%-10s %-15s %-4s %-8s %s\n",
			p.ID,
			p.Status,
			sp,
			p.Priority.String(),
			truncateString(p.Title, 40),
		)
	}

	fmt.Println()
	fmt.Println("Use 'deespec pbi show <id>' for details.")

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
