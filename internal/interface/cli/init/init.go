package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/embed"
	"github.com/YoshitsuguKoike/deespec/internal/infra/config"
	"github.com/spf13/cobra"
)

// NewCommand creates the init command
func NewCommand() *cobra.Command {
	var (
		dir   string
		force bool
		home  string
	)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a workflow project with .deespec v0.1.14 structure",
		Long: `Initialize a new deespec project with the standard directory structure.
All files will be created under the .deespec/ directory.`,
		RunE: func(c *cobra.Command, _ []string) error {
			if dir == "" {
				dir = "."
			}

			// Determine the deespec home directory
			deespecHome := home
			if deespecHome == "" {
				deespecHome = ".deespec"
			}
			deespecDir := filepath.Join(dir, deespecHome)

			// Get all templates
			templates, err := embed.GetTemplates()
			if err != nil {
				return fmt.Errorf("failed to load templates: %w", err)
			}

			// Create necessary directories
			requiredDirs := []string{
				filepath.Join(deespecDir, "etc", "policies"),
				filepath.Join(deespecDir, "prompts", "system"),
				filepath.Join(deespecDir, "specs", "sbi"),
				filepath.Join(deespecDir, "specs", "pbi"),
				filepath.Join(deespecDir, "templates"),
				filepath.Join(deespecDir, "sessions"),
				filepath.Join(deespecDir, "knowledge"),
			}

			for _, d := range requiredDirs {
				if err := os.MkdirAll(d, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", d, err)
				}
			}

			// Write all template files
			for _, tmpl := range templates {
				result, err := embed.WriteTemplate(deespecDir, tmpl, force)
				if err != nil {
					return fmt.Errorf("failed to write %s: %w", tmpl.Path, err)
				}
				if result.Action == "SKIP" {
					fmt.Printf("SKIP: %s (exists; use --force to overwrite)\n", filepath.Join(deespecDir, result.Path))
				} else {
					fmt.Printf("%s: %s\n", result.Action, filepath.Join(deespecDir, result.Path))
				}
			}

			// Create setting.json with default configuration (only if not exists or force)
			settingPath := filepath.Join(deespecDir, "setting.json")
			settingExists := fileExists(settingPath)
			if force || !settingExists {
				settingContent := config.CreateDefaultSettings()
				if err := writeFileAtomic(settingPath, settingContent, 0644); err != nil {
					return fmt.Errorf("failed to write setting.json: %w", err)
				}
				if force && settingExists {
					fmt.Printf("WROTE (force): %s\n", settingPath)
				} else {
					fmt.Printf("WROTE: %s\n", settingPath)
				}
			} else {
				fmt.Printf("SKIP: %s (exists; use --force to overwrite)\n", settingPath)
			}

			// Create health.json with initial state (only if not exists or force)
			healthPath := filepath.Join(deespecDir, "var", "health.json")
			healthExists := fileExists(healthPath)
			if force || !healthExists {
				healthContent := fmt.Sprintf(`{"ts":"%s","turn":0,"step":"plan","ok":true,"error":""}`,
					time.Now().UTC().Format(time.RFC3339Nano))
				if err := writeFileAtomic(healthPath, []byte(healthContent), 0644); err != nil {
					return fmt.Errorf("failed to write health.json: %w", err)
				}
				if force && healthExists {
					fmt.Printf("WROTE (force): %s\n", healthPath)
				} else {
					fmt.Printf("WROTE: %s\n", healthPath)
				}
			} else {
				fmt.Printf("SKIP: %s (exists; use --force to overwrite)\n", healthPath)
			}

			// Note: journal.ndjson is NOT created during init
			// It will be created automatically during first run

			// Update .gitignore to exclude .deespec/var
			if err := updateGitignore(dir); err != nil {
				// Non-fatal error, just warn
				fmt.Printf("Warning: Could not update .gitignore: %v\n", err)
			}

			// Print success message
			fmt.Printf("Initialized .deespec v0.1.14 structure in %s:\n", deespecDir)
			fmt.Println("  ├── setting.json          # Configuration file (NEW)")
			fmt.Println("  ├── etc/")
			fmt.Println("  │   ├── workflow.yaml")
			fmt.Println("  │   └── policies/")
			fmt.Println("  │       └── review_policy.yaml")
			fmt.Println("  ├── prompts/")
			fmt.Println("  │   ├── WIP.md            # Implementation prompt")
			fmt.Println("  │   ├── REVIEW.md         # Code review prompt")
			fmt.Println("  │   ├── REVIEW_AND_WIP.md # Combined prompt")
			fmt.Println("  │   └── system/")
			fmt.Println("  │       ├── plan.md")
			fmt.Println("  │       ├── implement.md")
			fmt.Println("  │       ├── test.md")
			fmt.Println("  │       ├── review.md")
			fmt.Println("  │       └── done.md")
			fmt.Println("  ├── specs/")
			fmt.Println("  │   ├── sbi/")
			fmt.Println("  │   └── pbi/")
			fmt.Println("  └── var/")
			fmt.Println("      ├── state.json")
			fmt.Println("      ├── health.json")
			fmt.Println("      └── artifacts/")

			if !force {
				fmt.Println("\nNote: Existing files were preserved. Use --force to overwrite.")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", ".", "Target directory")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing files")
	cmd.Flags().StringVar(&home, "home", "", "Custom deespec home directory (default: .deespec)")

	return cmd
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeFileAtomic(path string, content []byte, perm os.FileMode) error {
	// Write to temp file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, content, perm); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up
		return err
	}

	return nil
}

// updateGitignore adds deespec v1 exclusion rules to .gitignore
func updateGitignore(rootDir string) error {
	gitignorePath := filepath.Join(rootDir, ".gitignore")

	// Define the deespec block
	deespecBlock := `# >>> deespec v1
/.deespec/var/
/.deespec/var/*
!/.deespec/var/.keep
# <<< deespec v1`

	// Read existing content (or start with empty)
	var existingContent []byte
	if fileExists(gitignorePath) {
		var err error
		existingContent, err = os.ReadFile(gitignorePath)
		if err != nil {
			return fmt.Errorf("failed to read .gitignore: %w", err)
		}
	}

	contentStr := string(existingContent)

	// Check if deespec block already exists
	if strings.Contains(contentStr, "# >>> deespec v1") {
		// Already present, nothing to do (idempotent)
		fmt.Printf("SKIP: .gitignore deespec block already present\n")
		return nil
	}

	// Prepare new content
	var newContent strings.Builder
	newContent.WriteString(contentStr)

	// Ensure there's a newline before our block
	if len(contentStr) > 0 && !strings.HasSuffix(contentStr, "\n") {
		newContent.WriteString("\n")
	}

	// Add an extra newline for separation if file has content
	if len(contentStr) > 0 {
		newContent.WriteString("\n")
	}

	// Add deespec block
	newContent.WriteString(deespecBlock)
	newContent.WriteString("\n")

	// Write atomically
	err := writeFileAtomic(gitignorePath, []byte(newContent.String()), 0644)
	if err == nil {
		fmt.Printf("APPENDED: .gitignore deespec v1 block\n")
	}
	return err
}
