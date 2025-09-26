package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// NewTestWorkspace creates a temporary workspace for testing
// It creates a temp directory, changes to it, and sets up the basic .deespec structure
// Returns a cleanup function that should be called via t.Cleanup()
func NewTestWorkspace(t *testing.T) func() {
	t.Helper()

	// Save current working directory
	originalCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Create temp directory
	tmpDir := t.TempDir()

	// Change to temp directory
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create basic .deespec structure
	dirs := []string{
		".deespec/specs/sbi",
		".deespec/specs/pbi",
		".deespec/etc/policies",
		".deespec/journal",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			// Restore working directory before failing
			os.Chdir(originalCwd)
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Return cleanup function
	return func() {
		// Restore original working directory
		if err := os.Chdir(originalCwd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}
}

// WriteTestPolicy creates a test policy file in the workspace
func WriteTestPolicy(t *testing.T, content string) string {
	t.Helper()

	policyPath := filepath.Join(".deespec", "etc", "policies", "register_policy.yaml")

	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test policy: %v", err)
	}

	return policyPath
}

// CreateSpecDirectory creates a spec directory for collision testing
func CreateSpecDirectory(t *testing.T, specPath string) {
	t.Helper()

	if err := os.MkdirAll(specPath, 0755); err != nil {
		t.Fatalf("Failed to create spec directory %s: %v", specPath, err)
	}

	// Create a marker file to ensure directory exists
	markerPath := filepath.Join(specPath, ".marker")
	if err := os.WriteFile(markerPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create marker file: %v", err)
	}
}

// WriteTestInput creates a test input file and returns its path
func WriteTestInput(t *testing.T, content string) string {
	t.Helper()

	inputFile := "test_input.yaml"
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test input: %v", err)
	}

	return inputFile
}

// CheckNoAbsolutePaths checks if the given path is absolute and fails if it is
func CheckNoAbsolutePaths(t *testing.T, path string) {
	t.Helper()

	if filepath.IsAbs(path) {
		t.Fatalf("Absolute path detected (not allowed in tests): %s", path)
	}
}

// AssertFileNotExists verifies that a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()

	CheckNoAbsolutePaths(t, path)

	if _, err := os.Stat(path); err == nil {
		t.Errorf("File should not exist but does: %s", path)
	}
}

// AssertFileExists verifies that a file exists
func AssertFileExists(t *testing.T, path string) {
	t.Helper()

	CheckNoAbsolutePaths(t, path)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("File should exist but doesn't: %s", path)
	}
}