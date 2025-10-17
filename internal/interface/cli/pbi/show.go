package pbi

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/spf13/cobra"
)

// pbiShowFlags holds the flags for pbi show command
type pbiShowFlags struct {
	detail bool // Show PBI detail instead of SBI list
}

// NewShowCommand creates a new show command
func NewShowCommand() *cobra.Command {
	flags := &pbiShowFlags{}

	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show PBI's SBI list or PBI details",
		Long: `Display SBI list for a specific PBI (default) or PBI file details (with --detail flag).

By default, shows all SBIs belonging to the specified PBI.
Use --detail flag to see the PBI file content.`,
		Example: `  # Show SBI list for PBI
  deespec pbi show PBI-001

  # Show PBI file details
  deespec pbi show PBI-001 --detail
  deespec pbi show PBI-001 -d`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			if flags.detail {
				return runShowDetail(pbiID)
			}
			return runShowSBIList(cmd.Context(), pbiID)
		},
	}

	cmd.Flags().BoolVarP(&flags.detail, "detail", "d", false, "Show PBI file details instead of SBI list")

	return cmd
}

// runShowSBIList displays SBI list for a specific PBI
func runShowSBIList(ctx context.Context, pbiID string) error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Get Task UseCase
	taskUseCase := container.GetTaskUseCase()

	// Build filter with parent PBI ID
	req := dto.ListTasksRequest{
		Types:    []string{"SBI"},
		ParentID: &pbiID,
		Limit:    100, // Show all SBIs for this PBI
		Offset:   0,
	}

	// Execute list operation
	response, err := taskUseCase.ListTasks(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to list SBIs for PBI %s: %w", pbiID, err)
	}

	// Display SBI list in table format
	return outputSBITable(response.Tasks, pbiID, taskUseCase, ctx)
}

// outputSBITable outputs the SBI list in table format
func outputSBITable(tasks []dto.TaskDTO, pbiID string, taskUseCase interface{ GetSBI(context.Context, string) (*dto.SBIDTO, error) }, ctx context.Context) error {
	if len(tasks) == 0 {
		fmt.Printf("No SBIs found for PBI: %s\n", pbiID)
		return nil
	}

	// Print header
	fmt.Printf("SBIs for PBI: %s\n", pbiID)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Create table writer
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print table header
	fmt.Fprintf(w, "ID\tTITLE\tSTATUS\tSTEP\tTURN\tSTARTED\tCOMPLETED\tCREATED\n")
	fmt.Fprintf(w, "---\t-----\t------\t----\t----\t-------\t---------\t-------\n")

	// Print rows
	for _, task := range tasks {
		id := task.ID
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
	fmt.Fprintf(w, "Total: %d SBIs for PBI %s\n", len(tasks), pbiID)

	return nil
}

// runShowDetail displays PBI file details
func runShowDetail(pbiID string) error {
	// Open database
	db, err := sql.Open("sqlite3", ".deespec/deespec.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run migrations first
	migrator := sqlite.NewMigrator(db)
	if err := migrator.Migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Create repository
	rootPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	repo := persistence.NewPBISQLiteRepository(db, rootPath)

	// Find PBI
	p, err := repo.FindByID(pbiID)
	if err != nil {
		return fmt.Errorf("failed to find PBI: %w", err)
	}

	// Get body
	body, err := repo.GetBody(pbiID)
	if err != nil {
		return fmt.Errorf("failed to get PBI body: %w", err)
	}

	// Display PBI details
	fmt.Printf("📦 %s: %s\n", p.ID, p.Title)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
	fmt.Printf("📊 Status: %s\n", p.Status)
	fmt.Printf("🔢 Story Points: %d\n", p.EstimatedStoryPoints)
	fmt.Printf("⭐ Priority: %s (%d)\n", p.Priority.String(), p.Priority)
	if p.ParentEpicID != "" {
		fmt.Printf("📂 Parent EPIC: %s\n", p.ParentEpicID)
	}
	fmt.Println()
	fmt.Printf("📄 Markdown File: %s\n", p.GetMarkdownPath())
	fmt.Println()
	fmt.Printf("🕐 Created: %s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("🕐 Updated: %s\n", p.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Println()
	fmt.Println("📋 Body:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println(body)

	return nil
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
