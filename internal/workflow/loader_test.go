package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWorkflow(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "minimal valid workflow",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md`,
			wantErr: "",
		},
		{
			name: "with decision regex",
			yaml: `name: test
steps:
  - id: plan
    agent: claude_cli
    prompt_path: prompts/system/plan.md
    decision:
      regex: "^(yes|no)$"`,
			wantErr: "",
		},
		{
			name: "missing name",
			yaml: `steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow: "name" is required`,
		},
		{
			name:    "missing steps",
			yaml:    `name: test`,
			wantErr: `workflow: "steps" must be a non-empty array`,
		},
		{
			name: "empty steps array",
			yaml: `name: test
steps: []`,
			wantErr: `workflow: "steps" must be a non-empty array`,
		},
		{
			name: "missing step id",
			yaml: `name: test
steps:
  - agent: system
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow.steps[0]: "id" is required`,
		},
		{
			name: "missing agent",
			yaml: `name: test
steps:
  - id: plan
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow.steps[0]: "agent" is required`,
		},
		{
			name: "missing prompt_path",
			yaml: `name: test
steps:
  - id: plan
    agent: system`,
			wantErr: `workflow.steps[0]: "prompt_path" is required`,
		},
		{
			name: "deprecated prompt field",
			yaml: `name: test
steps:
  - id: plan
    agent: claude_cli
    prompt: "This is the old prompt field"`,
			wantErr: `workflow.steps[0]: "prompt" is not allowed (use prompt_path)`,
		},
		{
			name: "unsupported agent",
			yaml: `name: test
steps:
  - id: plan
    agent: unsupported_agent
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow.steps[0]: unsupported agent "unsupported_agent"`,
		},
		{
			name: "duplicate step id",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
  - id: plan
    agent: claude_cli
    prompt_path: prompts/system/implement.md`,
			wantErr: `workflow.steps[1]: duplicate id "plan"`,
		},
		{
			name: "empty name",
			yaml: `name: "  "
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow: "name" is required`,
		},
		{
			name: "empty step id",
			yaml: `name: test
steps:
  - id: "  "
    agent: system
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow.steps[0]: "id" is required`,
		},
		{
			name: "empty agent",
			yaml: `name: test
steps:
  - id: plan
    agent: "  "
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow.steps[0]: "agent" is required`,
		},
		{
			name: "empty prompt_path",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: "  "`,
			wantErr: `workflow.steps[0]: "prompt_path" is required`,
		},
		{
			name: "multiple steps valid",
			yaml: `name: workflow-v1
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
  - id: implement
    agent: claude_cli
    prompt_path: prompts/system/implement.md
  - id: test
    agent: system
    prompt_path: prompts/system/test.md`,
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with YAML content
			tmpDir := t.TempDir()
			wfPath := filepath.Join(tmpDir, "workflow.yaml")
			if err := os.WriteFile(wfPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}

			// Load workflow
			ctx := context.Background()
			wf, err := LoadWorkflow(ctx, wfPath)

			// Check error
			if tt.wantErr != "" {
				if err == nil {
					t.Errorf("LoadWorkflow() expected error %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("LoadWorkflow() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("LoadWorkflow() unexpected error = %v", err)
				}
				if wf == nil {
					t.Error("LoadWorkflow() returned nil workflow without error")
				}
			}
		})
	}
}

func TestCheckDeprecatedPromptField(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "no prompt field",
			yaml: `steps:
  - id: plan
    prompt_path: test.md`,
			wantErr: false,
		},
		{
			name: "has prompt field in first step",
			yaml: `steps:
  - id: plan
    prompt: "old style"`,
			wantErr: true,
		},
		{
			name: "has prompt field in second step",
			yaml: `steps:
  - id: plan
    prompt_path: test.md
  - id: impl
    prompt: "old style"`,
			wantErr: true,
		},
		{
			name:    "empty yaml",
			yaml:    ``,
			wantErr: false,
		},
		{
			name:    "no steps",
			yaml:    `name: test`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDeprecatedPromptField([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("checkDeprecatedPromptField() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}