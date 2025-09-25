package runner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadPromptWithLimit(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		fileContent []byte
		limitKB     int
		wantErr     bool
		errContains string
	}{
		{
			name:        "file within limit",
			fileContent: make([]byte, 63*1024), // 63KB
			limitKB:     64,
			wantErr:     false,
		},
		{
			name:        "file at exact limit",
			fileContent: make([]byte, 64*1024), // 64KB
			limitKB:     64,
			wantErr:     false,
		},
		{
			name:        "file over limit",
			fileContent: make([]byte, 65*1024), // 65KB
			limitKB:     64,
			wantErr:     true,
			errContains: "prompt file too large (size=65KB, limit=64KB)",
		},
		{
			name:        "large file with higher limit",
			fileContent: make([]byte, 100*1024), // 100KB
			limitKB:     128,
			wantErr:     false,
		},
		{
			name:        "small file with small limit",
			fileContent: []byte("small content"),
			limitKB:     1,
			wantErr:     false,
		},
		{
			name:        "2KB file with 1KB limit",
			fileContent: make([]byte, 2*1024),
			limitKB:     1,
			wantErr:     true,
			errContains: "prompt file too large (size=2KB, limit=1KB)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			testFile := filepath.Join(tmpDir, "test_prompt.md")
			if err := os.WriteFile(testFile, tt.fileContent, 0644); err != nil {
				t.Fatal(err)
			}

			// Test ReadPromptWithLimit
			content, err := ReadPromptWithLimit(testFile, tt.limitKB)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("ReadPromptWithLimit() expected error, got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("ReadPromptWithLimit() error = %v, want containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("ReadPromptWithLimit() unexpected error = %v", err)
				}
				if len(content) != len(tt.fileContent) {
					t.Errorf("ReadPromptWithLimit() content length = %d, want %d", len(content), len(tt.fileContent))
				}
			}

			// Clean up
			os.Remove(testFile)
		})
	}
}

func TestReadPromptWithLimit_FileNotFound(t *testing.T) {
	nonExistentFile := "/tmp/non_existent_file_12345.md"

	_, err := ReadPromptWithLimit(nonExistentFile, 64)
	if err == nil {
		t.Error("ReadPromptWithLimit() expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "prompt stat:") {
		t.Errorf("ReadPromptWithLimit() error = %v, want containing 'prompt stat:'", err)
	}
}

func TestReadPromptWithLimit_ErrorMessage(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.md")

	// Create 100KB file
	largeContent := make([]byte, 100*1024)
	if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
		t.Fatal(err)
	}

	// Test with 64KB limit
	_, err := ReadPromptWithLimit(testFile, 64)
	if err == nil {
		t.Fatal("Expected error for oversized file")
	}

	// Check error message format
	errMsg := err.Error()
	if !strings.Contains(errMsg, "size=100KB") {
		t.Errorf("Error message should contain 'size=100KB', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "limit=64KB") {
		t.Errorf("Error message should contain 'limit=64KB', got: %s", errMsg)
	}
	if !strings.Contains(errMsg, testFile) {
		t.Errorf("Error message should contain file path, got: %s", errMsg)
	}
}