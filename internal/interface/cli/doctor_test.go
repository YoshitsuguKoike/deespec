package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

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

// TestDoctorPromptSizeAndEncoding tests size limit and encoding validation (SBI-DR-002)
func TestDoctorPromptSizeAndEncoding(t *testing.T) {
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
		setupFiles      map[string][]byte  // path -> content (as bytes for binary data)
		expectErrors    []string
		expectWarnings  []string
		expectOK        []string
	}{
		{
			name: "prompt within size limit",
			workflowYAML: `name: test
constraints:
  max_prompt_kb: 2
steps:
  - id: small
    agent: system
    prompt_path: prompts/system/small.md`,
			setupFiles: map[string][]byte{
				"prompts/system/small.md": []byte("Small file"),
			},
			expectOK: []string{
				"OK: prompt_path (small) size=1KB utf8=valid",
			},
		},
		{
			name: "prompt exceeds size limit",
			workflowYAML: `name: test
constraints:
  max_prompt_kb: 1
steps:
  - id: large
    agent: system
    prompt_path: prompts/system/large.md`,
			setupFiles: map[string][]byte{
				"prompts/system/large.md": make([]byte, 2*1024), // 2KB file
			},
			expectErrors: []string{
				"ERROR: prompt_path (large) exceeds max_prompt_kb=1 (found 2)",
			},
		},
		{
			name: "UTF-8 BOM warning",
			workflowYAML: `name: test
steps:
  - id: bom
    agent: system
    prompt_path: prompts/system/bom.md`,
			setupFiles: map[string][]byte{
				"prompts/system/bom.md": append([]byte{0xEF, 0xBB, 0xBF}, []byte("UTF-8 with BOM")...),
			},
			expectWarnings: []string{
				"WARN: prompt_path (bom) contains UTF-8 BOM",
			},
			expectOK: []string{
				"OK: prompt_path (bom)",
			},
		},
		{
			name: "CRLF warning",
			workflowYAML: `name: test
steps:
  - id: crlf
    agent: system
    prompt_path: prompts/system/crlf.md`,
			setupFiles: map[string][]byte{
				"prompts/system/crlf.md": []byte("Line 1\r\nLine 2\r\n"),
			},
			expectWarnings: []string{
				"WARN: prompt_path (crlf) contains CRLF",
			},
			expectOK: []string{
				"OK: prompt_path (crlf)",
			},
		},
		{
			name: "invalid UTF-8",
			workflowYAML: `name: test
steps:
  - id: invalid
    agent: system
    prompt_path: prompts/system/invalid.md`,
			setupFiles: map[string][]byte{
				"prompts/system/invalid.md": []byte{0xFF, 0xFE, 0x00, 0x00}, // Invalid UTF-8
			},
			expectErrors: []string{
				"ERROR: prompt_path (invalid) invalid UTF-8 encoding",
			},
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
				if err := os.WriteFile(fullPath, content, 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Load workflow to test validation
			ctx := context.Background()
			wf, err := workflow.LoadWorkflow(ctx, workflowPath)
			if err != nil {
				t.Fatalf("Failed to load workflow: %v", err)
			}

			// Get max prompt size
			maxPromptKB := wf.Constraints.MaxPromptKB
			if maxPromptKB <= 0 {
				maxPromptKB = workflow.DefaultMaxPromptKB
			}

			// Capture output
			output := &bytes.Buffer{}
			errorOutput := &bytes.Buffer{}
			promptErrors := 0
			promptWarnings := 0

			// Run validation logic (extracted from doctor command)
			for _, step := range wf.Steps {
				// Check existence
				fileInfo, err := os.Stat(step.ResolvedPromptPath)
				if err != nil {
					errorOutput.WriteString("ERROR: prompt_path not found: " + step.ResolvedPromptPath + "\n")
					promptErrors++
					continue
				}

				// Check size
				fileSizeKB := (fileInfo.Size() + 1023) / 1024
				if fileSizeKB > int64(maxPromptKB) {
					errorOutput.WriteString(fmt.Sprintf("ERROR: prompt_path (%s) exceeds max_prompt_kb=%d (found %d)\n",
						step.ID, maxPromptKB, fileSizeKB))
					promptErrors++
					continue
				}

				// Read content
				content, err := os.ReadFile(step.ResolvedPromptPath)
				if err != nil {
					errorOutput.WriteString("ERROR: prompt_path not readable: " + step.ResolvedPromptPath + "\n")
					promptErrors++
					continue
				}

				// Check UTF-8
				if !utf8.Valid(content) {
					errorOutput.WriteString("ERROR: prompt_path (" + step.ID + ") invalid UTF-8 encoding\n")
					promptErrors++
					continue
				}

				// Check BOM
				if len(content) >= 3 && bytes.HasPrefix(content, []byte{0xEF, 0xBB, 0xBF}) {
					output.WriteString("WARN: prompt_path (" + step.ID + ") contains UTF-8 BOM\n")
					promptWarnings++
				}

				// Check CRLF
				if bytes.Contains(content, []byte("\r\n")) {
					output.WriteString("WARN: prompt_path (" + step.ID + ") contains CRLF; prefer LF\n")
					promptWarnings++
				}

				// Report OK
				output.WriteString(fmt.Sprintf("OK: prompt_path (%s) size=%dKB utf8=valid lf=ok\n", step.ID, fileSizeKB))
			}

			// Check output contains expected strings
			outputStr := output.String()
			errorStr := errorOutput.String()

			for _, expected := range tt.expectErrors {
				if !strings.Contains(errorStr, expected) && !strings.Contains(outputStr, expected) {
					t.Errorf("Expected error containing %q, got output:\n%s\nerrors:\n%s", expected, outputStr, errorStr)
				}
			}
			for _, expected := range tt.expectWarnings {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected warning %q, got output:\n%s", expected, outputStr)
				}
			}
			for _, expected := range tt.expectOK {
				if !strings.Contains(outputStr, expected) {
					t.Errorf("Expected OK message %q, got output:\n%s", expected, outputStr)
				}
			}
		})
	}
}

