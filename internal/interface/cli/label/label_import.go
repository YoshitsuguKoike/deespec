package label

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
	"github.com/spf13/cobra"
)

// newLabelImportCmd creates the label import command
func newLabelImportCmd() *cobra.Command {
	var recursive bool
	var prefixFromDir bool
	var dryRun bool
	var force bool

	cmd := &cobra.Command{
		Use:   "import <directory>",
		Short: "Import labels from directory",
		Long: `Import labels from a directory containing template files.

This command scans a directory for .md files and creates labels based on the files found.
Each .md file becomes a label with the file as its template.

Template directories are configured in setting.json (label_config.template_dirs).`,
		Example: `  # Import from .claude directory
  deespec label import .claude --recursive

  # Dry run to see what would be imported
  deespec label import .deespec/prompts/labels --dry-run

  # Import with directory name as prefix
  deespec label import .claude/perspectives --prefix-from-dir`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]

			// Initialize container
			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			config := common.GetGlobalConfig().LabelConfig()
			ctx := context.Background()

			// Scan directory for .md files
			var files []string
			if recursive {
				err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if !info.IsDir() && filepath.Ext(path) == ".md" {
						// Check exclusion patterns
						if shouldExclude(path, config.Import.ExcludePatterns) {
							if !dryRun {
								fmt.Printf("⊗ Skipping (excluded): %s\n", path)
							}
							return nil
						}
						files = append(files, path)
					}
					return nil
				})
			} else {
				entries, err := os.ReadDir(dir)
				if err != nil {
					return fmt.Errorf("failed to read directory: %w", err)
				}
				for _, entry := range entries {
					if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" {
						path := filepath.Join(dir, entry.Name())
						if shouldExclude(path, config.Import.ExcludePatterns) {
							if !dryRun {
								fmt.Printf("⊗ Skipping (excluded): %s\n", path)
							}
							continue
						}
						files = append(files, path)
					}
				}
			}

			if len(files) == 0 {
				fmt.Println("No .md files found")
				return nil
			}

			fmt.Printf("Found %d template files\n", len(files))
			if dryRun {
				fmt.Println("\n[DRY RUN] Would import the following labels:")
			}

			imported := 0
			skipped := 0
			var copyExternalFiles *bool // nil = not yet asked, true/false = user choice

			for _, filePath := range files {
				// Generate label name from file path
				labelName := generateLabelName(filePath, dir, prefixFromDir)

				// Validate line count
				lineCount, err := countFileLines(filePath)
				if err != nil {
					fmt.Printf("⚠ Warning: failed to count lines in %s: %v\n", filePath, err)
					continue
				}

				if lineCount > config.Import.MaxLineCount {
					fmt.Printf("⊗ Skipping %s: exceeds max line count (%d > %d)\n",
						filePath, lineCount, config.Import.MaxLineCount)
					skipped++
					continue
				}

				// Check if file is outside project and handle accordingly
				isExternal, err := isOutsideProject(filePath)
				if err != nil {
					fmt.Printf("⚠ Warning: failed to check if file is external: %v\n", err)
					isExternal = false
				}

				var templatePath string
				if isExternal {
					// Ask user only once
					if copyExternalFiles == nil {
						shouldCopy, err := promptCopyExternal()
						if err != nil {
							fmt.Printf("⚠ Warning: failed to read user input: %v\n", err)
							shouldCopy = false
						}
						copyExternalFiles = &shouldCopy
					}

					if *copyExternalFiles {
						// Copy file to .deespec/labels/
						copiedPath, err := copyExternalFile(filePath)
						if err != nil {
							fmt.Printf("⚠ Failed to copy external file %s: %v\n", filePath, err)
							skipped++
							continue
						}
						fmt.Printf("  📁 Copied to: %s\n", copiedPath)
						templatePath = copiedPath
					} else {
						// Use absolute path
						absPath, err := filepath.Abs(filePath)
						if err != nil {
							absPath = filePath
						}
						templatePath = absPath
					}
				} else {
					// Use absolute path for project files
					absPath, err := filepath.Abs(filePath)
					if err != nil {
						absPath = filePath
					}
					templatePath = absPath
				}

				if dryRun {
					fmt.Printf("  ✓ %s -> template: %s (%d lines)\n", labelName, templatePath, lineCount)
					imported++
					continue
				}

				// Check if label already exists
				existing, _ := labelRepo.FindByName(ctx, labelName)
				if existing != nil && !force {
					fmt.Printf("⊗ Skipping %s: label already exists (use --force to overwrite)\n", labelName)
					skipped++
					continue
				}

				// Create or update label
				if existing != nil {
					// Update existing label
					existing.AddTemplatePath(templatePath)
					if err := labelRepo.Update(ctx, existing); err != nil {
						fmt.Printf("⚠ Failed to update label %s: %v\n", labelName, err)
						continue
					}

					// Sync from file to calculate hash and line count
					if err := labelRepo.SyncFromFile(ctx, existing.ID()); err != nil {
						fmt.Printf("⚠ Warning: failed to sync label %s: %v\n", labelName, err)
					}

					fmt.Printf("  ↻ Updated: %s (added template: %s)\n", labelName, templatePath)
				} else {
					// Create new label with descriptive format: "filename.md imported from /original/path"
					filename := filepath.Base(filePath)
					description := fmt.Sprintf("%s imported from %s", filename, filePath)
					lbl := label.NewLabel(labelName, description, []string{templatePath}, 0)

					if err := labelRepo.Save(ctx, lbl); err != nil {
						fmt.Printf("⚠ Failed to save label %s: %v\n", labelName, err)
						continue
					}

					// Sync from file to calculate hash and line count
					if err := labelRepo.SyncFromFile(ctx, lbl.ID()); err != nil {
						fmt.Printf("⚠ Warning: failed to sync label %s: %v\n", labelName, err)
					}

					fmt.Printf("  ✓ Imported: %s (template: %s, %d lines)\n", labelName, templatePath, lineCount)
				}
				imported++
			}

			fmt.Printf("\n")
			if dryRun {
				fmt.Printf("Would import: %d labels\n", imported)
			} else {
				fmt.Printf("Summary: %d imported, %d skipped\n", imported, skipped)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&recursive, "recursive", "r", false, "Recursively scan subdirectories")
	cmd.Flags().BoolVar(&prefixFromDir, "prefix-from-dir", false, "Use directory name as label prefix")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be imported without actually importing")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing labels")

	return cmd
}

