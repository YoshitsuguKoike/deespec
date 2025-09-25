package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadWorkflow loads and validates a workflow from the specified path
func LoadWorkflow(ctx context.Context, wfPath string) (*Workflow, error) {
	// Read workflow file
	data, err := os.ReadFile(wfPath)
	if err != nil {
		return nil, fmt.Errorf("workflow: read: %w", err)
	}

	// Check for deprecated "prompt" field
	if err := checkDeprecatedPromptField(data); err != nil {
		return nil, err
	}

	// Parse YAML
	var wf Workflow
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("workflow: parse: %w", err)
	}

	// Validate schema
	if err := validateWorkflow(&wf); err != nil {
		return nil, err
	}

	return &wf, nil
}

// checkDeprecatedPromptField checks if the deprecated "prompt" field exists
func checkDeprecatedPromptField(data []byte) error {
	// Parse as generic YAML to check for deprecated fields
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil // If it doesn't parse as map, let the structured parsing handle it
	}

	// Check steps
	if stepsRaw, ok := raw["steps"]; ok {
		if steps, ok := stepsRaw.([]interface{}); ok {
			for i, stepRaw := range steps {
				if step, ok := stepRaw.(map[string]interface{}); ok {
					if _, hasPrompt := step["prompt"]; hasPrompt {
						return fmt.Errorf(`workflow.steps[%d]: "prompt" is not allowed (use prompt_path)`, i)
					}
				}
			}
		}
	}

	return nil
}

// validateWorkflow performs schema validation on the workflow
func validateWorkflow(wf *Workflow) error {
	// Validate name
	if strings.TrimSpace(wf.Name) == "" {
		return errors.New(`workflow: "name" is required`)
	}

	// Validate steps
	if len(wf.Steps) == 0 {
		return errors.New(`workflow: "steps" must be a non-empty array`)
	}

	// Track seen IDs for duplicate detection
	seen := make(map[string]struct{})

	// Validate each step
	for i, step := range wf.Steps {
		idx := fmt.Sprintf("workflow.steps[%d]", i)

		// Validate ID
		if strings.TrimSpace(step.ID) == "" {
			return fmt.Errorf(`%s: "id" is required`, idx)
		}

		// Check for duplicate ID
		if _, exists := seen[step.ID]; exists {
			return fmt.Errorf(`%s: duplicate id "%s"`, idx, step.ID)
		}
		seen[step.ID] = struct{}{}

		// Validate agent
		if strings.TrimSpace(step.Agent) == "" {
			return fmt.Errorf(`%s: "agent" is required`, idx)
		}

		// Validate agent is supported
		switch step.Agent {
		case "claude_cli", "system":
			// Supported agents
		default:
			return fmt.Errorf(`%s: unsupported agent "%s"`, idx, step.Agent)
		}

		// Validate prompt_path
		if strings.TrimSpace(step.PromptPath) == "" {
			return fmt.Errorf(`%s: "prompt_path" is required`, idx)
		}
	}

	return nil
}