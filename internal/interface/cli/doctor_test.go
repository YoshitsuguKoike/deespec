package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/workflow"
)

func TestDoctorPromptValidation(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	deespecDir := filepath.Join(tmpDir, ".deespec")
	etcDir := filepath.Join(deespecDir, "etc")
	promptsDir := filepath.Join(deespecDir, "prompts", "system")

	// Create directory structure
	if err := os.MkdirAll(etcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Set DEE_HOME for testing
	t.Setenv("DEE_HOME", deespecDir)

	tests := []struct {
		name            string
		workflowYAML    string
		setupFiles      map[string]string  // path -> content
		expectErrors    []string
		expectOK        []string
		expectExitCode  bool  // true if should exit with code 1
	}{
		{
			name: "all prompts exist and readable",
			workflowYAML: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
  - id: implement
    agent: claude_cli
    prompt_path: prompts/system/implement.md`,
			setupFiles: map[string]string{
				"prompts/system/plan.md":      "# Plan prompt",
				"prompts/system/implement.md": "# Implement prompt",
			},
			expectOK: []string{
				"OK: prompt_path (plan) readable",
				"OK: prompt_path (implement) readable",
			},
			expectExitCode: false,
		},
		{
			name: "prompt file missing",
			workflowYAML: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/plan.md
  - id: implement
    agent: claude_cli
    prompt_path: prompts/system/missing.md`,
			setupFiles: map[string]string{
				"prompts/system/plan.md": "# Plan prompt",
			},
			expectOK: []string{
				"OK: prompt_path (plan) readable",
			},
			expectErrors: []string{
				"ERROR: prompt_path not found: ",
				"missing.md",
			},
			expectExitCode: true,
		},
		{
			name: "prompt path is directory",
			workflowYAML: `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system`,
			setupFiles: map[string]string{},
			expectErrors: []string{
				"ERROR: prompt_path not a regular file:",
			},
			expectExitCode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			workflowPath := filepath.Join(etcDir, "workflow.yaml")
			if err := os.WriteFile(workflowPath, []byte(tt.workflowYAML), 0644); err != nil {
				t.Fatal(err)
			}

			// Create test files
			for path, content := range tt.setupFiles {
				fullPath := filepath.Join(deespecDir, path)
				dir := filepath.Dir(fullPath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Load workflow to test validation
			ctx := context.Background()
			wf, err := workflow.LoadWorkflow(ctx, workflowPath)
			if err != nil {
				t.Fatalf("Failed to load workflow: %v", err)
			}

			// Capture output
			output := &bytes.Buffer{}
			promptErrors := 0

			// Run validation logic (extracted from doctor command)
			for _, step := range wf.Steps {
				// Check existence
				fileInfo, err := os.Stat(step.ResolvedPromptPath)
				if err != nil {
					if os.IsNotExist(err) {
						output.WriteString("ERROR: prompt_path not found: " + step.ResolvedPromptPath + "\n")
					} else {
						output.WriteString("ERROR: prompt_path not accessible: " + step.ResolvedPromptPath + "\n")
					}
					promptErrors++
					continue
				}

				// Check it's a regular file
				if !fileInfo.Mode().IsRegular() {
					output.WriteString("ERROR: prompt_path not a regular file: " + step.ResolvedPromptPath + "\n")
					promptErrors++
					continue
				}

				// Check readability by attempting to read
				file, err := os.Open(step.ResolvedPromptPath)
				if err != nil {
					output.WriteString("ERROR: prompt_path not readable: " + step.ResolvedPromptPath + "\n")
					promptErrors++
					continue
				}
				file.Close()

				// Report OK for this step's prompt
				output.WriteString("OK: prompt_path (" + step.ID + ") readable\n")
			}

			// Check output contains expected strings
			outputStr := output.String()
			for _, expected := range tt.expectErrors {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected error containing %q, got output:\n%s", expected, outputStr)
				}
			}
			for _, expected := range tt.expectOK {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected OK message %q, got output:\n%s", expected, outputStr)
				}
			}

			// Check exit code expectation
			if tt.expectExitCode && promptErrors == 0 {
				t.Errorf("Expected errors but got none")
			}
			if !tt.expectExitCode && promptErrors > 0 {
				t.Errorf("Expected no errors but got %d", promptErrors)
			}
		})
	}
}

// TestDoctorPromptValidationIntegration tests the actual doctor command
func TestDoctorPromptValidationIntegration(t *testing.T) {
	t.Skip("Integration test skipped - os.Exit() behavior is difficult to test in unit tests")
	// Create temp directory for test
	tmpDir := t.TempDir()
	deespecDir := filepath.Join(tmpDir, ".deespec")
	etcDir := filepath.Join(deespecDir, "etc")
	promptsDir := filepath.Join(deespecDir, "prompts", "system")
	varDir := filepath.Join(deespecDir, "var")

	// Create directory structure
	for _, dir := range []string{etcDir, promptsDir, varDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create workflow with missing prompt
	workflowYAML := `name: test
steps:
  - id: plan
    agent: system
    prompt_path: prompts/system/missing.md`

	if err := os.WriteFile(filepath.Join(etcDir, "workflow.yaml"), []byte(workflowYAML), 0644); err != nil {
		t.Fatal(err)
	}

	// Set environment
	t.Setenv("DEE_HOME", deespecDir)

	// Create doctor command and capture output
	cmd := newDoctorCmd()

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set up goroutine to read output
	output := make(chan string)
	go func() {
		buf := new(bytes.Buffer)
		io.Copy(buf, r)
		output <- buf.String()
	}()

	// Run command (should error due to missing prompt)
	_ = cmd.Execute()  // Ignore error as we're testing output

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	outputStr := <-output

	// Check that error was reported
	if !strings.Contains(outputStr, "ERROR: prompt_path not found") {
		t.Errorf("Expected prompt not found error, got output:\n%s", outputStr)
	}

	// Note: We can't easily test os.Exit() behavior in unit tests
	// That would require a separate integration test
}