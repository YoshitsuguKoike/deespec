package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterCommand(t *testing.T) {
	// Save original exit function
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()

	// Enable test mode to skip path validation
	isTestMode = true
	defer func() { isTestMode = false }()

	// Mock exit function for tests
	var exitCode int
	exitFunc = func(code int) {
		exitCode = code
	}

	tests := []struct {
		name         string
		input        string
		inputType    string // "yaml" or "json"
		expectOK     bool
		expectExit   int // expected exit code (0 for success)
		expectID     string
		validateFunc func(t *testing.T, result RegisterResult)
	}{
		{
			name: "valid YAML input",
			input: `id: SBI-REG-001
title: Test Registration
labels:
  - test
  - registration`,
			inputType: "yaml",
			expectOK:  true,
			expectExit: 0,
			expectID:  "SBI-REG-001",
			validateFunc: func(t *testing.T, result RegisterResult) {
				if result.SpecPath != ".deespec/specs/sbi/SBI-REG-001_test-registration" {
					t.Errorf("unexpected spec_path: %s", result.SpecPath)
				}
				if len(result.Warnings) != 0 {
					t.Errorf("expected no warnings, got: %v", result.Warnings)
				}
			},
		},
		{
			name: "valid JSON input",
			input: `{
				"id": "SBI-TEST-002",
				"title": "JSON Test",
				"labels": ["json", "test"]
			}`,
			inputType: "json",
			expectOK:  true,
			expectExit: 0,
			expectID:  "SBI-TEST-002",
			validateFunc: func(t *testing.T, result RegisterResult) {
				if result.SpecPath != ".deespec/specs/sbi/SBI-TEST-002_json-test" {
					t.Errorf("unexpected spec_path: %s", result.SpecPath)
				}
			},
		},
		{
			name: "invalid ID format",
			input: `id: invalid-id
title: Test
labels: [test]`,
			inputType:  "yaml",
			expectOK:   false,
			expectExit: 1,
			expectID:   "invalid-id",
			validateFunc: func(t *testing.T, result RegisterResult) {
				if len(result.Warnings) == 0 {
					t.Errorf("expected warnings for invalid ID")
				}
			},
		},
		{
			name: "empty title",
			input: `id: SBI-TEST-003
title: ""
labels: [test]`,
			inputType:  "yaml",
			expectOK:   false,
			expectExit: 1,
			expectID:   "SBI-TEST-003",
			validateFunc: func(t *testing.T, result RegisterResult) {
				if !strings.Contains(result.Warnings[0], "title") {
					t.Errorf("expected title validation error")
				}
			},
		},
		{
			name: "empty labels",
			input: `id: SBI-TEST-004
title: Test
labels: []`,
			inputType:  "yaml",
			expectOK:   false,
			expectExit: 1,
			expectID:   "SBI-TEST-004",
			validateFunc: func(t *testing.T, result RegisterResult) {
				if !strings.Contains(result.Warnings[0], "labels") {
					t.Errorf("expected labels validation error")
				}
			},
		},
		{
			name: "unknown fields",
			input: `id: SBI-TEST-005
title: Test
labels: [test]
unknown_field: value`,
			inputType:  "yaml",
			expectOK:   false,
			expectExit: 1,
			validateFunc: func(t *testing.T, result RegisterResult) {
				if len(result.Warnings) == 0 {
					t.Errorf("expected warnings for unknown fields")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset exit code for each test
			exitCode = 0

			// Test with file input
			tmpFile := filepath.Join(t.TempDir(), "input."+tt.inputType)
			if err := os.WriteFile(tmpFile, []byte(tt.input), 0644); err != nil {
				t.Fatal(err)
			}

			// Capture output
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			// Run command directly with flags
			cmd := NewRegisterCommand()
			_ = runRegisterWithFlags(cmd, []string{}, false, tmpFile)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			// Parse JSON output
			var result RegisterResult
			if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
				t.Fatalf("failed to parse JSON output: %v, output: %s", err, output)
			}

			// Check exit code
			if exitCode != tt.expectExit {
				t.Errorf("expected exit code %d, got %d", tt.expectExit, exitCode)
			}

			// Validate result
			if result.OK != tt.expectOK {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectOK, result.OK)
			}

			if tt.expectID != "" && result.ID != tt.expectID {
				t.Errorf("expected ID=%s, got ID=%s", tt.expectID, result.ID)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, result)
			}

			// Ensure output is single line
			lines := strings.Split(output, "\n")
			if len(lines) > 2 || (len(lines) == 2 && lines[1] != "") {
				t.Errorf("output should be single line, got %d lines", len(lines))
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		id       string
		title    string
		expected string
	}{
		{
			id:       "SBI-TEST-001",
			title:    "Simple Title",
			expected: ".deespec/specs/sbi/SBI-TEST-001_simple-title",
		},
		{
			id:       "SBI-TEST-002",
			title:    "Title With  Multiple   Spaces",
			expected: ".deespec/specs/sbi/SBI-TEST-002_title-with-multiple-spaces",
		},
		{
			id:       "SBI-TEST-003",
			title:    "Title-With-Hyphens",
			expected: ".deespec/specs/sbi/SBI-TEST-003_title-with-hyphens",
		},
		{
			id:       "SBI-TEST-004",
			title:    "Title_With_Underscores",
			expected: ".deespec/specs/sbi/SBI-TEST-004_titlewithunderscores",
		},
		{
			id:       "SBI-TEST-005",
			title:    "Title123With456Numbers",
			expected: ".deespec/specs/sbi/SBI-TEST-005_title123with456numbers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			result := calculateSpecPath(tt.id, tt.title)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidateSpec(t *testing.T) {
	tests := []struct {
		name      string
		spec      RegisterSpec
		expectErr bool
		errMsg    string
	}{
		{
			name: "valid spec",
			spec: RegisterSpec{
				ID:     "SBI-TEST-001",
				Title:  "Valid Spec",
				Labels: []string{"test"},
			},
			expectErr: false,
		},
		{
			name: "invalid ID - wrong format",
			spec: RegisterSpec{
				ID:     "invalid-id",
				Title:  "Test",
				Labels: []string{"test"},
			},
			expectErr: true,
			errMsg:    "invalid ID format",
		},
		{
			name: "invalid ID - missing number",
			spec: RegisterSpec{
				ID:     "SBI-TEST-XX",
				Title:  "Test",
				Labels: []string{"test"},
			},
			expectErr: true,
			errMsg:    "invalid ID format",
		},
		{
			name: "invalid ID - too few digits",
			spec: RegisterSpec{
				ID:     "SBI-TEST-01",
				Title:  "Test",
				Labels: []string{"test"},
			},
			expectErr: true,
			errMsg:    "invalid ID format",
		},
		{
			name: "empty title",
			spec: RegisterSpec{
				ID:     "SBI-TEST-001",
				Title:  "",
				Labels: []string{"test"},
			},
			expectErr: true,
			errMsg:    "title cannot be empty",
		},
		{
			name: "empty labels",
			spec: RegisterSpec{
				ID:     "SBI-TEST-001",
				Title:  "Test",
				Labels: []string{},
			},
			expectErr: true,
			errMsg:    "labels cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSpec(&tt.spec)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}