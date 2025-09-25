package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadWorkflow(t *testing.T) {
	// Set DEE_HOME for testing
	t.Setenv("DEE_HOME", ".deespec")

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
			name: "review step with decision regex",
			yaml: `name: test
steps:
  - id: review
    agent: claude_cli
    prompt_path: prompts/system/review.md
    decision:
      regex: "^DECISION:\\s+(OK|NEEDS_CHANGES)\\s*$"`,
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
			name: "bash agent not supported",
			yaml: `name: test
steps:
  - id: plan
    agent: bash
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow.steps[0]: unsupported agent "bash" (allowed=`,
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
		{
			name: "absolute path not allowed",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: /etc/passwd`,
			wantErr: `workflow.steps[0]: "prompt_path" must be relative to .deespec`,
		},
		{
			name: "parent directory reference not allowed",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: ../outside.md`,
			wantErr: `workflow.steps[0]: "prompt_path" must not contain ".."`,
		},
		{
			name: "parent directory in middle not allowed",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/../../../etc/passwd`,
			wantErr: `workflow.steps[0]: "prompt_path" must not contain ".."`,
		},
		{
			name: "unknown field in workflow",
			yaml: `name: test
foo: bar
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md`,
			wantErr: `workflow: parse:`,
		},
		{
			name: "unknown field in step",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
    unknown_field: value`,
			wantErr: `workflow: parse:`,
		},
		{
			name: "workflow with vars field is valid",
			yaml: `name: test
vars:
  project_name: myproject
  language: en
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md`,
			wantErr: "",
		},
		{
			name: "decision on review step with default regex",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
  - id: review
    agent: claude_cli
    prompt_path: prompts/system/review.md`,
			wantErr: "",
		},
		{
			name: "decision on review step with custom regex",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
  - id: review
    agent: claude_cli
    prompt_path: prompts/system/review.md
    decision:
      regex: "^REVIEW_RESULT:\\s*(OK|NEEDS_CHANGES)\\s*$"`,
			wantErr: "",
		},
		{
			name: "decision on non-review step is not allowed",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
    decision:
      regex: "^DECISION:\\s+(OK|NEEDS_CHANGES)\\s*$"`,
			wantErr: `workflow.steps[0]: decision is only allowed on step id "review"`,
		},
		{
			name: "invalid regex in decision",
			yaml: `name: test
steps:
  - id: review
    agent: system
    prompt_path: prompts/system/review.md
    decision:
      regex: "("`,
			wantErr: `workflow.steps[0]: decision.regex compile failed:`,
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

func TestResolvePromptPath(t *testing.T) {
	// Set DEE_HOME for testing
	t.Setenv("DEE_HOME", ".deespec")

	tests := []struct {
		name          string
		yaml          string
		wantResolved  []string
		checkResolved bool
	}{
		{
			name: "simple relative path resolution",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md`,
			wantResolved:  []string{".deespec/prompts/system/plan.md"},
			checkResolved: true,
		},
		{
			name: "multiple paths resolution",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
  - id: impl
    agent: claude_cli
    prompt_path: prompts/system/implement.md`,
			wantResolved:  []string{".deespec/prompts/system/plan.md", ".deespec/prompts/system/implement.md"},
			checkResolved: true,
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
			if err != nil {
				t.Fatalf("LoadWorkflow() error = %v", err)
			}

			// Check resolved paths
			if tt.checkResolved {
				if len(wf.Steps) != len(tt.wantResolved) {
					t.Fatalf("Expected %d steps, got %d", len(tt.wantResolved), len(wf.Steps))
				}
				for i, step := range wf.Steps {
					if !strings.HasSuffix(step.ResolvedPromptPath, tt.wantResolved[i]) {
						t.Errorf("Step[%d].ResolvedPromptPath = %q, want suffix %q",
							i, step.ResolvedPromptPath, tt.wantResolved[i])
					}
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