package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadExternalPrompt_Success(t *testing.T) {
	// Setup: Create temporary prompts directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create .deespec/prompts directory
	promptsDir := filepath.Join(".deespec", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Test cases for each status
	testCases := []struct {
		name         string
		status       string
		filename     string
		content      string
		wantInPrompt string
	}{
		{
			name:     "WIP status loads WIP.md",
			status:   "WIP",
			filename: "WIP.md",
			content: `# Test Implementation
SBI ID: {{.SBIID}}
Turn: {{.Turn}}
Task: {{.TaskDescription}}`,
			wantInPrompt: "Test Implementation",
		},
		{
			name:     "READY status loads WIP.md",
			status:   "READY",
			filename: "WIP.md",
			content: `# Ready Implementation
SBI: {{.SBIID}}`,
			wantInPrompt: "Ready Implementation",
		},
		{
			name:     "REVIEW status loads REVIEW.md",
			status:   "REVIEW",
			filename: "REVIEW.md",
			content: `# Review Task
Reviewing: {{.SBIID}}
Step: {{.Step}}`,
			wantInPrompt: "Review Task",
		},
		{
			name:     "REVIEW&WIP status loads REVIEW_AND_WIP.md",
			status:   "REVIEW&WIP",
			filename: "REVIEW_AND_WIP.md",
			content: `# Force Implementation
SBI: {{.SBIID}}
Final attempt`,
			wantInPrompt: "Force Implementation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Write test prompt file
			promptFile := filepath.Join(promptsDir, tc.filename)
			if err := os.WriteFile(promptFile, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Failed to write prompt file: %v", err)
			}
			defer os.Remove(promptFile) // Clean up after test

			// Create builder
			builder := ClaudeCodePromptBuilder{
				WorkDir: "test/dir",
				SBIDir:  "test/sbi",
				SBIID:   "TEST-001",
				Turn:    5,
				Step:    "test_step",
			}

			// Load external prompt
			prompt, err := builder.LoadExternalPrompt(tc.status, "Test task description")
			if err != nil {
				t.Errorf("Failed to load prompt for %s: %v", tc.status, err)
			}

			// Verify prompt contains expected content
			if !strings.Contains(prompt, tc.wantInPrompt) {
				t.Errorf("Prompt should contain '%s', got: %s", tc.wantInPrompt, prompt)
			}

			// Verify placeholders are replaced
			if strings.Contains(prompt, "{{.SBIID}}") {
				t.Error("SBIID placeholder was not replaced")
			}
			if strings.Contains(prompt, "{{.Turn}}") {
				t.Error("Turn placeholder was not replaced")
			}

			// Verify actual values are present
			if !strings.Contains(prompt, "TEST-001") {
				t.Error("SBIID value not found in prompt")
			}
			// Only check for Turn if it was in the original content
			if strings.Contains(tc.content, "Turn:") && !strings.Contains(prompt, "5") {
				t.Error("Turn value not found in prompt")
			}
		})
	}
}

func TestLoadExternalPrompt_FileNotFound(t *testing.T) {
	// Setup: Use temp directory without prompt files
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create builder
	builder := ClaudeCodePromptBuilder{
		WorkDir: "test/dir",
		SBIDir:  "test/sbi",
		SBIID:   "TEST-001",
		Turn:    1,
		Step:    "test_step",
	}

	// Test each status returns error when file is missing
	statuses := []string{"WIP", "REVIEW", "REVIEW&WIP"}

	for _, status := range statuses {
		t.Run("Missing file for "+status, func(t *testing.T) {
			_, err := builder.LoadExternalPrompt(status, "Test task")
			if err == nil {
				t.Errorf("Expected error for missing file, but got nil for status: %s", status)
			}

			// Verify it's a file not found error
			if !os.IsNotExist(err) {
				t.Errorf("Expected file not exist error, got: %v", err)
			}
		})
	}
}

func TestLoadExternalPrompt_InvalidStatus(t *testing.T) {
	builder := ClaudeCodePromptBuilder{
		WorkDir: "test/dir",
		SBIDir:  "test/sbi",
		SBIID:   "TEST-001",
		Turn:    1,
		Step:    "test_step",
	}

	// Test with invalid status
	_, err := builder.LoadExternalPrompt("INVALID_STATUS", "Test task")
	if err == nil {
		t.Error("Expected error for invalid status")
	}

	expectedErr := "unknown status: INVALID_STATUS"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got: %v", expectedErr, err)
	}
}

