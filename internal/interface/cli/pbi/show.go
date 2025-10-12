package pbi

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	"github.com/spf13/cobra"
)

// NewShowCommand creates a new show command
func NewShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show PBI details",
		Long:  "Display detailed information about a specific PBI",
		Example: `  # Show PBI details
  deespec pbi show PBI-001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runShow(pbiID)
		},
	}

	return cmd
}

func runShow(pbiID string) error {
	// Open database
	db, err := sql.Open("sqlite3", ".deespec/deespec.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run migrations first
	migrator := sqlite.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repository
	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	repo := persistence.NewPBISQLiteRepository(db, rootPath)

	// Find PBI
	p, err := repo.FindByID(pbiID)
	if err != nil {
		return fmt.Errorf("failed to find PBI: %w", err)
	}

	// Get body
	body, err := repo.GetBody(pbiID)
	if err != nil {
		return fmt.Errorf("failed to get PBI body: %w", err)
	}

	// Display PBI details
	fmt.Printf("📦 %s: %s\n", p.ID, p.Title)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("📊 Status: %s\n", p.Status)
	fmt.Printf("🔢 Story Points: %d\n", p.EstimatedStoryPoints)
	fmt.Printf("⭐ Priority: %s (%d)\n", p.Priority.String(), p.Priority)
	if p.ParentEpicID != "" {
		fmt.Printf("📂 Parent EPIC: %s\n", p.ParentEpicID)
	}
	fmt.Println()
	fmt.Printf("📄 Markdown File: %s\n", p.GetMarkdownPath())
	fmt.Println()
	fmt.Printf("🕐 Created: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("🕐 Updated: %s\n", p.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("📋 Body:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(body)

	return nil
}