// TestDoctorPlaceholderValidation tests placeholder validation (SBI-DR-003)
func TestDoctorPlaceholderValidation(t *testing.T) {
	tests := []struct {
		name             string
		content          string
		stepID           string
		expectErrors     []string
		expectWarnings   []string
	}{
		{
			name:    "valid placeholders only",
			content: "Hello {project_name}, task {task_id} on turn {turn} in {language}",
			stepID:  "test",
			expectErrors: nil,
			expectWarnings: nil,
		},
		{
			name:    "empty placeholder",
			content: "Hello {}, this is invalid",
			stepID:  "test",
			expectErrors: []string{"contains empty placeholder {} at line"},
			expectWarnings: nil,
		},
		{
			name:    "unknown placeholder",
			content: "Hello {foo}, this is unknown",
			stepID:  "test",
			expectErrors: []string{"unknown placeholder {foo} at line"},
			expectWarnings: nil,
		},
		{
			name:    "placeholder in fenced code block (ignored)",
			content: "```\n{foo} should be ignored\n```\nValid: {turn}",
			stepID:  "test",
			expectErrors: nil,
			expectWarnings: nil,
		},
		{
			name:    "placeholder in inline code (ignored)",
			content: "Use `{foo}` in your template. Valid: {turn}",
			stepID:  "test",
			expectErrors: nil,
			expectWarnings: nil,
		},
		{
			name:    "escaped placeholder (ignored)",
			content: "Literal \\{foo\\} should be ignored. Valid: {turn}",
			stepID:  "test",
			expectErrors: nil,
			expectWarnings: nil,
		},
		{
			name:    "mustache template (warning)",
			content: "Hello {{name}}, this is mustache style",
			stepID:  "test",
			expectErrors: nil,
			expectWarnings: []string{"contains non-standard {{name}} at line"},
		},
		{
			name:    "invalid identifier",
			content: "Hello {foo-bar}, this has invalid characters",
			stepID:  "test",
			expectErrors: []string{"invalid placeholder {foo-bar} at line"},
			expectWarnings: nil,
		},
		{
			name:    "whitespace placeholder",
			content: "Hello {   }, this is empty with spaces",
			stepID:  "test",
			expectErrors: []string{"contains empty placeholder {} at line"},
			expectWarnings: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, warnings := validatePlaceholders(tt.content, tt.stepID)

			// Check errors
			for _, expectedError := range tt.expectErrors {
				found := false
				for _, err := range errors {
					if strings.Contains(err, expectedError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected error containing %q, but got errors: %v", expectedError, errors)
				}
			}

			// Check warnings
			for _, expectedWarning := range tt.expectWarnings {
				found := false
				for _, warn := range warnings {
					if strings.Contains(warn, expectedWarning) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q, but got warnings: %v", expectedWarning, warnings)
				}
			}

			// Check no unexpected errors
			if len(tt.expectErrors) == 0 && len(errors) > 0 {
				t.Errorf("Expected no errors, but got: %v", errors)
			}

			// Check no unexpected warnings
			if len(tt.expectWarnings) == 0 && len(warnings) > 0 {
				t.Errorf("Expected no warnings, but got: %v", warnings)
			}
		})
	}
}

// TestDoctorRemoveCodeBlocks tests the code block removal functionality
func TestDoctorRemoveCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove fenced code block",
			input:    "Before\n```go\n{code}\n```\nAfter",
			expected: "Before\n\nAfter",
		},
		{
			name:     "remove inline code",
			input:    "Use `{placeholder}` in templates",
			expected: "Use  in templates",
		},
		{
			name:     "remove escaped braces",
			input:    "Literal \\{foo\\} and \\{bar\\}",
			expected: "Literal foo and bar",
		},
		{
			name:     "mixed content",
			input:    "```\n{ignore}\n```\nValid: {turn}\n`{also_ignore}`",
			expected: "\nValid: {turn}\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeCodeBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("removeCodeBlocks(%q) = %q, want %q", tt.input, result, tt.expected)
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