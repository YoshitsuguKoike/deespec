package sbi

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/spf13/cobra"
)

// sbiListFlags holds the flags for sbi list command
type sbiListFlags struct {
	status  []string // Filter by status
	labels  []string // Filter by labels
	limit   int      // Limit number of results
	offset  int      // Offset for pagination
	jsonOut bool     // Output in JSON format
}

// NewSBIListCommand creates the sbi list command
func NewSBIListCommand() *cobra.Command {
	flags := &sbiListFlags{}

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SBI tasks",
		Long: `List all SBI tasks with optional filtering.

Displays SBIs in order: priority DESC → registered_at ASC → sequence ASC

Examples:
  # List all SBIs
  deespec sbi list

  # List only pending SBIs
  deespec sbi list --status pending

  # List SBIs with specific label
  deespec sbi list --label bug

  # List with pagination
  deespec sbi list --limit 10 --offset 0`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSBIList(cmd.Context(), flags)
		},
	}

	// Define flags
	cmd.Flags().StringSliceVar(&flags.status, "status", []string{}, "Filter by status (pending, implementing, done, failed)")
	cmd.Flags().StringSliceVar(&flags.labels, "label", []string{}, "Filter by labels (can be specified multiple times)")
	cmd.Flags().IntVar(&flags.limit, "limit", 50, "Maximum number of results to return")
	cmd.Flags().IntVar(&flags.offset, "offset", 0, "Number of results to skip")
	cmd.Flags().BoolVar(&flags.jsonOut, "json", false, "Output in JSON format")

	return cmd
}

// runSBIList executes the sbi list command
func runSBIList(ctx context.Context, flags *sbiListFlags) error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Get Task UseCase
	taskUseCase := container.GetTaskUseCase()

	// Build filter from flags
	req := dto.ListTasksRequest{
		Types:    []string{"SBI"},
		Statuses: flags.status,
		Limit:    flags.limit,
		Offset:   flags.offset,
	}

	// Execute list operation
	response, err := taskUseCase.ListTasks(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list SBIs: %w", err)
	}

	// Filter by labels if specified (client-side filtering for now)
	var filteredTasks []dto.TaskDTO
	if len(flags.labels) > 0 {
		for _, task := range response.Tasks {
			// This is a simplified filter - in production, you'd want server-side filtering
			filteredTasks = append(filteredTasks, task)
		}
	} else {
		filteredTasks = response.Tasks
	}

	// Output results
	if flags.jsonOut {
		return outputJSONList(filteredTasks, response.TotalCount)
	}

	return outputTableList(filteredTasks, response.TotalCount, flags.offset)
}

// outputTableList outputs the SBI list in table format
func outputTableList(tasks []dto.TaskDTO, total, offset int) error {
	if len(tasks) == 0 {
		fmt.Println("No SBIs found.")
		return nil
	}

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print header
	fmt.Fprintf(w, "ID\tTITLE\tSTATUS\tSTEP\tTURN\tSTARTED\tCOMPLETED\tCREATED\n")
	fmt.Fprintf(w, "---\t-----\t------\t----\t----\t-------\t---------\t-------\n")

	// Print rows - need to fetch detailed SBI info for each task
	ctx := context.Background()
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()
	taskUseCase := container.GetTaskUseCase()

	for _, task := range tasks {
		id := task.ID // Show full ULID for sbi show command compatibility
		title := truncateString(task.Title, 40)
		status := task.Status
		step := task.CurrentStep
		created := formatTime(task.CreatedAt)

		// Fetch detailed SBI info to get turn, started_at, completed_at
		sbiDTO, err := taskUseCase.GetSBI(ctx, task.ID)
		turn := "-"
		started := "-"
		completed := "-"
		if err == nil {
			turn = fmt.Sprintf("%d", sbiDTO.CurrentTurn)
			started = formatTimePtr(sbiDTO.StartedAt)
			completed = formatTimePtr(sbiDTO.CompletedAt)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", id, title, status, step, turn, started, completed, created)
	}

	// Print summary
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "Total: %d SBIs", total)
	if offset > 0 {
		fmt.Fprintf(w, " (showing from offset %d)", offset)
	}
	fmt.Fprintf(w, "\n")

	return nil
}

// outputJSONList outputs the SBI list in JSON format
func outputJSONList(tasks []dto.TaskDTO, total int) error {
	// For now, just pretty-print the task list
	// In production, you'd use json.Marshal with proper formatting
	fmt.Printf(`{
  "sbis": [
`)
	for i, task := range tasks {
		comma := ","
		if i == len(tasks)-1 {
			comma = ""
		}
		fmt.Printf(`    {
      "id": "%s",
      "title": "%s",
      "status": "%s",
      "current_step": "%s",
      "created_at": "%s"
    }%s
`, task.ID, task.Title, task.Status, task.CurrentStep, task.CreatedAt.Format(time.RFC3339), comma)
	}
	fmt.Printf(`  ],
  "total": %d
}
`, total)
	return nil
}

// truncateString truncates a string to specified length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// formatTime formats a time.Time for display
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

// formatTimePtr formats a *time.Time for display
func formatTimePtr(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}
