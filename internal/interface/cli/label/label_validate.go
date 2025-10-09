package label

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"fmt"
	"strconv"
	"text/tabwriter"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/spf13/cobra"
)

// newLabelValidateCmd creates the label validate command
func newLabelValidateCmd() *cobra.Command {
	var sync bool
	var showDetails bool

	cmd := &cobra.Command{
		Use:   "validate [name-or-id]",
		Short: "Validate label template integrity",
		Long: `Validate the integrity of label template files by comparing stored hashes with current file contents.

This command checks if template files have been modified, deleted, or are missing.
Use --sync to automatically update hashes for modified files.`,
		Example: `  # Validate all labels
  deespec label validate

  # Validate specific label
  deespec label validate security

  # Validate and sync modified files
  deespec label validate --sync

  # Show detailed hash information
  deespec label validate --details`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize container
			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer container.Close()

			labelRepo := container.GetLabelRepository()
			ctx := context.Background()

			// Validate specific label or all labels
			var results []*repository.ValidationResult

			if len(args) == 1 {
				// Validate specific label
				nameOrID := args[0]
				var labelID int

				// Try to parse as ID first
				if id, err := strconv.Atoi(nameOrID); err == nil {
					labelID = id
				} else {
					// Find by name
					lbl, err := labelRepo.FindByName(ctx, nameOrID)
					if err != nil {
						return fmt.Errorf("label not found: %s", nameOrID)
					}
					labelID = lbl.ID()
				}

				result, err := labelRepo.ValidateIntegrity(ctx, labelID)
				if err != nil {
					return fmt.Errorf("failed to validate label: %w", err)
				}
				results = []*repository.ValidationResult{result}
			} else {
				// Validate all labels
				var err error
				results, err = labelRepo.ValidateAllLabels(ctx)
				if err != nil {
					return fmt.Errorf("failed to validate labels: %w", err)
				}
			}

			if len(results) == 0 {
				fmt.Println("No labels to validate")
				return nil
			}

			// Display results
			okCount := 0
			modifiedCount := 0
			missingCount := 0

			fmt.Printf("Validating %d label(s)...\n\n", len(results))

			if showDetails {
				// Detailed output with hashes
				for _, result := range results {
					// Get label info
					lbl, err := labelRepo.FindByID(ctx, result.LabelID)
					if err != nil {
						fmt.Printf("⚠ Warning: failed to get label info for ID %d: %v\n", result.LabelID, err)
						continue
					}

					fmt.Printf("Label: %s (ID: %d)\n", lbl.Name(), result.LabelID)
					fmt.Printf("  Status: %s\n", formatValidationStatus(result.Status))

					if result.Status != repository.ValidationOK {
						fmt.Printf("  Expected Hash: %s\n", result.ExpectedHash)
						fmt.Printf("  Actual Hash:   %s\n", result.ActualHash)
					}

					fmt.Println()

					updateStatusCounts(result.Status, &okCount, &modifiedCount, &missingCount)
				}
			} else {
				// Table output
				w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "ID\tNAME\tSTATUS\tMESSAGE")
				fmt.Fprintln(w, "--\t----\t------\t-------")

				for _, result := range results {
					// Get label name
					lbl, err := labelRepo.FindByID(ctx, result.LabelID)
					if err != nil {
						fmt.Printf("⚠ Warning: failed to get label info for ID %d: %v\n", result.LabelID, err)
						continue
					}

					statusSymbol, message := formatValidationResult(result.Status)
					fmt.Fprintf(w, "%d\t%s\t%s\t%s\n",
						result.LabelID, lbl.Name(), statusSymbol, message)

					updateStatusCounts(result.Status, &okCount, &modifiedCount, &missingCount)
				}

				w.Flush()
			}

			// Summary
			fmt.Printf("\nSummary:\n")
			fmt.Printf("  ✓ OK:       %d\n", okCount)
			if modifiedCount > 0 {
				fmt.Printf("  ⚠ Modified: %d\n", modifiedCount)
			}
			if missingCount > 0 {
				fmt.Printf("  ❌ Missing:  %d\n", missingCount)
			}

			// Sync if requested and there are issues
			if sync && (modifiedCount > 0 || missingCount > 0) {
				fmt.Printf("\n--sync flag detected. Syncing modified labels...\n\n")

				syncedCount := 0
				failedCount := 0

				for _, result := range results {
					if result.Status == repository.ValidationModified {
						lbl, err := labelRepo.FindByID(ctx, result.LabelID)
						if err != nil {
							fmt.Printf("⚠ Failed to sync label ID %d: %v\n", result.LabelID, err)
							failedCount++
							continue
						}

						if err := labelRepo.SyncFromFile(ctx, result.LabelID); err != nil {
							fmt.Printf("⚠ Failed to sync label '%s': %v\n", lbl.Name(), err)
							failedCount++
							continue
						}

						fmt.Printf("  ↻ Synced: %s (ID: %d)\n", lbl.Name(), result.LabelID)
						syncedCount++
					}
				}

				fmt.Printf("\nSync Summary: %d synced, %d failed\n", syncedCount, failedCount)

				// Note for missing files
				if missingCount > 0 {
					fmt.Printf("\nNote: Missing files cannot be auto-synced. Please restore the files or update label templates.\n")
				}
			} else if modifiedCount > 0 || missingCount > 0 {
				fmt.Printf("\nTip: Use --sync to automatically update hashes for modified files.\n")
			}

			// Return error if validation failed
			if modifiedCount > 0 || missingCount > 0 {
				return fmt.Errorf("validation failed: %d modified, %d missing", modifiedCount, missingCount)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&sync, "sync", false, "Automatically sync modified labels")
	cmd.Flags().BoolVar(&showDetails, "details", false, "Show detailed hash information")

	return cmd
}

// formatValidationStatus formats a validation status with color/symbol
func formatValidationStatus(status repository.ValidationStatus) string {
	switch status {
	case repository.ValidationOK:
		return "✓ OK"
	case repository.ValidationModified:
		return "⚠ MODIFIED"
	case repository.ValidationMissing:
		return "❌ MISSING"
	default:
		return "? UNKNOWN"
	}
}

// formatValidationResult formats validation result for table output
func formatValidationResult(status repository.ValidationStatus) (string, string) {
	switch status {
	case repository.ValidationOK:
		return "✓", "All files match"
	case repository.ValidationModified:
		return "⚠", "File content changed"
	case repository.ValidationMissing:
		return "❌", "Template file not found"
	default:
		return "?", "Unknown status"
	}
}

// updateStatusCounts updates counters based on validation status
func updateStatusCounts(status repository.ValidationStatus, okCount, modifiedCount, missingCount *int) {
	switch status {
	case repository.ValidationOK:
		*okCount++
	case repository.ValidationModified:
		*modifiedCount++
	case repository.ValidationMissing:
		*missingCount++
	}
}
