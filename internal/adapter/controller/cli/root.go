package cli

import (
	"github.com/YoshitsuguKoike/deespec/internal/application/port/input"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/spf13/cobra"
)

// RootBuilder builds the root CLI command with all subcommands
type RootBuilder struct {
	// Use cases
	taskUseCase         input.TaskUseCase
	workflowUseCase     input.WorkflowUseCase
	epicWorkflowUseCase input.EPICWorkflowUseCase
	pbiWorkflowUseCase  input.PBIWorkflowUseCase
	sbiWorkflowUseCase  input.SBIWorkflowUseCase

	// Output
	presenter output.Presenter

	// Version info
	version   string
	buildInfo string
}

// NewRootBuilder creates a new root command builder
func NewRootBuilder(
	taskUC input.TaskUseCase,
	workflowUC input.WorkflowUseCase,
	epicWorkflowUC input.EPICWorkflowUseCase,
	pbiWorkflowUC input.PBIWorkflowUseCase,
	sbiWorkflowUC input.SBIWorkflowUseCase,
	presenter output.Presenter,
	version string,
	buildInfo string,
) *RootBuilder {
	return &RootBuilder{
		taskUseCase:         taskUC,
		workflowUseCase:     workflowUC,
		epicWorkflowUseCase: epicWorkflowUC,
		pbiWorkflowUseCase:  pbiWorkflowUC,
		sbiWorkflowUseCase:  sbiWorkflowUC,
		presenter:           presenter,
		version:             version,
		buildInfo:           buildInfo,
	}
}

// Build creates the root command with all subcommands
func (b *RootBuilder) Build() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "deespec",
		Short: "DeeSpec - Spec Backlog Item Management System",
		Long: `DeeSpec is a task management system for EPIC, PBI, and SBI tasks.
It provides AI-powered task decomposition, implementation, and review workflows.`,
		Version: b.version,
	}

	// Add global flags
	var outputFormat string
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "cli", "Output format (cli, json)")

	// Create controllers
	epicController := NewEPICController(
		b.taskUseCase,
		b.epicWorkflowUseCase,
		b.workflowUseCase,
		b.presenter,
	)

	pbiController := NewPBIController(
		b.taskUseCase,
		b.pbiWorkflowUseCase,
		b.workflowUseCase,
		b.presenter,
	)

	sbiController := NewSBIController(
		b.taskUseCase,
		b.sbiWorkflowUseCase,
		b.workflowUseCase,
		b.presenter,
	)

	workflowController := NewWorkflowController(
		b.workflowUseCase,
		b.taskUseCase,
		b.presenter,
	)

	// Add subcommands
	rootCmd.AddCommand(
		epicController.BuildCommand(),
		pbiController.BuildCommand(),
		sbiController.BuildCommand(),
		workflowController.BuildCommand(),
		b.versionCommand(),
	)

	return rootCmd
}

// versionCommand creates the 'version' command
func (b *RootBuilder) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			versionInfo := map[string]string{
				"version":   b.version,
				"buildInfo": b.buildInfo,
			}
			return b.presenter.PresentSuccess("DeeSpec Version", versionInfo)
		},
	}
}
