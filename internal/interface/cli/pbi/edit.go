package pbi

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/migration"
	"github.com/spf13/cobra"
)

// NewEditCommand creates a new edit command
func NewEditCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit PBI_ID",
		Short: "Edit a PBI's Markdown file",
		Long: `Opens the PBI's Markdown file in your default editor.
The editor is determined by the $EDITOR environment variable.
If $EDITOR is not set, it falls back to 'vim'.`,
		Example: `  # Edit PBI-001
  deespec pbi edit PBI-001

  # Use specific editor
  EDITOR=nano deespec pbi edit PBI-001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runEdit(pbiID)
		},
	}

	return cmd
}

func runEdit(pbiID string) error {
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

	// Get Markdown file path
	mdPath := filepath.Join(rootPath, ".deespec", "specs", "pbi", pbiID, "pbi.md")

	// Check if file exists
	if _, err := os.Stat(mdPath); os.IsNotExist(err) {
		return fmt.Errorf("Markdown file not found: %s", mdPath)
	}

	// Get editor from environment or use default
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	fmt.Printf("📝 Opening %s in %s...\n", pbiID, editor)

	// Create command to open editor
	cmd := exec.Command(editor, mdPath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run editor and wait for it to complete
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run editor: %w", err)
	}

	fmt.Printf("\n✅ Finished editing %s\n", pbiID)
	fmt.Printf("File: %s\n", mdPath)

	return nil
}
