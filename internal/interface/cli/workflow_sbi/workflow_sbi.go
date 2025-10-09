package workflow_sbi

import (
	"context"
	"errors"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/application/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

// SBIWorkflowRunner implements WorkflowRunner for SBI processing
type SBIWorkflowRunner struct {
	enabled bool
}

// NewSBIWorkflowRunner creates a new SBI workflow runner
func NewSBIWorkflowRunner() *SBIWorkflowRunner {
	return &SBIWorkflowRunner{
		enabled: true,
	}
}

// Name returns the workflow name
func (swr *SBIWorkflowRunner) Name() string {
	return "sbi"
}

// Description returns a human-readable description
func (swr *SBIWorkflowRunner) Description() string {
	return "Spec Backlog Item processing workflow"
}

// IsEnabled checks if the workflow should be executed
func (swr *SBIWorkflowRunner) IsEnabled() bool {
	return swr.enabled
}

// SetEnabled sets the enabled state of the workflow
func (swr *SBIWorkflowRunner) SetEnabled(enabled bool) {
	swr.enabled = enabled
}

// Run executes one cycle of the SBI workflow
func (swr *SBIWorkflowRunner) Run(ctx context.Context, config workflow.WorkflowConfig) error {
	// Check for cancellation before starting
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Extract AutoFB setting from config
	autoFB := config.AutoFB

	// Check global config for auto-fb (config takes precedence over flag)
	if common.GetGlobalConfig() != nil && common.GetGlobalConfig().AutoFB() {
		autoFB = true
	}

	// In test environment, return a test error to avoid calling runOnce
	// which requires full application context
	if isTestEnvironment() {
		return errors.New("test environment: SBI workflow runner execution skipped")
	}

	// TODO: Implement SBI workflow execution without circular dependency
	// This workflow runner is currently not used in the main execution path
	// The main execution uses workflow.WorkflowManager directly
	_ = autoFB // Will be used when implementation is added
	return errors.New("SBI workflow runner: not implemented (use 'deespec run' instead)")
}

// Validate checks if the workflow can be executed
func (swr *SBIWorkflowRunner) Validate() error {
	// Add any SBI-specific validation here
	// For example, check if required directories exist, etc.
	return nil
}

// isTestEnvironment checks if we're running in a test environment
func isTestEnvironment() bool {
	// Check for common test environment indicators
	for _, arg := range os.Args {
		if arg == "-test.v" || arg == "-test.run" ||
			(len(arg) > 6 && arg[:6] == "-test.") {
			return true
		}
	}
	return false
}
