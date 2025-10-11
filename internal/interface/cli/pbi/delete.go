package pbi

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/migration"
	"github.com/spf13/cobra"
)

// NewDeleteCommand creates a new delete command
func NewDeleteCommand() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete PBI_ID",
		Short: "Delete a PBI",
		Long: `Delete a Product Backlog Item (PBI).
This removes both the database record and the Markdown directory.
By default, asks for confirmation before deleting.`,
		Example: `  # Delete with confirmation
  deespec pbi delete PBI-001

  # Delete without confirmation
  deespec pbi delete PBI-001 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runDelete(pbiID, force)
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")

	return cmd
}

func runDelete(pbiID string, force bool) error {
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

	// Check if PBI exists
	exists, err := repo.Exists(pbiID)
	if err != nil {
		return fmt.Errorf("failed to check PBI existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("PBI not found: %s", pbiID)
	}

	// Load PBI details for confirmation message
	p, err := repo.FindByID(pbiID)
	if err != nil {
		return fmt.Errorf("failed to load PBI: %w", err)
	}

	// Confirmation prompt (unless --force)
	if !force {
		fmt.Printf("⚠️  Delete PBI: %s\n", pbiID)
		fmt.Printf("    Title: %s\n", p.Title)
		fmt.Printf("    This will remove both database record and Markdown files.\n\n")
		fmt.Print("Are you sure? (y/N): ")

		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("❌ Cancelled")
			return nil
		}
	}

	// Delete PBI
	if err := repo.Delete(pbiID); err != nil {
		return fmt.Errorf("failed to delete PBI: %w", err)
	}

	fmt.Printf("✅ PBI deleted: %s\n", pbiID)

	return nil
}
