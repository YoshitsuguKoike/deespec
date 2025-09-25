package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/YoshitsuguKoike/deespec/internal/embed"
)

func newInitCmd() *cobra.Command {
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
				filepath.Join(deespecDir, "var", "artifacts"),
				filepath.Join(deespecDir, "sessions"),
				filepath.Join(deespecDir, "knowledge"),
			}

			for _, d := range requiredDirs {
				if err := os.MkdirAll(d, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", d, err)
				}
			}

			// Write all template files
			var filesWritten []string
			for _, tmpl := range templates {
				if err := embed.WriteTemplate(deespecDir, tmpl, force); err != nil {
					return fmt.Errorf("failed to write %s: %w", tmpl.Path, err)
				}
				filesWritten = append(filesWritten, tmpl.Path)
			}

			// Create health.json with initial state
			healthPath := filepath.Join(deespecDir, "var", "health.json")
			if force || !fileExists(healthPath) {
				healthContent := fmt.Sprintf(`{"ts":"%s","turn":0,"step":"plan","ok":true,"error":""}`,
					time.Now().UTC().Format(time.RFC3339Nano))
				if err := writeFileAtomic(healthPath, []byte(healthContent), 0644); err != nil {
					return fmt.Errorf("failed to write health.json: %w", err)
				}
			}

			// Print success message
			fmt.Printf("Initialized .deespec v0.1.14 structure in %s:\n", deespecDir)
			fmt.Println("  ├── etc/")
			fmt.Println("  │   ├── workflow.yaml")
			fmt.Println("  │   └── policies/")
			fmt.Println("  │       └── review_policy.yaml")
			fmt.Println("  ├── prompts/")
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
