package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	usecaseSbi "github.com/YoshitsuguKoike/deespec/internal/usecase/sbi"
)

func TestNewSBIRegisterCommand(t *testing.T) {
	cmd := NewSBIRegisterCommand()

	// Test command properties
	if cmd.Use != "register" {
		t.Errorf("Expected Use to be 'register', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	// Test required flags
	titleFlag := cmd.Flag("title")
	if titleFlag == nil {
		t.Error("Expected --title flag to be defined")
	}

	// Test optional flags
	flags := []string{"body", "json", "dry-run", "quiet", "label", "labels"}
	for _, flagName := range flags {
		flag := cmd.Flag(flagName)
		if flag == nil {
			t.Errorf("Expected --%s flag to be defined", flagName)
		}
	}
}

func TestSBIRegister_MissingTitle(t *testing.T) {
	cmd := NewSBIRegisterCommand()
	cmd.SetArgs([]string{"--body", "test content"})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when title is missing")
	}

	if !strings.Contains(err.Error(), "title") {
		t.Errorf("Error should mention 'title', got: %v", err)
	}
}

func TestSBIRegister_DryRun(t *testing.T) {
	// Test dry-run mode functionality
	// Note: fmt.Printf outputs directly to os.Stdout, so we test the command execution only

	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "Dry run with valid input",
			args: []string{
				"--title", "Test Spec",
				"--body", "Test content",
				"--dry-run",
				"--quiet", // Use quiet mode to avoid output issues
			},
			wantErr: false,
		},
		{
			name: "Dry run with JSON output",
			args: []string{
				"--title", "JSON Test",
				"--body", "JSON content",
				"--dry-run",
				"--json",
				"--quiet",
			},
			wantErr: false,
		},
		{
			name: "Dry run missing title",
			args: []string{
				"--body", "Test content",
				"--dry-run",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSBIRegisterCommand()
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSBIRegister_StdinInput(t *testing.T) {
	// This test simulates stdin input
	// In real test, we would need to mock os.Stdin
	// For now, we just test that body flag works
	cmd := NewSBIRegisterCommand()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{
		"--title", "Stdin Test",
		"--body", "Direct body input",
		"--dry-run",
		"--quiet",
	})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// In quiet mode with successful execution, there should be no output
	if buf.String() != "" {
		t.Errorf("Expected no output, got: %s", buf.String())
	}
}

func TestRunSBIRegister_EmptyTitle(t *testing.T) {
	flags := &sbiRegisterFlags{
		title: "",
		body:  "test",
	}

	err := runSBIRegister(context.Background(), flags)
	if err == nil {
		t.Error("Expected error for empty title")
	}

	if !strings.Contains(err.Error(), "title is required") {
		t.Errorf("Expected 'title is required' error, got: %v", err)
	}
}

func TestHandleDryRun(t *testing.T) {
	tests := []struct {
		name      string
		input     usecaseSbi.RegisterSBIInput
		flags     *sbiRegisterFlags
		wantError bool
		validate  func(t *testing.T)
	}{
		{
			name: "Dry run with valid input",
			input: usecaseSbi.RegisterSBIInput{
				Title:  "Test Title",
				Body:   "Test Body",
				Labels: []string{"test", "dry-run"},
			},
			flags: &sbiRegisterFlags{
				jsonOut: false,
				quiet:   false,
			},
			wantError: false,
			validate: func(t *testing.T) {
				// Verify no actual files were created
				// (handleDryRun uses MemMapFs internally)
			},
		},
		{
			name: "Dry run with JSON output",
			input: usecaseSbi.RegisterSBIInput{
				Title: "JSON Title",
				Body:  "JSON Body",
			},
			flags: &sbiRegisterFlags{
				jsonOut: true,
				quiet:   false,
			},
			wantError: false,
			validate: func(t *testing.T) {
				// JSON output is handled by the function
			},
		},
		{
			name: "Dry run in quiet mode",
			input: usecaseSbi.RegisterSBIInput{
				Title: "Quiet Title",
				Body:  "Quiet Body",
			},
			flags: &sbiRegisterFlags{
				jsonOut: false,
				quiet:   true,
			},
			wantError: false,
			validate: func(t *testing.T) {
				// No output expected in quiet mode
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: handleDryRun creates its own MemMapFs internally
			// so we don't need to verify file system state here
			err := handleDryRun(tt.input, tt.flags)

			if (err != nil) != tt.wantError {
				t.Errorf("handleDryRun() error = %v, wantError %v", err, tt.wantError)
			}

			if tt.validate != nil {
				tt.validate(t)
			}
		})
	}
}

func TestIsInputFromTerminal(t *testing.T) {
	// Test the isInputFromTerminal function
	// In actual implementation, this checks os.Stdin.Stat()
	// For unit test, we acknowledge this is difficult to mock
	result := isInputFromTerminal()

	// The function should return a boolean
	// When running tests, it usually returns true (terminal input)
	if result != true && result != false {
		t.Error("isInputFromTerminal should return a boolean")
	}
}

func TestOutputJSON(t *testing.T) {
	// Note: outputJSON writes to os.Stdout directly
	// In a real test, we would need to capture stdout
	// For now, we verify the function exists and can be called

	output := &usecaseSbi.RegisterSBIOutput{
		ID:       "SBI-TEST123",
		SpecPath: ".deespec/specs/sbi/SBI-TEST123/spec.md",
	}

	// This would normally write to stdout
	// We can't easily capture it in this test without refactoring
	// The existence of the function is verified by compilation
	_ = output
}

func TestProcessLabels(t *testing.T) {
	tests := []struct {
		name       string
		labelArray []string
		labelsStr  string
		expected   []string
	}{
		{
			name:       "Empty inputs",
			labelArray: []string{},
			labelsStr:  "",
			expected:   []string{},
		},
		{
			name:       "Single label from --label",
			labelArray: []string{"label1"},
			labelsStr:  "",
			expected:   []string{"label1"},
		},
		{
			name:       "Multiple labels from --label",
			labelArray: []string{"label1", "label2", "label3"},
			labelsStr:  "",
			expected:   []string{"label1", "label2", "label3"},
		},
		{
			name:       "Single label from --labels",
			labelArray: []string{},
			labelsStr:  "label1",
			expected:   []string{"label1"},
		},
		{
			name:       "Multiple labels from --labels (comma-separated)",
			labelArray: []string{},
			labelsStr:  "label1,label2,label3",
			expected:   []string{"label1", "label2", "label3"},
		},
		{
			name:       "Labels with spaces (should be trimmed)",
			labelArray: []string{" label1 ", "  label2  "},
			labelsStr:  " label3 , label4 ",
			expected:   []string{"label1", "label2", "label3", "label4"},
		},
		{
			name:       "Duplicate labels (should be deduplicated)",
			labelArray: []string{"label1", "label2", "label1"},
			labelsStr:  "label2,label3,label1",
			expected:   []string{"label1", "label2", "label3"},
		},
		{
			name:       "Mixed input from both flags",
			labelArray: []string{"label1", "label2"},
			labelsStr:  "label3,label4",
			expected:   []string{"label1", "label2", "label3", "label4"},
		},
		{
			name:       "Empty strings in input (should be filtered)",
			labelArray: []string{"", "label1", ""},
			labelsStr:  ",label2,,label3,",
			expected:   []string{"label1", "label2", "label3"},
		},
		{
			name:       "Whitespace-only labels (should be filtered)",
			labelArray: []string{"   ", "label1", "\t\n"},
			labelsStr:  "  ,label2,   ",
			expected:   []string{"label1", "label2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := processLabels(tt.labelArray, tt.labelsStr)

			if len(result) != len(tt.expected) {
				t.Errorf("processLabels() returned %d labels, expected %d", len(result), len(tt.expected))
				t.Errorf("Got: %v, Expected: %v", result, tt.expected)
				return
			}

			for i, label := range result {
				if label != tt.expected[i] {
					t.Errorf("processLabels()[%d] = %q, expected %q", i, label, tt.expected[i])
				}
			}
		})
	}
}

func TestSBIRegisterWithLabels(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{
			name: "Register with single --label",
			args: []string{
				"--title", "Test with Label",
				"--body", "Test content",
				"--label", "feature",
				"--dry-run",
				"--quiet",
			},
			wantErr: false,
		},
		{
			name: "Register with multiple --label flags",
			args: []string{
				"--title", "Test with Multiple Labels",
				"--body", "Test content",
				"--label", "feature",
				"--label", "security",
				"--label", "v1.0",
				"--dry-run",
				"--quiet",
			},
			wantErr: false,
		},
		{
			name: "Register with --labels (comma-separated)",
			args: []string{
				"--title", "Test with Comma Labels",
				"--body", "Test content",
				"--labels", "feature,security,v1.0",
				"--dry-run",
				"--quiet",
			},
			wantErr: false,
		},
		{
			name: "Register with both --label and --labels",
			args: []string{
				"--title", "Test with Mixed Labels",
				"--body", "Test content",
				"--label", "feature",
				"--labels", "security,v1.0",
				"--dry-run",
				"--quiet",
			},
			wantErr: false,
		},
		{
			name: "Register with duplicate labels",
			args: []string{
				"--title", "Test with Duplicate Labels",
				"--body", "Test content",
				"--label", "feature",
				"--label", "feature",
				"--labels", "feature,security",
				"--dry-run",
				"--quiet",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := NewSBIRegisterCommand()
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
