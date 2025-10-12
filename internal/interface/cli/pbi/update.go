package pbi

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/application/usecase/pbi"
	pbidomain "github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/persistence/sqlite"
	"github.com/spf13/cobra"
)

// NewUpdateCommand creates a new update command
func NewUpdateCommand() *cobra.Command {
	var (
		status      string
		storyPoints int
		priority    int
	)

	cmd := &cobra.Command{
		Use:   "update PBI_ID",
		Short: "Update a PBI's metadata",
		Long: `Update Product Backlog Item (PBI) metadata.
You can update status, story points, and priority.
The Markdown body is preserved unchanged.`,
		Example: `  # Update status
  deespec pbi update PBI-001 --status in_progress

  # Update multiple fields
  deespec pbi update PBI-001 \
    --status planning \
    --story-points 5 \
    --priority 1

  # Update priority only
  deespec pbi update PBI-002 --priority 2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			return runUpdate(pbiID, status, storyPoints, priority)
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Update status (pending|planning|planed|in_progress|done)")
	cmd.Flags().IntVarP(&storyPoints, "story-points", "s", -1, "Update story points (0-13)")
	cmd.Flags().IntVarP(&priority, "priority", "p", -1, "Update priority (0=通常, 1=高, 2=緊急)")

	return cmd
}

func runUpdate(pbiID, status string, storyPoints, priority int) error {
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

	// Create use case
	useCase := pbi.NewUpdatePBIUseCase(repo)

	// Build update options
	opts := pbi.UpdateOptions{}

	// Validate and set status
	if status != "" {
		validStatuses := []string{"pending", "planning", "planed", "in_progress", "done"}
		isValid := false
		for _, validStatus := range validStatuses {
			if status == validStatus {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid status: %s (valid: pending, planning, planed, in_progress, done)", status)
		}
		s := pbidomain.Status(status)
		opts.Status = &s
	}

	// Validate and set story points
	if storyPoints >= 0 {
		if storyPoints > 13 {
			return fmt.Errorf("story points must be between 0 and 13, got %d", storyPoints)
		}
		opts.EstimatedStoryPoints = &storyPoints
	}

	// Validate and set priority
	if priority >= 0 {
		if priority > 2 {
			return fmt.Errorf("priority must be 0 (通常), 1 (高), or 2 (緊急), got %d", priority)
		}
		p := pbidomain.Priority(priority)
		opts.Priority = &p
	}

	// Check if any updates were provided
	if opts.Status == nil && opts.EstimatedStoryPoints == nil && opts.Priority == nil {
		return fmt.Errorf("no updates specified (use --status, --story-points, or --priority)")
	}

	// Execute use case
	if err := useCase.Execute(pbiID, opts); err != nil {
		return fmt.Errorf("failed to update PBI: %w", err)
	}

	fmt.Printf("✅ PBI updated: %s\n", pbiID)
	fmt.Printf("\nView details: deespec pbi show %s\n", pbiID)

	return nil
}
