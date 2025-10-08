package cli

import (
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/input"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/spf13/cobra"
)

// WorkflowController handles workflow execution CLI commands
type WorkflowController struct {
	workflowUseCase input.WorkflowUseCase
	taskUseCase     input.TaskUseCase
	presenter       output.Presenter
}

// NewWorkflowController creates a new workflow controller
func NewWorkflowController(
	workflowUC input.WorkflowUseCase,
	taskUC input.TaskUseCase,
	presenter output.Presenter,
) *WorkflowController {
	return &WorkflowController{
		workflowUseCase: workflowUC,
		taskUseCase:     taskUC,
		presenter:       presenter,
	}
}

// ImplementCommand creates 'workflow implement' command
func (c *WorkflowController) ImplementCommand() *cobra.Command {
	var (
		agentType   string
		maxTokens   int
		temperature float64
		timeout     time.Duration
	)

	cmd := &cobra.Command{
		Use:   "implement [task-id]",
		Short: "Implement a task using AI agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			req := dto.ImplementTaskRequest{
				TaskID: taskID,
			}

			if err := c.presenter.PresentProgress("Starting task implementation...", 0, 0); err != nil {
				return err
			}

			result, err := c.workflowUseCase.ImplementTask(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("Task implemented", result)
		},
	}

	cmd.Flags().StringVar(&agentType, "agent", "", "Agent type (claude-code, gemini-cli, codex)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 4096, "Maximum tokens for agent response")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.7, "Agent temperature (0.0-1.0)")
	cmd.Flags().DurationVar(&timeout, "timeout", 5*time.Minute, "Implementation timeout")

	return cmd
}

// ReviewCommand creates 'workflow review' command
func (c *WorkflowController) ReviewCommand() *cobra.Command {
	var (
		agentType   string
		maxTokens   int
		temperature float64
		timeout     time.Duration
		autoApprove bool
	)

	cmd := &cobra.Command{
		Use:   "review [task-id]",
		Short: "Review a task implementation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			req := dto.ReviewTaskRequest{
				TaskID:   taskID,
				Approved: autoApprove,
			}

			if err := c.presenter.PresentProgress("Reviewing task implementation...", 0, 0); err != nil {
				return err
			}

			result, err := c.workflowUseCase.ReviewTask(cmd.Context(), req)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("Task reviewed", result)
		},
	}

	cmd.Flags().StringVar(&agentType, "agent", "", "Agent type for review")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 2048, "Maximum tokens for review response")
	cmd.Flags().Float64Var(&temperature, "temperature", 0.3, "Agent temperature")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Minute, "Review timeout")
	cmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Automatically approve if review passes")

	return cmd
}

// CompleteCommand creates 'workflow complete' command
func (c *WorkflowController) CompleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "complete [task-id]",
		Short: "Mark a task as completed",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			err := c.workflowUseCase.CompleteTask(cmd.Context(), taskID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("Task marked as completed", nil)
		},
	}
}

// PickCommand creates 'workflow pick' command
func (c *WorkflowController) PickCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "pick [task-id]",
		Short: "Pick a task for work",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			err := c.workflowUseCase.PickTask(cmd.Context(), taskID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			// Fetch and display the picked task
			task, err := c.taskUseCase.GetTask(cmd.Context(), taskID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("Task picked for work", task)
		},
	}
}

// RunCommand creates 'workflow run' command for automated workflow execution
func (c *WorkflowController) RunCommand() *cobra.Command {
	var (
		agentType   string
		maxTurns    int
		maxAttempts int
		autoReview  bool
		autoApprove bool
	)

	cmd := &cobra.Command{
		Use:   "run [task-id]",
		Short: "Run full workflow for a task (pick -> implement -> review -> complete)",
		Long: `Executes the complete workflow for a task:
1. Pick the task
2. Implement using AI agent
3. Review the implementation
4. Complete if review passes (with --auto-approve)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			// Step 1: Pick task
			if err := c.presenter.PresentProgress("Picking task...", 0, 0); err != nil {
				return err
			}
			if err := c.workflowUseCase.PickTask(cmd.Context(), taskID); err != nil {
				return c.presenter.PresentError(err)
			}

			// Step 2: Implement task
			if err := c.presenter.PresentProgress("Implementing task...", 0, 0); err != nil {
				return err
			}
			implReq := dto.ImplementTaskRequest{
				TaskID: taskID,
			}
			implResult, err := c.workflowUseCase.ImplementTask(cmd.Context(), implReq)
			if err != nil {
				return c.presenter.PresentError(err)
			}
			if err := c.presenter.PresentSuccess("Implementation complete", implResult); err != nil {
				return err
			}

			// Step 3: Review task (if enabled)
			if autoReview {
				if err := c.presenter.PresentProgress("Reviewing implementation...", 0, 0); err != nil {
					return err
				}
				reviewReq := dto.ReviewTaskRequest{
					TaskID:   taskID,
					Approved: autoApprove,
				}
				reviewResult, err := c.workflowUseCase.ReviewTask(cmd.Context(), reviewReq)
				if err != nil {
					return c.presenter.PresentError(err)
				}
				if err := c.presenter.PresentSuccess("Review complete", reviewResult); err != nil {
					return err
				}
			}

			// Step 4: Complete task (if auto-approve enabled and review passed)
			if autoApprove {
				if err := c.presenter.PresentProgress("Completing task...", 0, 0); err != nil {
					return err
				}
				if err := c.workflowUseCase.CompleteTask(cmd.Context(), taskID); err != nil {
					return c.presenter.PresentError(err)
				}
				return c.presenter.PresentSuccess("Workflow completed successfully", nil)
			}

			return c.presenter.PresentSuccess("Workflow execution complete (manual approval required)", nil)
		},
	}

	cmd.Flags().StringVar(&agentType, "agent", "", "Agent type (claude-code, gemini-cli, codex)")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 3, "Maximum number of implementation turns")
	cmd.Flags().IntVar(&maxAttempts, "max-attempts", 2, "Maximum attempts per turn")
	cmd.Flags().BoolVar(&autoReview, "auto-review", true, "Automatically review after implementation")
	cmd.Flags().BoolVar(&autoApprove, "auto-approve", false, "Automatically approve and complete if review passes")

	return cmd
}

// StatusCommand creates 'workflow status' command
func (c *WorkflowController) StatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status [task-id]",
		Short: "Get workflow execution status for a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			taskID := args[0]

			task, err := c.taskUseCase.GetTask(cmd.Context(), taskID)
			if err != nil {
				return c.presenter.PresentError(err)
			}

			return c.presenter.PresentSuccess("Workflow status", task)
		},
	}
}

// BuildCommand creates the 'workflow' parent command with all subcommands
func (c *WorkflowController) BuildCommand() *cobra.Command {
	workflowCmd := &cobra.Command{
		Use:   "workflow",
		Short: "Workflow execution commands",
		Long:  "Commands for executing and managing task workflows (pick, implement, review, complete)",
	}

	workflowCmd.AddCommand(
		c.PickCommand(),
		c.ImplementCommand(),
		c.ReviewCommand(),
		c.CompleteCommand(),
		c.RunCommand(),
		c.StatusCommand(),
	)

	return workflowCmd
}
