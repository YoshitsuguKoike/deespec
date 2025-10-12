package label

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
	"github.com/spf13/cobra"
)

// newLabelCmd creates the label command group
// NewCommand creates the label command
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "label",
		Short: "Manage labels for task specifications",
		Long: `Manage labels for task specifications (SBI/PBI/EPIC) to categorize and enhance AI context.

Labels are stored in SQLite and associated with template files for instruction management.
Templates provide additional context to AI prompts, enabling more specialized implementations.`,
		Example: `  # Register a new label with template
  deespec label register security --template security/guidelines.md --description "Security best practices"

  # List all labels
  deespec label list

  # Show label details
  deespec label show security

  # Import labels from directory
  deespec label import .claude --recursive

  # Validate label integrity
  deespec label validate --sync`,
	}

	// Add subcommands
	cmd.AddCommand(newLabelRegisterCmd())
	cmd.AddCommand(newLabelListCmd())
	cmd.AddCommand(newLabelShowCmd())
	cmd.AddCommand(newLabelUpdateCmd())
	cmd.AddCommand(newLabelDeleteCmd())
	cmd.AddCommand(newLabelTemplatesCmd())
	cmd.AddCommand(newLabelImportCmd())
	cmd.AddCommand(newLabelValidateCmd())

	return cmd
}

// newLabelRegisterCmd creates the label register command
func newLabelRegisterCmd() *cobra.Command {
	var description string
	var templates []string
	var priority int
	var color string

	cmd := &cobra.Command{
		Use:   "register <name>",
		Short: "Register a new label",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("requires a label name")
			}
			if len(args) > 1 {
				// Likely caused by shell glob expansion like '--template *.md'
				return fmt.Errorf(`too many arguments: received %d arguments (%v)

Did you use wildcards like '--template *.md'?
This syntax is not supported because the shell expands '*.md' into multiple files,
which become extra positional arguments.

Use one of these methods instead:

  1. Specify each file with separate --template flags:
     deespec label register %s --template file1.md --template file2.md --template file3.md

  2. Use printf with xargs for many files:
     printf -- '--template %%s\n' *.md | xargs deespec label register %s --description "..."

  3. Use 'label import' for bulk registration (creates one label per file):
     deespec label import .claude --recursive`, len(args), args, args[0], args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Initialize container
			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Check if label already exists
			existing, _ := labelRepo.FindByName(ctx, name)
			if existing != nil {
				return fmt.Errorf("label '%s' already exists (ID: %d)", name, existing.ID())
			}

			// Process template paths - check for external files and copy if needed
			processedTemplates := make([]string, 0, len(templates))
			var copyExternalFiles *bool // nil = not yet asked, true/false = user choice

			for _, templatePath := range templates {
				// Check if file is outside project
				isExternal, err := isOutsideProject(templatePath)
				if err != nil {
					fmt.Printf("⚠ Warning: failed to check if file is external: %v\n", err)
					isExternal = false
				}

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
						copiedPath, err := copyExternalFile(templatePath)
						if err != nil {
							return fmt.Errorf("failed to copy external file %s: %w", templatePath, err)
						}
						fmt.Printf("  📁 Copied to: %s\n", copiedPath)
						processedTemplates = append(processedTemplates, copiedPath)
					} else {
						// Use original path
						processedTemplates = append(processedTemplates, templatePath)
					}
				} else {
					// Use original path for project files
					processedTemplates = append(processedTemplates, templatePath)
				}
			}

			// Create new label
			lbl := label.NewLabel(name, description, processedTemplates, priority)
			if color != "" {
				lbl.SetColor(color)
			}

			// Save to repository
			if err := labelRepo.Save(ctx, lbl); err != nil {
				return fmt.Errorf("failed to save label: %w", err)
			}

			fmt.Printf("✓ Label registered: %s (ID: %d)\n", name, lbl.ID())
			if len(processedTemplates) > 0 {
				fmt.Printf("  Templates: %v\n", processedTemplates)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "Label description")
	cmd.Flags().StringSliceVarP(&templates, "template", "t", []string{}, "Template file paths (can be specified multiple times)")
	cmd.Flags().IntVarP(&priority, "priority", "p", 0, "Merge priority (higher value = higher priority)")
	cmd.Flags().StringVarP(&color, "color", "c", "", "UI display color (e.g., #FF5733)")

	return cmd
}

