package health

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateHealthFile(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedErrors int
		expectedWarns  int
		expectedOK     int
		checkMessages  []string
	}{
		{
			name: "valid health.json",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "plan",
				"ok": true,
				"error": ""
			}`,
			expectedErrors: 0,
			expectedWarns:  0,
			expectedOK:     1,
		},
		{
			name: "missing required key",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "plan",
				"ok": true
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"missing required key: error"},
		},
		{
			name: "invalid timestamp format",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789+09:00",
				"turn": 0,
				"step": "plan",
				"ok": true,
				"error": ""
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"not RFC3339Nano UTC Z"},
		},
		{
			name: "negative turn",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": -1,
				"step": "plan",
				"ok": true,
				"error": ""
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"must be >= 0"},
		},
		{
			name: "invalid step",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "invalid",
				"ok": true,
				"error": ""
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"invalid value: invalid"},
		},
		{
			name: "invalid ok type",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "plan",
				"ok": "yes",
				"error": ""
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"must be a boolean"},
		},
		{
			name: "invalid error type",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "plan",
				"ok": true,
				"error": 123
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"must be a string"},
		},
		{
			name: "ok/error consistency warning - ok=true with error",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "plan",
				"ok": true,
				"error": "failed to connect"
			}`,
			expectedErrors: 0,
			expectedWarns:  1,
			expectedOK:     0,
			checkMessages:  []string{"ok=true but error=\"failed to connect\""},
		},
		{
			name: "ok/error consistency warning - ok=false without error",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "plan",
				"ok": false,
				"error": ""
			}`,
			expectedErrors: 0,
			expectedWarns:  1,
			expectedOK:     0,
			checkMessages:  []string{"ok=false but error is empty"},
		},
		{
			name: "valid ok=false with error",
			content: `{
				"ts": "2025-09-26T01:45:00.123456789Z",
				"turn": 0,
				"step": "plan",
				"ok": false,
				"error": "connection failed"
			}`,
			expectedErrors: 0,
			expectedWarns:  0,
			expectedOK:     1,
		},
		{
			name: "multiple errors",
			content: `{
				"ts": "invalid",
				"turn": -1,
				"step": "invalid",
				"ok": "maybe",
				"error": 123
			}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages: []string{
				"invalid RFC3339Nano format",
				"must be >= 0",
				"invalid value: invalid",
				"must be a boolean",
				"must be a string",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "health.json")
			err := os.WriteFile(tmpFile, []byte(tt.content), 0644)
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}

			// Validate
			result, err := ValidateHealthFile(tmpFile)
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

func TestValidateHealthFile_NotFound(t *testing.T) {
	result, err := ValidateHealthFile("/nonexistent/health.json")
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