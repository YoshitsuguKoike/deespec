package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestConstraintsNormalization(t *testing.T) {
	tests := []struct {
		name            string
		yaml            string
		wantMaxPromptKB int
	}{
		{
			name: "no constraints uses default",
			yaml: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: DefaultMaxPromptKB,
		},
		{
			name: "valid constraint value",
			yaml: `name: test
constraints:
  max_prompt_kb: 128
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: 128,
		},
		{
			name: "zero falls back to default",
			yaml: `name: test
constraints:
  max_prompt_kb: 0
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: DefaultMaxPromptKB,
		},
		{
			name: "negative falls back to default",
			yaml: `name: test
constraints:
  max_prompt_kb: -50
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: DefaultMaxPromptKB,
		},
		{
			name: "over limit falls back to default",
			yaml: `name: test
constraints:
  max_prompt_kb: 1024
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: DefaultMaxPromptKB,
		},
		{
			name: "boundary value 1",
			yaml: `name: test
constraints:
  max_prompt_kb: 1
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: 1,
		},
		{
			name: "boundary value 512",
			yaml: `name: test
constraints:
  max_prompt_kb: 512
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: 512,
		},
		{
			name: "boundary value 513 falls back",
			yaml: `name: test
constraints:
  max_prompt_kb: 513
steps:
  - id: plan
    agent: system
    prompt_path: test.md`,
			wantMaxPromptKB: DefaultMaxPromptKB,
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

			// Check normalized value
			if wf.Constraints.MaxPromptKB != tt.wantMaxPromptKB {
				t.Errorf("MaxPromptKB = %d, want %d", wf.Constraints.MaxPromptKB, tt.wantMaxPromptKB)
			}
		})
	}
}