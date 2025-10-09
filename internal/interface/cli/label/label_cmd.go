package label

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"fmt"
	"os"
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

  # Attach label to a task
  deespec label attach SBI-xxx security

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
	cmd.AddCommand(newLabelAttachCmd())
	cmd.AddCommand(newLabelDetachCmd())
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
		Args:  cobra.ExactArgs(1),
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

			// Create new label
			lbl := label.NewLabel(name, description, templates, priority)
			if color != "" {
				lbl.SetColor(color)
			}

			// Save to repository
			if err := labelRepo.Save(ctx, lbl); err != nil {
				return fmt.Errorf("failed to save label: %w", err)
			}

			fmt.Printf("✓ Label registered: %s (ID: %d)\n", name, lbl.ID())
			if len(templates) > 0 {
				fmt.Printf("  Templates: %v\n", templates)
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

	cmd := &cobra.Command{
		Use:   "delete <name-or-id>",
		Short: "Delete a label",
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

			// Delete label
			if err := labelRepo.Delete(ctx, lbl.ID()); err != nil {
				return fmt.Errorf("failed to delete label: %w", err)
			}

			fmt.Printf("✓ Label deleted: %s (ID: %d)\n", lbl.Name(), lbl.ID())
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	return cmd
}

// newLabelAttachCmd creates the label attach command
func newLabelAttachCmd() *cobra.Command {
	var position int

	cmd := &cobra.Command{
		Use:   "attach <task-id> <label-name>",
		Short: "Attach a label to a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			labelName := args[1]

			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Find label
			lbl, err := labelRepo.FindByName(ctx, labelName)
			if err != nil {
				return fmt.Errorf("label not found: %s", labelName)
			}

			// Attach to task
			if err := labelRepo.AttachToTask(ctx, taskID, lbl.ID(), position); err != nil {
				return fmt.Errorf("failed to attach label: %w", err)
			}

			fmt.Printf("✓ Label '%s' attached to task %s\n", labelName, taskID)
			return nil
		},
	}

	cmd.Flags().IntVarP(&position, "position", "p", 0, "Display position")

	return cmd
}

// newLabelDetachCmd creates the label detach command
func newLabelDetachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detach <task-id> <label-name>",
		Short: "Detach a label from a task",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]
			labelName := args[1]

			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Find label
			lbl, err := labelRepo.FindByName(ctx, labelName)
			if err != nil {
				return fmt.Errorf("label not found: %s", labelName)
			}

			// Detach from task
			if err := labelRepo.DetachFromTask(ctx, taskID, lbl.ID()); err != nil {
				return fmt.Errorf("failed to detach label: %w", err)
			}

			fmt.Printf("✓ Label '%s' detached from task %s\n", labelName, taskID)
			return nil
		},
	}
}

// newLabelTemplatesCmd creates the label templates command
func newLabelTemplatesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "templates <name-or-id>",
		Short: "Show label template files",
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

			// Display templates
			fmt.Printf("Templates for label '%s' (ID: %d):\n", lbl.Name(), lbl.ID())
			if len(lbl.TemplatePaths()) == 0 {
				fmt.Println("  (no templates)")
				return nil
			}

			for i, path := range lbl.TemplatePaths() {
				hash, exists := lbl.GetContentHash(path)
				if exists {
					fmt.Printf("  %d. %s\n", i+1, path)
					fmt.Printf("     Hash: %s\n", hash)
				} else {
					fmt.Printf("  %d. %s (no hash)\n", i+1, path)
				}
			}

			return nil
		},
	}
}
