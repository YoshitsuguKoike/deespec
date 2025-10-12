package pbi

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	"github.com/spf13/cobra"
)

// NewEditCommand creates a new edit command
func NewEditCommand() *cobra.Command {
	var title string

	cmd := &cobra.Command{
		Use:   "edit PBI_ID",
		Short: "Edit a PBI's Markdown file",
		Long: `Opens the PBI's Markdown file in your default editor.
The editor is determined by the $EDITOR environment variable.
If $EDITOR is not set, it falls back to 'vim'.

If --title is specified, the title is updated in both the database
and the Markdown file before opening the editor.`,
		Example: `  # Edit PBI-001
  deespec pbi edit PBI-001

  # Update title and then edit
  deespec pbi edit PBI-001 -t "New Title"

  # Use specific editor
  EDITOR=nano deespec pbi edit PBI-001`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runEdit(pbiID, title)
		},
	}

	cmd.Flags().StringVarP(&title, "title", "t", "", "Update title before editing")

	return cmd
}

func runEdit(pbiID string, newTitle string) error {
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

	// Update title if specified
	if newTitle != "" {
		// Update database
		_, err := db.Exec("UPDATE pbis SET title = ? WHERE id = ?", newTitle, pbiID)
		if err != nil {
			return fmt.Errorf("failed to update title in database: %w", err)
		}

		// Update Markdown file - replace H1
		content, err := os.ReadFile(mdPath)
		if err != nil {
			return fmt.Errorf("failed to read markdown file: %w", err)
		}

		// Find and replace first H1
		lines := splitLines(string(content))
		updated := false
		for i, line := range lines {
			if len(line) > 0 && line[0] == '#' {
				// Found H1, replace it
				lines[i] = "# " + newTitle
				updated = true
				break
			}
		}

		// If no H1 found, prepend one
		if !updated {
			lines = append([]string{"# " + newTitle, ""}, lines...)
		}

		// Join lines back
		newContent := ""
		for i, line := range lines {
			if i > 0 {
				newContent += "\n"
			}
			newContent += line
		}

		// Write back
		if err := os.WriteFile(mdPath, []byte(newContent), 0644); err != nil {
			return fmt.Errorf("failed to write markdown file: %w", err)
		}

		fmt.Printf("✅ Title updated: %s\n", newTitle)
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
