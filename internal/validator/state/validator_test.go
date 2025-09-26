package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateStateFile(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedErrors int
		expectedWarns  int
		expectedOK     int
		checkMessages  []string
	}{
		{
			name: "valid state.json",
			content: `{
				"version": 1,
				"step": "plan",
				"turn": 0,
				"meta.updated_at": "2025-09-26T01:45:00.123456789Z"
			}`,
			expectedErrors: 0,
			expectedWarns:  0,
			expectedOK:     1,
		},
		{
			name: "missing required key",
			content: `{
				"version": 1,
				"step": "plan",
				"turn": 0
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"missing required key: meta.updated_at"},
		},
		{
			name: "forbidden key present",
			content: `{
				"version": 1,
				"step": "plan",
				"turn": 0,
				"meta.updated_at": "2025-09-26T01:45:00.123456789Z",
				"current": "plan"
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"forbidden key present: current"},
		},
		{
			name: "invalid version",
			content: `{
				"version": 2,
				"step": "plan",
				"turn": 0,
				"meta.updated_at": "2025-09-26T01:45:00.123456789Z"
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"must be 1"},
		},
		{
			name: "invalid step",
			content: `{
				"version": 1,
				"step": "invalid",
				"turn": 0,
				"meta.updated_at": "2025-09-26T01:45:00.123456789Z"
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"invalid value: invalid"},
		},
		{
			name: "negative turn",
			content: `{
				"version": 1,
				"step": "plan",
				"turn": -1,
				"meta.updated_at": "2025-09-26T01:45:00.123456789Z"
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"must be >= 0"},
		},
		{
			name: "invalid timestamp format",
			content: `{
				"version": 1,
				"step": "plan",
				"turn": 0,
				"meta.updated_at": "2025-09-26T01:45:00.123456789+09:00"
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"not RFC3339Nano UTC Z"},
		},
		{
			name: "invalid JSON",
			content: `{
				"version": 1,
				"step": "plan",
				"turn": 0
				invalid json
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"invalid JSON"},
		},
		{
			name: "type mismatch - version string",
			content: `{
				"version": "1",
				"step": "plan",
				"turn": 0,
				"meta.updated_at": "2025-09-26T01:45:00.123456789Z"
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"must be an integer"},
		},
		{
			name: "type mismatch - meta.updated_at number",
			content: `{
				"version": 1,
				"step": "plan",
				"turn": 0,
				"meta.updated_at": 123456789
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"must be a string"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "state.json")
			err := os.WriteFile(tmpFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			// Validate
			result, err := ValidateStateFile(tmpFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Summary.Error != tt.expectedErrors {
				t.Errorf("expected %d errors, got %d", tt.expectedErrors, result.Summary.Error)
			}

			if result.Summary.Warn != tt.expectedWarns {
				t.Errorf("expected %d warnings, got %d", tt.expectedWarns, result.Summary.Warn)
			}

			if result.Summary.OK != tt.expectedOK {
				t.Errorf("expected %d OK, got %d", tt.expectedOK, result.Summary.OK)
			}

			// Check expected messages
			allMessages := []string{}
			for _, file := range result.Files {
				for _, issue := range file.Issues {
					allMessages = append(allMessages, issue.Message)
				}
			}

			for _, expectedMsg := range tt.checkMessages {
				found := false
				for _, msg := range allMessages {
					if strings.Contains(msg, expectedMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected message containing '%s' not found in: %v", expectedMsg, allMessages)
				}
			}
		})
	}
}

func TestValidateStateFile_NotFound(t *testing.T) {
	result, err := ValidateStateFile("/nonexistent/state.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Summary.Files != 1 {
		t.Errorf("expected 1 file, got %d", result.Summary.Files)
	}

	if result.Summary.Warn != 1 {
		t.Errorf("expected 1 warning for missing file, got %d", result.Summary.Warn)
	}

	if result.Summary.Error != 0 {
		t.Errorf("expected 0 errors for missing file, got %d", result.Summary.Error)
	}

	if len(result.Files) != 1 || result.Files[0].Issues[0].Message != "file not found" {
		t.Errorf("expected 'file not found' warning")
	}
}