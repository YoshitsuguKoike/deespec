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
func ExpandStepPrompt(ctx context.Context, step workflow.Step, vars map[string]string) (*ExpandedPrompt, error) {
	// Read the prompt file
	rawContent, err := os.ReadFile(step.ResolvedPromptPath)
	if err != nil {
		return nil, fmt.Errorf("read prompt file %s: %w", step.ResolvedPromptPath, err)
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

	// Expand prompts for all steps
	var prompts []*ExpandedPrompt
	for _, step := range wf.Steps {
		prompt, err := ExpandStepPrompt(ctx, step, vars)
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

	// Expand the prompt
	return ExpandStepPrompt(ctx, *targetStep, vars)
}