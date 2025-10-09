package workflow

import "context"

// WorkflowRunner defines the interface for workflow execution
type WorkflowRunner interface {
	// Name returns the workflow name (e.g., "sbi", "pbi")
	Name() string

	// Run executes one cycle of the workflow
	Run(ctx context.Context, config WorkflowConfig) error

	// IsEnabled checks if the workflow should be executed
	IsEnabled() bool

	// Description returns a human-readable description
	Description() string
}
