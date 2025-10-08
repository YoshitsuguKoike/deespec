package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/input"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/spf13/cobra"
)

// PBIController handles PBI-related CLI commands
type PBIController struct {
	taskUseCase        input.TaskUseCase
	pbiWorkflowUseCase input.PBIWorkflowUseCase
	workflowUseCase    input.WorkflowUseCase
	presenter          output.Presenter
}

// NewPBIController creates a new PBI controller
func NewPBIController(
	taskUC input.TaskUseCase,
	pbiWorkflowUC input.PBIWorkflowUseCase,
	workflowUC input.WorkflowUseCase,
	presenter output.Presenter,
) *PBIController {
	return &PBIController{
		taskUseCase:        taskUC,
		pbiWorkflowUseCase: pbiWorkflowUC,
		workflowUseCase:    workflowUC,
		presenter:          presenter,
	}
}

// CreateCommand creates 'pbi create' command
func (c *PBIController) CreateCommand() *cobra.Command {
	var (
		description        string
		epicID             string
		acceptanceCriteria []string
		storyPoints        int
		priority           int
		labels             []string
		assignedAgent      string
	)

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a new PBI (Product Backlog Item)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var parentEPICID *string
			if epicID != "" {
				parentEPICID = &epicID
			}

			req := dto.CreatePBIRequest{
				Title:              args[0],
				Description:        description,
				ParentEPICID:       parentEPICID,
				AcceptanceCriteria: acceptanceCriteria,
				StoryPoints:        storyPoints,
				Priority:           priority,
				Labels:             labels,
				AssignedAgent:      assignedAgent,
			}

			result, err := c.taskUseCase.CreatePBI(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBI created successfully", result)
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "PBI description")
	cmd.Flags().StringVarP(&epicID, "epic", "e", "", "Parent EPIC ID")
	cmd.Flags().StringArrayVar(&acceptanceCriteria, "criteria", []string{}, "Acceptance criteria")
	cmd.Flags().IntVar(&storyPoints, "points", 0, "Story points")
	cmd.Flags().IntVarP(&priority, "priority", "p", 3, "Priority (1-5)")
	cmd.Flags().StringArrayVar(&labels, "labels", []string{}, "Labels")
	cmd.Flags().StringVar(&assignedAgent, "agent", "", "Assigned agent type")

	return cmd
}

// GetCommand creates 'pbi get' command
func (c *PBIController) GetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get [pbi-id]",
		Short: "Get PBI details by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]

			result, err := c.taskUseCase.GetPBI(cmd.Context(), pbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBI details", result)
		},
	}
}

// ListCommand creates 'pbi list' command
func (c *PBIController) ListCommand() *cobra.Command {
	var (
		epicID   string
		statuses []string
		limit    int
		offset   int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List PBIs",
		RunE: func(cmd *cobra.Command, args []string) error {
			var parentID *string
			if epicID != "" {
				parentID = &epicID
			}

			req := dto.ListTasksRequest{
				Types:    []string{"PBI"},
				ParentID: parentID,
				Statuses: statuses,
				Limit:    limit,
				Offset:   offset,
			}

			result, err := c.taskUseCase.ListTasks(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBI list", result)
		},
	}

	cmd.Flags().StringVar(&epicID, "epic", "", "Filter by parent EPIC ID")
	cmd.Flags().StringArrayVar(&statuses, "status", []string{}, "Filter by status")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")

	return cmd
}

// PickCommand creates 'pbi pick' command
func (c *PBIController) PickCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pick [pbi-id]",
		Short: "Pick a PBI for decomposition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]

			err := c.workflowUseCase.PickTask(cmd.Context(), pbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBI picked for decomposition", nil)
		},
	}
}

// DecomposeCommand creates 'pbi decompose' command
func (c *PBIController) DecomposeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "decompose [pbi-id]",
		Short: "Decompose PBI into SBIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]

			if err := c.presenter.PresentProgress("Decomposing PBI into SBIs...", 0, 0); err != nil {
				return err
			}

			result, err := c.pbiWorkflowUseCase.DecomposePBI(cmd.Context(), pbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBI decomposed successfully", result)
		},
	}
}

// ApproveCommand creates 'pbi approve' command
func (c *PBIController) ApproveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "approve [pbi-id]",
		Short: "Approve PBI decomposition and create SBIs",
		Long:  "Approves the decomposition result and creates the proposed SBIs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]

			// TODO: In a real implementation, this would parse the decomposition result
			// and convert it to CreateSBIRequest objects
			var sbiRequests []dto.CreateSBIRequest

			if err := c.presenter.PresentProgress("Creating SBIs from decomposition...", 0, 0); err != nil {
				return err
			}

			sbiIDs, err := c.pbiWorkflowUseCase.ApprovePBIDecomposition(cmd.Context(), pbiID, sbiRequests)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBIs created successfully", map[string]interface{}{
				"sbi_ids": sbiIDs,
				"count":   len(sbiIDs),
			})
		},
	}
}

// UpdateStatusCommand creates 'pbi status' command
func (c *PBIController) UpdateStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status [pbi-id] [new-status]",
		Short: "Update PBI status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]
			newStatus := args[1]

			err := c.taskUseCase.UpdateTaskStatus(cmd.Context(), pbiID, newStatus)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBI status updated", nil)
		},
	}
}

// DeleteCommand creates 'pbi delete' command
func (c *PBIController) DeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [pbi-id]",
		Short: "Delete a PBI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pbiID := args[0]

			err := c.taskUseCase.DeleteTask(cmd.Context(), pbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("PBI deleted successfully", nil)
		},
	}
}

// BuildCommand creates the 'pbi' parent command with all subcommands
func (c *PBIController) BuildCommand() *cobra.Command {
	pbiCmd := &cobra.Command{
		Use:   "pbi",
		Short: "Manage PBI (Product Backlog Items)",
		Long:  "Commands for creating, listing, and managing PBI tasks",
	}

	pbiCmd.AddCommand(
		c.CreateCommand(),
		c.GetCommand(),
		c.ListCommand(),
		c.PickCommand(),
		c.DecomposeCommand(),
		c.ApproveCommand(),
		c.UpdateStatusCommand(),
		c.DeleteCommand(),
	)

	return pbiCmd
}
