package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
		expectError  string
		validateFunc func(t *testing.T, result RegisterResult)
	}{
		{
			name: "valid YAML input with new ID format",
			input: `id: SBI-REG-001
title: Test Registration
labels:
  - test
  - registration`,
			inputType:  "yaml",
			expectOK:   true,
			expectExit: 0,
			expectID:   "SBI-REG-001",
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
			inputType:  "json",
			expectOK:   true,
			expectExit: 0,
			expectID:   "SBI-TEST-002",
		},
		{
			name: "new ID format - uppercase letters and numbers",
			input: `id: ABC123
title: New Format Test
labels: [test]`,
			inputType:  "yaml",
			expectOK:   true,
			expectExit: 0,
			expectID:   "ABC123",
		},
		{
			name: "invalid ID - lowercase letters",
			input: `id: sbi-invalid
title: Invalid Test
labels: [bad]`,
			inputType:   "yaml",
			expectOK:    false,
			expectExit:  1,
			expectError: "invalid id format",
		},
		{
			name: "invalid ID - special characters",
			input: `id: SBI@TEST
title: Invalid Test
labels: [bad]`,
			inputType:   "yaml",
			expectOK:    false,
			expectExit:  1,
			expectError: "invalid id format",
		},
		{
			name: "ID too long",
			input: `id: ` + strings.Repeat("A", 65) + `
title: Too Long ID
labels: [test]`,
			inputType:   "yaml",
			expectOK:    false,
			expectExit:  1,
			expectError: "invalid id format",
		},
		{
			name: "empty title",
			input: `id: SBI-TEST-003
title: ""
labels: [test]`,
			inputType:   "yaml",
			expectOK:    false,
			expectExit:  1,
			expectID:    "SBI-TEST-003",
			expectError: "title is required",
		},
		{
			name: "title too long",
			input: `id: SBI-TEST-004
title: "` + strings.Repeat("a", 201) + `"
labels: [test]`,
			inputType:   "yaml",
			expectOK:    false,
			expectExit:  1,
			expectError: "title length exceeds",
		},
		{
			name: "labels with valid format",
			input: `id: SBI-TEST-005
title: Valid Labels
labels: [test-label, another-123]`,
			inputType:  "yaml",
			expectOK:   true,
			expectExit: 0,
		},
		{
			name: "invalid label format - uppercase",
			input: `id: SBI-TEST-006
title: Invalid Labels
labels: [TEST]`,
			inputType:   "yaml",
			expectOK:    false,
			expectExit:  1,
			expectError: "invalid label format",
		},
		{
			name: "duplicate labels - warning",
			input: `id: SBI-WARN-001
title: Duplicate Labels
labels: [dup, test, dup]`,
			inputType:  "yaml",
			expectOK:   true,
			expectExit: 0,
			expectID:   "SBI-WARN-001",
			validateFunc: func(t *testing.T, result RegisterResult) {
				found := false
				for _, w := range result.Warnings {
					if strings.Contains(w, "duplicate label: dup") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected duplicate label warning")
				}
			},
		},
		{
			name: "too many labels - warning",
			input: `id: SBI-WARN-002
title: Many Labels
labels: [` + func() string {
				labels := []string{}
				for i := 0; i < 35; i++ {
					labels = append(labels, fmt.Sprintf("label-%d", i))
				}
				return strings.Join(labels, ", ")
			}() + `]`,
			inputType:  "yaml",
			expectOK:   true,
			expectExit: 0,
			validateFunc: func(t *testing.T, result RegisterResult) {
				found := false
				for _, w := range result.Warnings {
					if strings.Contains(w, "labels count exceeds") {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected labels count warning")
				}
			},
		},
		{
			name: "unknown fields",
			input: `id: SBI-TEST-007
title: Test
labels: [test]
unknown_field: value`,
			inputType:   "yaml",
			expectOK:    false,
			expectExit:  1,
			expectError: "field unknown_field not found",
		},
		{
			name: "labels are optional",
			input: `id: SBI-TEST-008
title: No Labels Test`,
			inputType:  "yaml",
			expectOK:   true,
			expectExit: 0,
			expectID:   "SBI-TEST-008",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset exit code for each test
			exitCode = 0

			// Create temp directory for test
			tmpDir := t.TempDir()
			oldDir, _ := os.Getwd()
			defer os.Chdir(oldDir)
			os.Chdir(tmpDir)

			// Test with file input
			tmpFile := filepath.Join(tmpDir, "input."+tt.inputType)
			if err := os.WriteFile(tmpFile, []byte(tt.input), 0644); err != nil {
				t.Fatal(err)
			}

			// Capture output
			oldStdout := os.Stdout
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			errR, errW, _ := os.Pipe()
			os.Stdout = w
			os.Stderr = errW

			// Run command directly with flags
			cmd := NewRegisterCommand()
			_ = runRegisterWithFlags(cmd, []string{}, false, tmpFile, CollisionError)

			w.Close()
			errW.Close()
			os.Stdout = oldStdout
			os.Stderr = oldStderr

			var buf bytes.Buffer
			buf.ReadFrom(r)
			output := buf.String()

			var errBuf bytes.Buffer
			errBuf.ReadFrom(errR)
			stderrOutput := errBuf.String()

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

			// Check error message if expected
			if tt.expectError != "" {
				if !strings.Contains(result.Error, tt.expectError) {
					t.Errorf("expected error containing '%s', got '%s'", tt.expectError, result.Error)
				}
			}

			// Ensure stderr has appropriate log messages
			if !tt.expectOK && !strings.Contains(stderrOutput, "ERROR") {
				t.Errorf("expected ERROR in stderr for failed validation")
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, result)
			}

			// Ensure output is single line
			lines := strings.Split(output, "\n")
			if len(lines) > 2 || (len(lines) == 2 && lines[1] != "") {
				t.Errorf("output should be single line, got %d lines", len(lines))
			}

			// Ensure warnings field is always present as array
			if result.Warnings == nil {
				t.Errorf("warnings field should always be an array, got nil")
			}
		})
	}
}