// newLabelListCmd creates the label list command
func newLabelListCmd() *cobra.Command {
	var showInactive bool
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all labels",
		RunE: func(cmd *cobra.Command, args []string) error {
			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			var labels []*label.Label
			if showInactive {
				labels, err = labelRepo.FindAll(ctx)
			} else {
				labels, err = labelRepo.FindActive(ctx)
			}
			if err != nil {
				return fmt.Errorf("failed to list labels: %w", err)
			}

			if jsonOutput {
				// TODO: JSON output
				return fmt.Errorf("JSON output not yet implemented")
			}

			// Table output
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION\tTEMPLATES\tPRIORITY\tACTIVE")
			fmt.Fprintln(w, "--\t----\t-----------\t---------\t--------\t------")

			for _, lbl := range labels {
				active := "✓"
				if !lbl.IsActive() {
					active = "✗"
				}
				templateCount := len(lbl.TemplatePaths())
				desc := lbl.Description()
				if len(desc) > 40 {
					desc = desc[:37] + "..."
				}

				fmt.Fprintf(w, "%d\t%s\t%s\t%d files\t%d\t%s\n",
					lbl.ID(), lbl.Name(), desc, templateCount, lbl.Priority(), active)
			}

			w.Flush()
			fmt.Printf("\nTotal: %d labels\n", len(labels))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&showInactive, "all", "a", false, "Show inactive labels")
	cmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "Output as JSON")

	return cmd
}

// newLabelShowCmd creates the label show command
func newLabelShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name-or-id>",
		Short: "Show label details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameOrID := args[0]

			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Try to parse as ID first
			var lbl *label.Label
			if id, err := strconv.Atoi(nameOrID); err == nil {
				lbl, err = labelRepo.FindByID(ctx, id)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			} else {
				lbl, err = labelRepo.FindByName(ctx, nameOrID)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			}

			// Display label details
			fmt.Printf("Label: %s (ID: %d)\n", lbl.Name(), lbl.ID())
			fmt.Printf("Description: %s\n", lbl.Description())
			fmt.Printf("Priority: %d\n", lbl.Priority())
			fmt.Printf("Active: %v\n", lbl.IsActive())
			fmt.Printf("Line Count: %d\n", lbl.LineCount())
			fmt.Printf("Last Synced: %s\n", lbl.LastSyncedAt().Format("2006-01-02 15:04:05"))

			if len(lbl.TemplatePaths()) > 0 {
				fmt.Printf("\nTemplates:\n")
				for _, path := range lbl.TemplatePaths() {
					hash, exists := lbl.GetContentHash(path)
					if exists {
						fmt.Printf("  - %s (hash: %s...)\n", path, hash[:8])
					} else {
						fmt.Printf("  - %s (no hash)\n", path)
					}
				}
			}

			if lbl.ParentLabelID() != nil {
				fmt.Printf("\nParent Label ID: %d\n", *lbl.ParentLabelID())
			}

			return nil
		},
	}
}

