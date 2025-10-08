package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/input"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/spf13/cobra"
)

// SBIController handles SBI-related CLI commands
type SBIController struct {
	taskUseCase        input.TaskUseCase
	sbiWorkflowUseCase input.SBIWorkflowUseCase
	workflowUseCase    input.WorkflowUseCase
	presenter          output.Presenter
}

// NewSBIController creates a new SBI controller
func NewSBIController(
	taskUC input.TaskUseCase,
	sbiWorkflowUC input.SBIWorkflowUseCase,
	workflowUC input.WorkflowUseCase,
	presenter output.Presenter,
) *SBIController {
	return &SBIController{
		taskUseCase:        taskUC,
		sbiWorkflowUseCase: sbiWorkflowUC,
		workflowUseCase:    workflowUC,
		presenter:          presenter,
	}
}

// CreateCommand creates 'sbi create' command
func (c *SBIController) CreateCommand() *cobra.Command {
	var (
		description    string
		pbiID          string
		priority       int
		estimatedHours float64
		labels         []string
		assignedAgent  string
		filePaths      []string
	)

	cmd := &cobra.Command{
		Use:   "create [title]",
		Short: "Create a new SBI (Spec Backlog Item)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var parentPBIID *string
			if pbiID != "" {
				parentPBIID = &pbiID
			}

			req := dto.CreateSBIRequest{
				Title:          args[0],
				Description:    description,
				ParentPBIID:    parentPBIID,
				Priority:       priority,
				EstimatedHours: estimatedHours,
				Labels:         labels,
				AssignedAgent:  assignedAgent,
				FilePaths:      filePaths,
			}

			result, err := c.taskUseCase.CreateSBI(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBI created successfully", result)
		},
	}

	cmd.Flags().StringVarP(&description, "description", "d", "", "SBI description")
	cmd.Flags().StringVarP(&pbiID, "pbi", "b", "", "Parent PBI ID")
	cmd.Flags().IntVarP(&priority, "priority", "p", 3, "Priority (1-5, higher is more urgent)")
	cmd.Flags().Float64VarP(&estimatedHours, "hours", "H", 0, "Estimated hours")
	cmd.Flags().StringArrayVar(&labels, "labels", []string{}, "Labels")
	cmd.Flags().StringVar(&assignedAgent, "agent", "", "Assigned agent type")
	cmd.Flags().StringArrayVar(&filePaths, "files", []string{}, "File paths to modify")

	return cmd
}

// GetCommand creates 'sbi get' command
func (c *SBIController) GetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "get [sbi-id]",
		Short: "Get SBI details by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			result, err := c.taskUseCase.GetSBI(cmd.Context(), sbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBI details", result)
		},
	}
}

// ListCommand creates 'sbi list' command
func (c *SBIController) ListCommand() *cobra.Command {
	var (
		statuses []string
		limit    int
		offset   int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List SBIs",
		RunE: func(cmd *cobra.Command, args []string) error {
			req := dto.ListTasksRequest{
				Types:    []string{"SBI"},
				Statuses: statuses,
				Limit:    limit,
				Offset:   offset,
			}

			result, err := c.taskUseCase.ListTasks(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBI list", result)
		},
	}

	cmd.Flags().StringArrayVar(&statuses, "status", []string{}, "Filter by status")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of results")
	cmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination")

	return cmd
}

// PickCommand creates 'sbi pick' command
func (c *SBIController) PickCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pick [sbi-id]",
		Short: "Pick an SBI for implementation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			err := c.workflowUseCase.PickTask(cmd.Context(), sbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBI picked for implementation", nil)
		},
	}
}

// GenerateCommand creates 'sbi generate' command
func (c *SBIController) GenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate [sbi-id]",
		Short: "Generate code for an SBI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			if err := c.presenter.PresentProgress("Generating code...", 0, 0); err != nil {
				return err
			}

			result, err := c.sbiWorkflowUseCase.GenerateSBICode(cmd.Context(), sbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("Code generated successfully", result)
		},
	}
}

// ApplyCommand creates 'sbi apply' command
func (c *SBIController) ApplyCommand() *cobra.Command {
	var artifactPaths []string

	cmd := &cobra.Command{
		Use:   "apply [sbi-id]",
		Short: "Apply generated code to filesystem",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			if err := c.presenter.PresentProgress("Applying code to filesystem...", 0, 0); err != nil {
				return err
			}

			err := c.sbiWorkflowUseCase.ApplySBICode(cmd.Context(), sbiID, artifactPaths)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("Code applied successfully", nil)
		},
	}

	cmd.Flags().StringArrayVar(&artifactPaths, "artifacts", []string{}, "Artifact paths to apply")

	return cmd
}

// RetryCommand creates 'sbi retry' command
func (c *SBIController) RetryCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "retry [sbi-id]",
		Short: "Retry SBI implementation after failure",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			if err := c.presenter.PresentProgress("Retrying SBI implementation...", 0, 0); err != nil {
				return err
			}

			result, err := c.sbiWorkflowUseCase.RetrySBIImplementation(cmd.Context(), sbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBI implementation retried", result)
		},
	}
}

// UpdateStatusCommand creates 'sbi status' command
func (c *SBIController) UpdateStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status [sbi-id] [new-status]",
		Short: "Update SBI status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]
			newStatus := args[1]

			err := c.taskUseCase.UpdateTaskStatus(cmd.Context(), sbiID, newStatus)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBI status updated", nil)
		},
	}
}

// DeleteCommand creates 'sbi delete' command
func (c *SBIController) DeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [sbi-id]",
		Short: "Delete an SBI",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sbiID := args[0]

			err := c.taskUseCase.DeleteTask(cmd.Context(), sbiID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("SBI deleted successfully", nil)
		},
	}
}

// BuildCommand creates the 'sbi' parent command with all subcommands
func (c *SBIController) BuildCommand() *cobra.Command {
	sbiCmd := &cobra.Command{
		Use:   "sbi",
		Short: "Manage SBI (Spec Backlog Items)",
		Long:  "Commands for creating, listing, and managing SBI tasks",
	}

	sbiCmd.AddCommand(
		c.CreateCommand(),
		c.GetCommand(),
		c.ListCommand(),
		c.PickCommand(),
		c.GenerateCommand(),
		c.ApplyCommand(),
		c.RetryCommand(),
		c.UpdateStatusCommand(),
		c.DeleteCommand(),
	)

	return sbiCmd
}
