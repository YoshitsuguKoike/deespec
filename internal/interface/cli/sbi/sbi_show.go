package sbi

import (
	"context"
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/spf13/cobra"
)

// sbiShowFlags holds the flags for sbi show command
type sbiShowFlags struct {
	jsonOut bool // Output in JSON format
}

// NewSBIShowCommand creates the sbi show command
func NewSBIShowCommand() *cobra.Command {
	flags := &sbiShowFlags{}

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show detailed information about an SBI",
		Long: `Display detailed information about a specific SBI task.

Shows all metadata including status, priority, sequence, timestamps, labels, and execution state.

Examples:
  # Show SBI details
  deespec sbi show 010b1f9c-2cbf-40e6-90d8-ecba5b62d335

  # Show with full ID
  deespec sbi show 010b1f9c-2cbf-40e6-90d8-ecba5b62d335

  # Show in JSON format
  deespec sbi show 010b1f9c --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSBIShow(cmd.Context(), args[0], flags)
		},
	}

	// Define flags
	cmd.Flags().BoolVar(&flags.jsonOut, "json", false, "Output in JSON format")

	return cmd
}

// runSBIShow executes the sbi show command
func runSBIShow(ctx context.Context, sbiID string, flags *sbiShowFlags) error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Get SBI Repository
	sbiRepo := container.GetSBIRepository()

	// Find SBI by ID
	sbiEntity, err := sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return fmt.Errorf("SBI not found: %s (error: %w)", sbiID, err)
	}

	// Output results
	if flags.jsonOut {
		return outputJSONShow(sbiEntity)
	}

	return outputDetailShow(sbiEntity)
}

// outputDetailShow outputs SBI details in human-readable format
func outputDetailShow(s *sbi.SBI) error {
	metadata := s.Metadata()
	execState := s.ExecutionState()

	fmt.Printf("SBI Details:\n")
	fmt.Printf("=============\n\n")
	fmt.Printf("ID:              %s\n", s.ID().String())
	fmt.Printf("Title:           %s\n", s.Title())
	fmt.Printf("Status:          %s\n", s.Status())
	fmt.Printf("Current Step:    %s\n", s.CurrentStep())
	fmt.Printf("Priority:        %d\n", metadata.Priority)
	fmt.Printf("Sequence:        %d\n", metadata.Sequence)
	fmt.Printf("Registered At:   %s\n", metadata.RegisteredAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Created At:      %s\n", s.CreatedAt().String())
	fmt.Printf("Updated At:      %s\n", s.UpdatedAt().String())

	if len(metadata.Labels) > 0 {
		fmt.Printf("Labels:          %v\n", metadata.Labels)
	}
	if metadata.AssignedAgent != "" {
		fmt.Printf("Assigned Agent:  %s\n", metadata.AssignedAgent)
	}

	fmt.Printf("\nExecution State:\n")
	fmt.Printf("  Current Turn:    %d\n", execState.CurrentTurn.Value())
	fmt.Printf("  Current Attempt: %d\n", execState.CurrentAttempt.Value())
	fmt.Printf("  Max Turns:       %d\n", execState.MaxTurns)
	fmt.Printf("  Max Attempts:    %d\n", execState.MaxAttempts)

	if execState.LastError != "" {
		fmt.Printf("  Last Error:      %s\n", execState.LastError)
	}

	fmt.Printf("\n")

	return nil
}

// outputJSONShow outputs SBI details in JSON format
func outputJSONShow(s *sbi.SBI) error {
	metadata := s.Metadata()
	execState := s.ExecutionState()

	fmt.Printf(`{
  "id": "%s",
  "title": "%s",
  "status": "%s",
  "current_step": "%s",
  "priority": %d,
  "sequence": %d,
  "registered_at": "%s",
  "created_at": "%s",
  "updated_at": "%s",
  "labels": %v,
  "assigned_agent": "%s",
  "execution_state": {
    "current_turn": %d,
    "current_attempt": %d,
    "max_turns": %d,
    "max_attempts": %d,
    "last_error": "%s"
  }
}
`,
		s.ID().String(),
		s.Title(),
		s.Status(),
		s.CurrentStep(),
		metadata.Priority,
		metadata.Sequence,
		metadata.RegisteredAt.Format("2006-01-02T15:04:05Z07:00"),
		s.CreatedAt().String(),
		s.UpdatedAt().String(),
		metadata.Labels,
		metadata.AssignedAgent,
		execState.CurrentTurn.Value(),
		execState.CurrentAttempt.Value(),
		execState.MaxTurns,
		execState.MaxAttempts,
		execState.LastError,
	)
	return nil
}