func TestLoadExternalPrompt_FileDeleted(t *testing.T) {
	// Setup: Create and then delete prompt file
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create .deespec/prompts directory
	promptsDir := filepath.Join(".deespec", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Create and then delete WIP.md
	wipFile := filepath.Join(promptsDir, "WIP.md")
	content := "# Test prompt"
	if err := os.WriteFile(wipFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create WIP.md: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(wipFile); os.IsNotExist(err) {
		t.Fatal("WIP.md should exist before deletion")
	}

	// Delete the file (simulating user deletion)
	if err := os.Remove(wipFile); err != nil {
		t.Fatalf("Failed to delete WIP.md: %v", err)
	}

	// Create builder
	builder := ClaudeCodePromptBuilder{
		WorkDir: "test/dir",
		SBIDir:  "test/sbi",
		SBIID:   "TEST-001",
		Turn:    1,
		Step:    "implement",
	}

	// Try to load deleted file
	_, err := builder.LoadExternalPrompt("WIP", "Test task")
	if err == nil {
		t.Error("Expected error when file is deleted")
	}

	// Verify it's a file not found error
	if !os.IsNotExist(err) {
		t.Errorf("Expected file not exist error after deletion, got: %v", err)
	}
}

func TestBuildPromptFallback(t *testing.T) {
	// This test verifies that buildPromptByStatus falls back to hardcoded prompts
	// when external files are not available

	// Setup: Use temp directory without prompt files
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create test state
	state := &State{
		Status:  "WIP",
		WIP:     "TEST-001",
		Turn:    1,
		Attempt: 1,
		Inputs: map[string]string{
			"todo": "Test implementation task",
		},
		LastArtifacts: make(map[string]string),
	}

	// Call buildPromptByStatus which should use fallback
	prompt := buildPromptByStatus(state)

	// Verify we got a valid prompt (not empty)
	if prompt == "" {
		t.Error("Expected fallback prompt, got empty string")
	}

	// Verify it contains expected hardcoded sections
	expectedSections := []string{
		"# Implementation Task",
		"## Context",
		"## Instructions",
		"## Available Tools",
		"## Implementation Note",
	}

	for _, section := range expectedSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("Fallback prompt should contain '%s'", section)
		}
	}
}

func TestPlaceholderReplacement(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create .deespec/prompts directory
	promptsDir := filepath.Join(".deespec", "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Create test prompt with all placeholders
	testPrompt := `# Test Prompt
WorkDir: {{.WorkDir}}
SBIID: {{.SBIID}}
Turn: {{.Turn}}
Step: {{.Step}}
SBIDir: {{.SBIDir}}
TaskDescription: {{.TaskDescription}}
Timestamp: {{.Timestamp}}`

	promptFile := filepath.Join(promptsDir, "WIP.md")
	if err := os.WriteFile(promptFile, []byte(testPrompt), 0644); err != nil {
		t.Fatalf("Failed to write prompt file: %v", err)
	}

	// Create builder with specific values
	builder := ClaudeCodePromptBuilder{
		WorkDir: "work/project",
		SBIDir:  "work/sbi/TEST-001",
		SBIID:   "TEST-001",
		Turn:    42,
		Step:    "implement_try",
	}

	// Load and check replacements
	prompt, err := builder.LoadExternalPrompt("WIP", "Custom task description")
	if err != nil {
		t.Fatalf("Failed to load prompt: %v", err)
	}

	// Check all placeholders are replaced
	checks := []struct {
		placeholder string
		expected    string
	}{
		{"{{.WorkDir}}", "work/project"},
		{"{{.SBIID}}", "TEST-001"},
		{"{{.Turn}}", "42"},
		{"{{.Step}}", "implement_try"},
		{"{{.SBIDir}}", "work/sbi/TEST-001"},
		{"{{.TaskDescription}}", "Custom task description"},
	}

	for _, check := range checks {
		if strings.Contains(prompt, check.placeholder) {
			t.Errorf("Placeholder %s was not replaced", check.placeholder)
		}
		if !strings.Contains(prompt, check.expected) {
			t.Errorf("Expected value '%s' not found in prompt", check.expected)
		}
	}

	// Verify timestamp is replaced (just check placeholder is gone)
	if strings.Contains(prompt, "{{.Timestamp}}") {
		t.Error("Timestamp placeholder was not replaced")
	}
}