// newLabelUpdateCmd creates the label update command
func newLabelUpdateCmd() *cobra.Command {
	var description string
	var priority int
	var activate, deactivate bool

	cmd := &cobra.Command{
		Use:   "update <name-or-id>",
		Short: "Update label properties",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameOrID := args[0]

			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Find label
			var lbl *label.Label
			if id, err := strconv.Atoi(nameOrID); err == nil {
				lbl, err = labelRepo.FindByID(ctx, id)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			} else {
				lbl, err = labelRepo.FindByName(ctx, nameOrID)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			}

			// Update properties
			updated := false
			if cmd.Flags().Changed("description") {
				lbl.SetDescription(description)
				updated = true
			}
			if cmd.Flags().Changed("priority") {
				lbl.SetPriority(priority)
				updated = true
			}
			if activate {
				lbl.Activate()
				updated = true
			}
			if deactivate {
				lbl.Deactivate()
				updated = true
			}

			if !updated {
				return fmt.Errorf("no updates specified")
			}

			// Save changes
			if err := labelRepo.Update(ctx, lbl); err != nil {
				return fmt.Errorf("failed to update label: %w", err)
			}

			fmt.Printf("✓ Label updated: %s (ID: %d)\n", lbl.Name(), lbl.ID())
			return nil
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "Update description")
	cmd.Flags().IntVarP(&priority, "priority", "p", 0, "Update priority")
	cmd.Flags().BoolVar(&activate, "activate", false, "Activate label")
	cmd.Flags().BoolVar(&deactivate, "deactivate", false, "Deactivate label")

	return cmd
}