func TestValidationFunctions(t *testing.T) {
	t.Run("validateID", func(t *testing.T) {
		tests := []struct {
			id        string
			expectErr bool
		}{
			{"ABC123", false},
			{"SBI-TEST-001", false},
			{"A-B-C", false},
			{"", true},           // empty
			{"abc123", true},     // lowercase
			{"ABC@123", true},    // special char
			{strings.Repeat("A", 65), true}, // too long
		}

		for _, tt := range tests {
			err := validateID(tt.id)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateID(%s): expected error=%v, got %v", tt.id, tt.expectErr, err)
			}
		}
	})

	t.Run("validateTitle", func(t *testing.T) {
		tests := []struct {
			title     string
			expectErr bool
		}{
			{"Valid Title", false},
			{"", true}, // empty
			{strings.Repeat("a", 200), false}, // max length
			{strings.Repeat("a", 201), true},  // too long
		}

		for _, tt := range tests {
			err := validateTitle(tt.title)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateTitle(%s): expected error=%v, got %v", tt.title, tt.expectErr, err)
			}
		}
	})

	t.Run("validateLabels", func(t *testing.T) {
		tests := []struct {
			labels       []string
			expectErr    bool
			expectWarn   bool
			warnContains string
		}{
			{[]string{"test", "label"}, false, false, ""},
			{[]string{"test-123", "another"}, false, false, ""},
			{[]string{"TEST"}, true, false, ""}, // uppercase
			{[]string{"test@"}, true, false, ""}, // special char
			{[]string{"dup", "dup"}, false, true, "duplicate"},
			{func() []string {
				labels := []string{}
				for i := 0; i < 35; i++ {
					labels = append(labels, fmt.Sprintf("label-%d", i))
				}
				return labels
			}(), false, true, "exceeds"},
			{nil, false, false, ""}, // nil is ok
		}

		for _, tt := range tests {
			warnings, err := validateLabels(tt.labels)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateLabels(%v): expected error=%v, got %v", tt.labels, tt.expectErr, err)
			}
			if tt.expectWarn && len(warnings) == 0 {
				t.Errorf("validateLabels(%v): expected warnings", tt.labels)
			}
			if tt.warnContains != "" {
				found := false
				for _, w := range warnings {
					if strings.Contains(w, tt.warnContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("validateLabels(%v): expected warning containing '%s'", tt.labels, tt.warnContains)
				}
			}
		}
	})
}

func TestInputSizeLimit(t *testing.T) {
	// Save original settings
	originalExit := exitFunc
	defer func() { exitFunc = originalExit }()
	isTestMode = true
	defer func() { isTestMode = false }()

	exitFunc = func(code int) {}

	// Create oversized input
	largeInput := `id: TEST-LARGE
title: Large Input Test
labels: [test]
data: ` + strings.Repeat("a", MaxInputSize)

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "large.yaml")
	if err := os.WriteFile(tmpFile, []byte(largeInput), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := NewRegisterCommand()
	_ = runRegisterWithFlags(cmd, []string{}, false, tmpFile, CollisionError)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Parse result
	var result RegisterResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if result.OK {
		t.Errorf("expected failure for oversized input")
	}

	if !strings.Contains(result.Error, "exceeds limit") {
		t.Errorf("expected size limit error, got: %s", result.Error)
	}
}

func TestBuildSpecPath(t *testing.T) {
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
			expected: ".deespec/specs/sbi/SBI-TEST-004_title-with-underscores",
		},
		{
			id:       "SBI-TEST-005",
			title:    "Title123With456Numbers",
			expected: ".deespec/specs/sbi/SBI-TEST-005_title123with456numbers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			config := GetDefaultPolicy()
			resolvedConfig, _ := ResolveRegisterConfig("", config)
			result, err := buildSafeSpecPathWithConfig(tt.id, tt.title, resolvedConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}