// generateLabelName generates a label name from file path
func generateLabelName(filePath, baseDir string, prefixFromDir bool) string {
	// Remove extension
	name := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	if prefixFromDir {
		// Get relative path from base directory
		relPath, err := filepath.Rel(baseDir, filePath)
		if err == nil && relPath != filepath.Base(filePath) {
			// Use directory structure as prefix
			dir := filepath.Dir(relPath)
			if dir != "." {
				// Replace path separators with colons
				prefix := strings.ReplaceAll(dir, string(filepath.Separator), ":")
				name = prefix + ":" + name
			}
		}
	}

	// Clean up the name
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")

	return name
}

// shouldExclude checks if a file should be excluded based on patterns
func shouldExclude(filePath string, patterns []string) bool {
	basename := filepath.Base(filePath)

	for _, pattern := range patterns {
		// Simple glob matching
		matched, err := filepath.Match(pattern, basename)
		if err == nil && matched {
			return true
		}

		// Check if pattern contains path separator (directory pattern)
		if strings.Contains(pattern, "/") || strings.Contains(pattern, string(filepath.Separator)) {
			matched, err := filepath.Match(pattern, filePath)
			if err == nil && matched {
				return true
			}
		}

		// Check for wildcard directory patterns like "tmp/**"
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(filePath, prefix+string(filepath.Separator)) {
				return true
			}
		}
	}

	return false
}

// countFileLines counts the number of lines in a file
func countFileLines(filePath string) (int, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	lines := strings.Count(string(content), "\n")
	// Add 1 if file doesn't end with newline
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}

	return lines, nil
}