// newLabelDeleteCmd creates the label delete command
func newLabelDeleteCmd() *cobra.Command {
	var force bool
	var deleteAll bool

	cmd := &cobra.Command{
		Use:   "delete <name-or-id>",
		Short: "Delete a label or all labels",
		Args: func(cmd *cobra.Command, args []string) error {
			if deleteAll && len(args) > 0 {
				return fmt.Errorf("cannot specify both --all and a label name/ID")
			}
			if !deleteAll && len(args) != 1 {
				return fmt.Errorf("requires a label name or ID (or use --all to delete all labels)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Handle --all flag
			if deleteAll {
				// Get all labels
				labels, err := labelRepo.FindAll(ctx)
				if err != nil {
					return fmt.Errorf("failed to list labels: %w", err)
				}

				if len(labels) == 0 {
					fmt.Println("No labels to delete")
					return nil
				}

				// Confirm deletion
				if !force {
					fmt.Printf("Are you sure you want to delete ALL %d labels? [y/N]: ", len(labels))
					var response string
					fmt.Scanln(&response)
					if response != "y" && response != "Y" {
						fmt.Println("Deletion cancelled")
						return nil
					}
				}

				// Collect all template paths before deletion
				allTemplatePaths := make([]string, 0)
				for _, lbl := range labels {
					allTemplatePaths = append(allTemplatePaths, lbl.TemplatePaths()...)
				}

				// Delete all labels
				deleted := 0
				failed := 0
				for _, lbl := range labels {
					if err := labelRepo.Delete(ctx, lbl.ID()); err != nil {
						fmt.Printf("⚠ Failed to delete label '%s' (ID: %d): %v\n", lbl.Name(), lbl.ID(), err)
						failed++
					} else {
						deleted++
					}
				}

				// Clean up all internal copied files (since all labels are deleted, we can clean up the entire directory)
				projectRoot, err := os.Getwd()
				if err == nil {
					labelsDir := filepath.Join(projectRoot, ".deespec", "labels")
					if _, err := os.Stat(labelsDir); err == nil {
						// Remove entire labels directory
						if err := os.RemoveAll(labelsDir); err != nil {
							fmt.Printf("⚠ Warning: failed to clean up labels directory: %v\n", err)
						}
					}
				}

				fmt.Printf("\n✓ Deleted %d labels", deleted)
				if failed > 0 {
					fmt.Printf(" (%d failed)", failed)
				}
				fmt.Println()
				return nil
			}

			// Handle single label deletion
			nameOrID := args[0]

			// Find label
			var lbl *label.Label
			if id, err := strconv.Atoi(nameOrID); err == nil {
				lbl, err = labelRepo.FindByID(ctx, id)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			} else {
				lbl, err = labelRepo.FindByName(ctx, nameOrID)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			}

			// Confirm deletion
			if !force {
				fmt.Printf("Are you sure you want to delete label '%s' (ID: %d)? [y/N]: ", lbl.Name(), lbl.ID())
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					fmt.Println("Deletion cancelled")
					return nil
				}
			}

			// Store template paths before deletion for cleanup
			templatePaths := lbl.TemplatePaths()

			// Delete label
			if err := labelRepo.Delete(ctx, lbl.ID()); err != nil {
				return fmt.Errorf("failed to delete label: %w", err)
			}

			// Clean up internal copied files
			deletedFiles := 0
			notFoundFiles := 0
			for _, templatePath := range templatePaths {
				result := deleteInternalCopiedFiles(templatePath, labelRepo)
				switch result.Status {
				case CleanupDeleted:
					deletedFiles++
				case CleanupNotFound:
					notFoundFiles++
					fmt.Printf("  ℹ File already deleted: %s\n", templatePath)
				case CleanupError:
					fmt.Printf("  ⚠ Warning: failed to clean up file %s: %v\n", templatePath, result.Error)
				case CleanupStillInUse:
					// Silent - file is still in use by other labels
				case CleanupSkipped:
					// Silent - not an internal copied file
				}
			}

			fmt.Printf("✓ Label deleted: %s (ID: %d)\n", lbl.Name(), lbl.ID())
			if deletedFiles > 0 || notFoundFiles > 0 {
				fmt.Printf("  Cleanup: %d files deleted", deletedFiles)
				if notFoundFiles > 0 {
					fmt.Printf(", %d already deleted", notFoundFiles)
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")
	cmd.Flags().BoolVar(&deleteAll, "all", false, "Delete all labels")

	return cmd
}

// newLabelTemplatesCmd creates the label templates command
func newLabelTemplatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "templates <name-or-id>",
		Short: "Show label template files with preview",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nameOrID := args[0]

			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Find label
			var lbl *label.Label
			if id, err := strconv.Atoi(nameOrID); err == nil {
				lbl, err = labelRepo.FindByID(ctx, id)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			} else {
				lbl, err = labelRepo.FindByName(ctx, nameOrID)
				if err != nil {
					return fmt.Errorf("label not found: %s", nameOrID)
				}
			}

			// Display label header
			fmt.Printf("Templates for label '%s' (ID: %d):\n", lbl.Name(), lbl.ID())
			if len(lbl.TemplatePaths()) == 0 {
				fmt.Println("  (no templates)")
				return nil
			}

			// Display template info
			for i, path := range lbl.TemplatePaths() {
				hash, exists := lbl.GetContentHash(path)
				if exists {
					fmt.Printf("  %d. %s\n", i+1, path)
					fmt.Printf("     Hash: %s\n", hash)
				} else {
					fmt.Printf("  %d. %s (no hash)\n", i+1, path)
				}
			}

			// For single template, show preview and offer full view
			if len(lbl.TemplatePaths()) == 1 {
				templatePath := lbl.TemplatePaths()[0]

				// Display preview
				totalLines, err := displayTemplatePreview(templatePath, 20)
				if err != nil {
					fmt.Printf("\n⚠ Warning: failed to display preview: %v\n", err)
					return nil
				}

				// Show remaining lines info
				if totalLines > 20 {
					fmt.Printf("\n--- %d more lines (total: %d lines) ---\n", totalLines-20, totalLines)
				} else {
					fmt.Printf("\n--- End of file (total: %d lines) ---\n", totalLines)
				}

				// Prompt for full view (only if terminal and more lines exist)
				if totalLines > 20 {
					if promptViewFullContent() {
						if err := viewFileWithPager(templatePath); err != nil {
							fmt.Printf("⚠ Warning: failed to open pager: %v\n", err)
						}
					}
				}
			} else {
				// Multiple templates - just show list
				fmt.Printf("\nUse 'deespec label templates <id>' with a single-template label to view content preview.\n")
			}

			return nil
		},
	}
}
