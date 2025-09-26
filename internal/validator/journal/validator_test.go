package journal

import (
	"strings"
	"testing"
)

func TestJournalValidation(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedErrors int
		expectedWarns  int
		expectedOK     int
		checkMessages  []string // Expected error/warning messages to contain these strings
	}{
		{
			name: "valid journal entry",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":["/tmp/artifacts/turn1/plan.md"]}
{"ts":"2025-09-26T00:15:34.123456789Z","turn":2,"step":"implement","decision":"PENDING","elapsed_ms":2300,"error":"","artifacts":[]}`,
			expectedErrors: 0,
			expectedWarns:  0,
			expectedOK:     2,
		},
		{
			name: "missing required key",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":""}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"expected exactly 7 keys, found 6"},
		},
		{
			name: "extra key",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[],"extra":"value"}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"expected exactly 7 keys, found 8"},
		},
		{
			name: "invalid timestamp (not UTC)",
			content: `{"ts":"2025-09-26T00:12:34.123456789+09:00","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"timestamp must be UTC"},
		},
		{
			name: "invalid timestamp format",
			content: `{"ts":"2025-09-26","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"invalid RFC3339Nano format"},
		},
		{
			name: "negative turn",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":-1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"turn must be >= 0"},
		},
		{
			name: "invalid step",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"invalid_step","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"invalid step value"},
		},
		{
			name: "invalid decision",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"MAYBE","elapsed_ms":1500,"error":"","artifacts":[]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"invalid decision value"},
		},
		{
			name: "negative elapsed_ms",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":-100,"error":"","artifacts":[]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"elapsed_ms must be >= 0"},
		},
		{
			name: "artifact turn inconsistency",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":["/tmp/artifacts/turn999/plan.md"]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"no artifact path contains /turn1/"},
		},
		{
			name: "turn monotonicity warning",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":5,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}
{"ts":"2025-09-26T00:15:34.123456789Z","turn":3,"step":"implement","decision":"OK","elapsed_ms":2300,"error":"","artifacts":[]}`,
			expectedErrors: 0,
			expectedWarns:  1,
			expectedOK:     1, // First line is OK, second line has warning
			checkMessages:  []string{"turn decreased from 5 to 3"},
		},
		{
			name: "empty lines ignored",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}

{"ts":"2025-09-26T00:15:34.123456789Z","turn":2,"step":"implement","decision":"OK","elapsed_ms":2300,"error":"","artifacts":[]}`,
			expectedErrors: 0,
			expectedWarns:  0,
			expectedOK:     2,
		},
		{
			name: "invalid JSON",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}
invalid json line`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     1,
			checkMessages:  []string{"invalid JSON"},
		},
		{
			name: "mixed artifact types (valid)",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":["/tmp/artifacts/turn1/plan.md",{"type":"summary","value":"test"}]}`,
			expectedErrors: 0,
			expectedWarns:  0,
			expectedOK:     1,
		},
		{
			name: "artifact with wrong type",
			content: `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[123]}`,
			expectedErrors: 1,
			expectedWarns:  0,
			expectedOK:     0,
			checkMessages:  []string{"artifact[0] must be string or object"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewValidator("test.ndjson")
			reader := strings.NewReader(tt.content)

			result, err := validator.ValidateFile(reader)
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

			// Check that expected messages are present
			allMessages := []string{}
			for _, line := range result.Lines {
				for _, issue := range line.Issues {
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

func TestValidTimestampFormats(t *testing.T) {
	validTimestamps := []string{
		"2025-09-26T00:12:34Z",
		"2025-09-26T00:12:34.1Z",
		"2025-09-26T00:12:34.12Z",
		"2025-09-26T00:12:34.123Z",
		"2025-09-26T00:12:34.1234Z",
		"2025-09-26T00:12:34.12345Z",
		"2025-09-26T00:12:34.123456Z",
		"2025-09-26T00:12:34.1234567Z",
		"2025-09-26T00:12:34.12345678Z",
		"2025-09-26T00:12:34.123456789Z",
	}

	for _, ts := range validTimestamps {
		t.Run("valid_"+ts, func(t *testing.T) {
			content := `{"ts":"` + ts + `","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}`

			validator := NewValidator("test.ndjson")
			reader := strings.NewReader(content)
			result, err := validator.ValidateFile(reader)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Summary.Error > 0 {
				t.Errorf("timestamp %s should be valid, but got errors: %d", ts, result.Summary.Error)
			}
		})
	}
}

func TestInvalidTimestampFormats(t *testing.T) {
	invalidTimestamps := []string{
		"2025-09-26T00:12:34.123456789+09:00", // Not UTC
		"2025-09-26T00:12:34.123456789",       // Missing Z
		"2025-09-26 00:12:34Z",                // Space instead of T
		"2025-09-26",                          // Date only
		"00:12:34Z",                           // Time only
		"invalid",                             // Not a timestamp
		"",                                    // Empty string
	}

	for _, ts := range invalidTimestamps {
		t.Run("invalid_"+ts, func(t *testing.T) {
			content := `{"ts":"` + ts + `","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}`

			validator := NewValidator("test.ndjson")
			reader := strings.NewReader(content)
			result, err := validator.ValidateFile(reader)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Summary.Error == 0 {
				t.Errorf("timestamp %s should be invalid, but got no errors", ts)
			}
		})
	}
}

func TestJournalSummaryConsistency(t *testing.T) {
	content := `{"ts":"2025-09-26T00:12:34.123456789Z","turn":1,"step":"plan","decision":"OK","elapsed_ms":1500,"error":"","artifacts":[]}
{"ts":"2025-09-26T00:15:34.123456789Z","turn":0,"step":"implement","decision":"OK","elapsed_ms":2300,"error":"","artifacts":[]}
{"ts":"2025-09-26T00:18:34.123456789Z","turn":-1,"step":"invalid","decision":"MAYBE","elapsed_ms":-100,"error":"","artifacts":[]}`

	validator := NewValidator("test.ndjson")
	reader := strings.NewReader(content)
	result, err := validator.ValidateFile(reader)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that summary counts are consistent
	totalCounts := result.Summary.OK + result.Summary.Warn + result.Summary.Error
	if totalCounts != result.Summary.Lines {
		t.Errorf("summary inconsistent: lines=%d but ok+warn+error=%d", result.Summary.Lines, totalCounts)
	}

	// Check that number of processed lines matches actual lines
	expectedLines := 3 // 3 JSON lines
	if result.Summary.Lines != expectedLines {
		t.Errorf("expected %d lines processed, got %d", expectedLines, result.Summary.Lines)
	}
}