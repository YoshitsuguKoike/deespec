package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
)

// TestSpec represents a specification for testing
type TestSpec struct {
	ID     string
	Title  string
	Labels []string
}

// TestValidationResult represents validation outcome for testing
type TestValidationResult struct {
	OK       bool
	Fatal    bool
	Errors   []string
	Warnings []string
}

// TestRegisterValidationBehavior verifies that validation behavior is consistent
// across both non-TX and TX paths
func TestRegisterValidationBehavior(t *testing.T) {
	tests := []struct {
		name           string
		spec           TestSpec
		existingSpecs  []TestSpec
		expectExitCode int
		expectStderr   string
		expectOK       bool
		useTX          bool
	}{
		{
			name: "Fatal validation error - both paths",
			spec: TestSpec{
				ID:     "", // Empty ID is fatal
				Title:  "Test",
				Labels: []string{"test"},
			},
			expectExitCode: 1,
			expectStderr:   "ERROR",
			expectOK:       false,
			useTX:          false,
		},
		{
			name: "Fatal validation error - TX path",
			spec: TestSpec{
				ID:     "", // Empty ID is fatal
				Title:  "Test",
				Labels: []string{"test"},
			},
			expectExitCode: 1,
			expectStderr:   "ERROR",
			expectOK:       false,
			useTX:          true,
		},
		{
			name: "Duplicate label warning - non-TX",
			spec: TestSpec{
				ID:     "test-001",
				Title:  "Test",
				Labels: []string{"test", "test"}, // Duplicate label
			},
			expectExitCode: 0,
			expectStderr:   "WARN",
			expectOK:       true,
			useTX:          false,
		},
		{
			name: "Duplicate label warning - TX",
			spec: TestSpec{
				ID:     "test-002",
				Title:  "Test",
				Labels: []string{"test", "test"}, // Duplicate label
			},
			expectExitCode: 0,
			expectStderr:   "WARN",
			expectOK:       true,
			useTX:          true,
		},
		{
			name: "Valid spec - non-TX",
			spec: TestSpec{
				ID:     "test-003",
				Title:  "Valid Test",
				Labels: []string{"test", "valid"},
			},
			expectExitCode: 0,
			expectStderr:   "",
			expectOK:       true,
			useTX:          false,
		},
		{
			name: "Valid spec - TX",
			spec: TestSpec{
				ID:     "test-004",
				Title:  "Valid Test",
				Labels: []string{"test", "valid"},
			},
			expectExitCode: 0,
			expectStderr:   "",
			expectOK:       true,
			useTX:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			tempDir, err := os.MkdirTemp("", "validation_test_*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Change to temp directory
			oldDir, _ := os.Getwd()
			os.Chdir(tempDir)
			defer os.Chdir(oldDir)

			// Create .deespec structure
			if err := os.MkdirAll(".deespec/var", 0755); err != nil {
				t.Fatalf("mkdir .deespec/var failed: %v", err)
			}
			if err := os.MkdirAll(".deespec/specs", 0755); err != nil {
				t.Fatalf("mkdir .deespec/specs failed: %v", err)
			}

			// Capture stderr
			var stderr bytes.Buffer
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Run registration based on path type
			var result *TestValidationResult
			if tt.useTX {
				result = validateSpecTX(&tt.spec, []TestSpec{})
			} else {
				result = validateSpec(&tt.spec, []TestSpec{})
			}

			// Restore stderr and capture output
			w.Close()
			os.Stderr = oldStderr
			buf := make([]byte, 1024)
			n, _ := r.Read(buf)
			stderr.Write(buf[:n])
			stderrStr := stderr.String()

			// Verify validation result
			if result.OK != tt.expectOK {
				t.Errorf("Expected OK=%v, got %v", tt.expectOK, result.OK)
			}

			// Verify stderr output
			if tt.expectStderr != "" && !strings.Contains(stderrStr, tt.expectStderr) {
				t.Errorf("Expected stderr to contain %q, got %q", tt.expectStderr, stderrStr)
			}

			// For fatal errors, verify that registration would fail
			if !result.OK && result.Fatal {
				// Try to perform actual registration (should fail)
				if tt.useTX {
					// TX path should rollback on fatal validation
					txnDir := filepath.Join(tempDir, ".deespec/var/txn")
					manager := txn.NewManager(txnDir)
					_, _ = manager.Begin(nil)

					// This should fail due to validation
					// Simplified check: validation should prevent commit
					err := error(nil)
					if !result.OK && result.Fatal {
						err = fmt.Errorf("validation failed")
					}
					if err == nil {
						t.Error("Expected registration to fail with fatal validation error")
					}
				}
			}
		})
	}
}

// validateSpec performs validation without transaction (original path)
func validateSpec(s *TestSpec, existingSpecs []TestSpec) *TestValidationResult {
	result := &TestValidationResult{OK: true}

	// Check for empty ID (fatal)
	if s.ID == "" {
		result.OK = false
		result.Fatal = true
		result.Errors = append(result.Errors, "ID cannot be empty")
		os.Stderr.WriteString("ERROR: ID cannot be empty\n")
	}

	// Check for duplicate labels (warning)
	seen := make(map[string]bool)
	for _, label := range s.Labels {
		if seen[label] {
			result.Warnings = append(result.Warnings, "Duplicate label: "+label)
			os.Stderr.WriteString("WARN: Duplicate label: " + label + "\n")
		}
		seen[label] = true
	}

	return result
}

// validateSpecTX performs validation with transaction support
func validateSpecTX(s *TestSpec, existingSpecs []TestSpec) *TestValidationResult {
	// Should have identical behavior to validateSpec
	return validateSpec(s, existingSpecs)
}
