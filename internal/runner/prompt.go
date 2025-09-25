package runner

import (
	"context"
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/app/state"
	"github.com/YoshitsuguKoike/deespec/internal/workflow"
)

// ExpandedPrompt represents a prompt after variable expansion
type ExpandedPrompt struct {
	StepID   string
	Original string // Original prompt with placeholders
	Expanded string // Expanded prompt with variables replaced
	Vars     map[string]string
}

// ExpandStepPrompt reads and expands a single step's prompt
func ExpandStepPrompt(ctx context.Context, step workflow.Step, vars map[string]string, limitKB int) (*ExpandedPrompt, error) {
	// Read the prompt file with size limit
	rawContent, err := ReadPromptWithLimit(step.ResolvedPromptPath, limitKB)
	if err != nil {
		return nil, err
	}

	original := string(rawContent)

	// Expand the prompt with variables
	expanded, err := workflow.ExpandPrompt(original, vars)
	if err != nil {
		return nil, fmt.Errorf("expand prompt for step %s: %w", step.ID, err)
	}

	return &ExpandedPrompt{
		StepID:   step.ID,
		Original: original,
		Expanded: expanded,
		Vars:     vars,
	}, nil
}

// ExpandWorkflowPrompts expands all prompts in a workflow
func ExpandWorkflowPrompts(ctx context.Context, wf *workflow.Workflow, paths app.Paths) ([]*ExpandedPrompt, error) {
	// Load state if it exists
	st, err := state.LoadState(paths.State)
	if err != nil {
		// State might not exist yet, use nil
		st = nil
	}

	// Build the variable map
	vars := workflow.BuildVarMap(ctx, paths, wf.Vars, st)

	// Get the size limit from workflow constraints
	limitKB := wf.Constraints.MaxPromptKB
	if limitKB <= 0 {
		limitKB = workflow.DefaultMaxPromptKB
	}

	// Expand prompts for all steps
	var prompts []*ExpandedPrompt
	for _, step := range wf.Steps {
		prompt, err := ExpandStepPrompt(ctx, step, vars, limitKB)
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, prompt)
	}

	return prompts, nil
}

// PrepareStepExecution prepares a step for execution by expanding its prompt
func PrepareStepExecution(ctx context.Context, wf *workflow.Workflow, stepID string, paths app.Paths) (*ExpandedPrompt, error) {
	// Find the step
	var targetStep *workflow.Step
	for _, step := range wf.Steps {
		if step.ID == stepID {
			targetStep = &step
			break
		}
	}

	if targetStep == nil {
		return nil, fmt.Errorf("step %s not found in workflow", stepID)
	}

	// Load state if it exists
	st, err := state.LoadState(paths.State)
	if err != nil {
		// State might not exist yet, use nil
		st = nil
	}

	// Build the variable map
	vars := workflow.BuildVarMap(ctx, paths, wf.Vars, st)

	// Get the size limit from workflow constraints
	limitKB := wf.Constraints.MaxPromptKB
	if limitKB <= 0 {
		limitKB = workflow.DefaultMaxPromptKB
	}

	// Expand the prompt
	return ExpandStepPrompt(ctx, *targetStep, vars, limitKB)
}

// ReadPromptWithLimit reads a prompt file with size limit enforcement
func ReadPromptWithLimit(path string, limitKB int) ([]byte, error) {
	// Get file info to check size
	fi, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("prompt stat: %w", err)
	}

	// Check if file size exceeds limit
	limitBytes := int64(limitKB * 1024)
	if fi.Size() > limitBytes {
		// Calculate size in KB (rounded up)
		sizeKB := (fi.Size() + 1023) / 1024
		return nil, fmt.Errorf("prompt file too large (size=%dKB, limit=%dKB): %s", sizeKB, limitKB, path)
	}

	// Read the file
	return os.ReadFile(path)
}