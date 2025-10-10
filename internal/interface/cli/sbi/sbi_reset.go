package sbi

import (
	"context"
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/spf13/cobra"
)

// sbiResetFlags holds the flags for sbi reset command
type sbiResetFlags struct {
	toStatus string // Target status to reset to
	force    bool   // Force reset without confirmation
}

// NewSBIResetCommand creates the sbi reset command
func NewSBIResetCommand() *cobra.Command {
	flags := &sbiResetFlags{}

	cmd := &cobra.Command{
		Use:   "reset <id>",
		Short: "Reset an SBI to a specific status",
		Long: `Reset an SBI task to a specific status for re-execution.

This is useful when:
- An SBI was incorrectly marked as completed
- Claude Code authentication failed and the SBI needs to be retried
- You want to re-execute a task from a specific step

Available statuses:
  - pending:      Reset to initial state (ready for pick)
  - implementing: Reset to implementation phase
  - reviewing:    Reset to review phase

Examples:
  # Reset SBI to pending status
  deespec sbi reset 010b1f9c --to-status pending

  # Force reset without confirmation
  deespec sbi reset 010b1f9c --to-status pending --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSBIReset(cmd.Context(), args[0], flags)
		},
	}

	// Define flags
	cmd.Flags().StringVar(&flags.toStatus, "to-status", "pending", "Target status (pending, implementing, reviewing)")
	cmd.Flags().BoolVar(&flags.force, "force", false, "Force reset without confirmation")

	return cmd
}

// runSBIReset executes the sbi reset command
func runSBIReset(ctx context.Context, sbiID string, flags *sbiResetFlags) error {
	// Validate target status
	validStatuses := map[string]bool{
		"pending":      true,
		"implementing": true,
		"reviewing":    true,
	}
	if !validStatuses[flags.toStatus] {
		return fmt.Errorf("invalid status: %s (must be: pending, implementing, or reviewing)", flags.toStatus)
	}

	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Get SBI Repository
	sbiRepo := container.GetSBIRepository()

	// Find SBI to confirm it exists
	sbiEntity, err := sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return fmt.Errorf("SBI not found: %s (error: %w)", sbiID, err)
	}

	// Show current state and ask for confirmation (unless --force)
	if !flags.force {
		fmt.Printf("Current SBI State:\n")
		fmt.Printf("  ID:     %s\n", sbiEntity.ID().String())
		fmt.Printf("  Title:  %s\n", sbiEntity.Title())
		fmt.Printf("  Status: %s\n", sbiEntity.Status())
		fmt.Printf("  Step:   %s\n", sbiEntity.CurrentStep())
		fmt.Printf("\n")
		fmt.Printf("Reset to status: %s\n", flags.toStatus)
		fmt.Printf("\nAre you sure? (y/N): ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Reset cancelled.")
			return nil
		}
	}

	// Perform reset using ResetSBIState
	err = sbiRepo.ResetSBIState(ctx, repository.SBIID(sbiID), flags.toStatus)
	if err != nil {
		return fmt.Errorf("failed to reset SBI: %w", err)
	}

	fmt.Printf("✓ SBI %s has been reset to status: %s\n", sbiID, flags.toStatus)
	fmt.Printf("\nYou can now re-run the SBI with: deespec sbi run\n")

	return nil
}
