package sbi

import (
	"context"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/spf13/cobra"
)

// sbiShowFlags holds the flags for sbi show command
type sbiShowFlags struct {
	jsonOut bool // Output in JSON format
	turn    int  // Specific turn to display (0 = show all)
}

// NewSBIShowCommand creates the sbi show command
func NewSBIShowCommand() *cobra.Command {
	flags := &sbiShowFlags{}

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show detailed information about an SBI",
		Long: `Display detailed information about a specific SBI task.

Shows all metadata including status, priority, sequence, timestamps, labels, execution state, and work history.

Examples:
  # Show SBI details with work history
  deespec sbi show 010b1f9c-2cbf-40e6-90d8-ecba5b62d335

  # Show specific turn report
  deespec sbi show 010b1f9c --turn 1
  deespec sbi show 010b1f9c -t 2

  # Show in JSON format
  deespec sbi show 010b1f9c --json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSBIShow(cmd.Context(), args[0], flags)
		},
	}

	// Define flags
	cmd.Flags().BoolVar(&flags.jsonOut, "json", false, "Output in JSON format")
	cmd.Flags().IntVarP(&flags.turn, "turn", "t", 0, "Show specific turn report (0 = show all work history)")

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

	// Get repositories
	sbiRepo := container.GetSBIRepository()
	execLogRepo := container.GetSBIExecLogRepository()

	// Find SBI by ID
	sbiEntity, err := sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return fmt.Errorf("SBI not found: %s (error: %w)", sbiID, err)
	}

	// Get execution logs
	execLogs, err := execLogRepo.FindBySBIID(ctx, sbiID)
	if err != nil {
		return fmt.Errorf("failed to get execution logs: %w", err)
	}

	// If specific turn is requested, show turn report
	if flags.turn > 0 {
		return outputTurnReport(sbiID, flags.turn, execLogs)
	}

	// Output results
	if flags.jsonOut {
		return outputJSONShow(sbiEntity)
	}

	return outputDetailShow(sbiEntity, execLogs)
}

// outputDetailShow outputs SBI details in human-readable format
func outputDetailShow(s *sbi.SBI, execLogs []*repository.SBIExecLog) error {
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

	// Display work history if available
	if len(execLogs) > 0 {
		fmt.Printf("\nWork History:\n")
		fmt.Printf("=============\n\n")

		// Group logs by turn
		turnMap := make(map[int][]*repository.SBIExecLog)
		for _, log := range execLogs {
			turnMap[log.Turn] = append(turnMap[log.Turn], log)
		}

		// Display each turn
		for turn := 1; turn <= execState.CurrentTurn.Value(); turn++ {
			logs, exists := turnMap[turn]
			if !exists {
				continue
			}

			fmt.Printf("Turn %d:\n", turn)
			for _, log := range logs {
				timestamp := log.ExecutedAt.Format("2006-01-02 15:04:05")
				if log.Step == "IMPLEMENT" {
					fmt.Printf("  IMPLEMENT: %s → %s\n", timestamp, log.ReportPath)
				} else if log.Step == "REVIEW" {
					decision := ""
					if log.Decision != nil {
						decision = fmt.Sprintf(" (%s)", *log.Decision)
					}
					fmt.Printf("  REVIEW:    %s → %s%s\n", timestamp, log.ReportPath, decision)
				}
			}
			fmt.Printf("\n")
		}

		fmt.Printf("💡 Use --turn N flag to view specific turn report\n")
		fmt.Printf("   Example: deespec sbi show %s --turn 1\n", s.ID().String())
	}

	fmt.Printf("\n")

	return nil
}

// outputTurnReport outputs a specific turn's report
func outputTurnReport(sbiID string, turn int, execLogs []*repository.SBIExecLog) error {
	// Find logs for the specified turn
	var turnLogs []*repository.SBIExecLog
	for _, log := range execLogs {
		if log.Turn == turn {
			turnLogs = append(turnLogs, log)
		}
	}

	if len(turnLogs) == 0 {
		return fmt.Errorf("no reports found for turn %d", turn)
	}

	fmt.Printf("SBI Turn %d Report\n", turn)
	fmt.Printf("==================\n\n")
	fmt.Printf("SBI ID: %s\n\n", sbiID)

	for _, log := range turnLogs {
		fmt.Printf("=== %s Report ===\n", log.Step)
		fmt.Printf("Executed At: %s\n", log.ExecutedAt.Format("2006-01-02 15:04:05"))
		if log.Decision != nil {
			fmt.Printf("Decision: %s\n", *log.Decision)
		}
		fmt.Printf("\n")

		// Read and display report content
		content, err := os.ReadFile(log.ReportPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  WARNING: Failed to read report file: %s\n", log.ReportPath)
			fmt.Fprintf(os.Stderr, "   Error: %v\n\n", err)
			continue
		}

		fmt.Printf("--- Report Content (READ-ONLY) ---\n")
		fmt.Printf("%s\n", string(content))
		fmt.Printf("--- End of Report ---\n\n")
	}

	fmt.Printf("📝 Note: Reports are read-only and cannot be edited.\n")
	fmt.Printf("💡 Use 'deespec sbi show %s' to see full SBI details and work history.\n", sbiID)

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
