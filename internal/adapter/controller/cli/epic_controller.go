package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/input"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/spf13/cobra"
)

// EPICController handles EPIC-related CLI commands
type EPICController struct {
	taskUseCase         input.TaskUseCase
	epicWorkflowUseCase input.EPICWorkflowUseCase
	workflowUseCase     input.WorkflowUseCase
	presenter           output.Presenter
}

// NewEPICController creates a new EPIC controller
func NewEPICController(
	taskUC input.TaskUseCase,
	epicWorkflowUC input.EPICWorkflowUseCase,
	workflowUC input.WorkflowUseCase,
	presenter output.Presenter,
) *EPICController {
	return &EPICController{
		taskUseCase:         taskUC,
		epicWorkflowUseCase: epicWorkflowUC,
		workflowUseCase:     workflowUC,
		presenter:           presenter,
	}
}

// CreateCommand creates 'epic create' command
func (c *EPICController) CreateCommand() *cobra.Command {
	var (
		description          string
		estimatedStoryPoints int
		priority             int
		labels               []string
		assignedAgent        string
	)

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a new EPIC (Large Feature Group)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			req := dto.CreateEPICRequest{
				Title:                args[0],
				Description:          description,
				EstimatedStoryPoints: estimatedStoryPoints,
				Priority:             priority,
				Labels:               labels,
				AssignedAgent:        assignedAgent,
			}

			result, err := c.taskUseCase.CreateEPIC(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("EPIC created successfully", result)
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "EPIC description")
	cmd.Flags().IntVar(&estimatedStoryPoints, "points", 0, "Estimated total story points")
	cmd.Flags().IntVarP(&priority, "priority", "p", 3, "Priority (1-5)")
	cmd.Flags().StringArrayVar(&labels, "labels", []string{}, "Labels")
	cmd.Flags().StringVar(&assignedAgent, "agent", "", "Assigned agent type")

	return cmd
}

// GetCommand creates 'epic get' command
func (c *EPICController) GetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get [epic-id]",
		Short: "Get EPIC details by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicID := args[0]

			result, err := c.taskUseCase.GetEPIC(cmd.Context(), epicID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("EPIC details", result)
		},
	}
}

// ListCommand creates 'epic list' command
func (c *EPICController) ListCommand() *cobra.Command {
	var (
		statuses []string
		limit    int
		offset   int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List EPICs",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := dto.ListTasksRequest{
				Types:    []string{"EPIC"},
				Statuses: statuses,
				Limit:    limit,
				Offset:   offset,
			}

			result, err := c.taskUseCase.ListTasks(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("EPIC list", result)
		},
	}

	cmd.Flags().StringArrayVar(&statuses, "status", []string{}, "Filter by status")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")

	return cmd
}

// PickCommand creates 'epic pick' command
func (c *EPICController) PickCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pick [epic-id]",
		Short: "Pick an EPIC for decomposition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicID := args[0]

			err := c.workflowUseCase.PickTask(cmd.Context(), epicID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("EPIC picked for decomposition", nil)
		},
	}
}

// DecomposeCommand creates 'epic decompose' command
func (c *EPICController) DecomposeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "decompose [epic-id]",
		Short: "Decompose EPIC into PBIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicID := args[0]

			if err := c.presenter.PresentProgress("Decomposing EPIC into PBIs...", 0, 0); err != nil {
				return err
			}

			result, err := c.epicWorkflowUseCase.DecomposeEPIC(cmd.Context(), epicID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("EPIC decomposed successfully", result)
		},
	}
}

// ApproveCommand creates 'epic approve' command
func (c *EPICController) ApproveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "approve [epic-id]",
		Short: "Approve EPIC decomposition and create PBIs",
		Long:  "Approves the decomposition result and creates the proposed PBIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicID := args[0]

			// TODO: In a real implementation, this would parse the decomposition result
			// and convert it to CreatePBIRequest objects
			var pbiRequests []dto.CreatePBIRequest

			if err := c.presenter.PresentProgress("Creating PBIs from decomposition...", 0, 0); err != nil {
				return err
			}

			pbiIDs, err := c.epicWorkflowUseCase.ApproveEPICDecomposition(cmd.Context(), epicID, pbiRequests)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBIs created successfully", map[string]interface{}{
				"pbi_ids": pbiIDs,
				"count":   len(pbiIDs),
			})
		},
	}
}

// UpdateStatusCommand creates 'epic status' command
func (c *EPICController) UpdateStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status [epic-id] [new-status]",
		Short: "Update EPIC status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicID := args[0]
			newStatus := args[1]

			err := c.taskUseCase.UpdateTaskStatus(cmd.Context(), epicID, newStatus)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("EPIC status updated", nil)
		},
	}
}

// DeleteCommand creates 'epic delete' command
func (c *EPICController) DeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [epic-id]",
		Short: "Delete an EPIC",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicID := args[0]

			err := c.taskUseCase.DeleteTask(cmd.Context(), epicID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("EPIC deleted successfully", nil)
		},
	}
}

// BuildCommand creates the 'epic' parent command with all subcommands
func (c *EPICController) BuildCommand() *cobra.Command {
	epicCmd := &cobra.Command{
		Use:   "epic",
		Short: "Manage EPIC (Large Feature Groups)",
		Long:  "Commands for creating, listing, and managing EPIC tasks",
	}

	epicCmd.AddCommand(
		c.CreateCommand(),
		c.GetCommand(),
		c.ListCommand(),
		c.PickCommand(),
		c.DecomposeCommand(),
		c.ApproveCommand(),
		c.UpdateStatusCommand(),
		c.DeleteCommand(),
	)

	return epicCmd
}
