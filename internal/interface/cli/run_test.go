package cli

import (
	"testing"
)

func TestParseDecision(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Simple SUCCEEDED",
			input: `Review complete

DECISION: SUCCEEDED

Task is ready.`,
			expected: "SUCCEEDED",
		},
		{
			name: "Simple FAILED",
			input: `Review complete

DECISION: FAILED

Multiple issues found.`,
			expected: "FAILED",
		},
		{
			name: "Simple NEEDS_CHANGES",
			input: `Review complete

DECISION: NEEDS_CHANGES

Minor updates required.`,
			expected: "NEEDS_CHANGES",
		},
		{
			name: "FAILED with leading asterisks",
			input: `Review complete

**DECISION: FAILED**

Multiple issues found.`,
			expected: "FAILED",
		},
		{
			name: "SUCCEEDED with trailing asterisks",
			input: `Review complete

DECISION: SUCCEEDED***

Perfect implementation.`,
			expected: "SUCCEEDED",
		},
		{
			name: "Mixed case decision",
			input: `Review complete

DECISION: Succeeded

Good job.`,
			expected: "SUCCEEDED",
		},
		{
			name: "Decision with extra whitespace",
			input: `Review complete

   DECISION:   FAILED

Issues found.`,
			expected: "FAILED",
		},
		{
			name: "No decision found - default to NEEDS_CHANGES",
			input: `Review complete

No explicit decision provided in this review.`,
			expected: "NEEDS_CHANGES",
		},
		{
			name: "Invalid decision value - default to NEEDS_CHANGES",
			input: `Review complete

DECISION: INVALID_VALUE

This should default.`,
			expected: "NEEDS_CHANGES",
		},
		{
			name:     "Decision in middle of text",
			input:    `The review shows that DECISION: SUCCEEDED is the right choice.`,
			expected: "SUCCEEDED",
		},
		{
			name: "Multiple decisions - takes first",
			input: `First review
DECISION: FAILED
Second review
DECISION: SUCCEEDED`,
			expected: "FAILED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDecision(tt.input)
			if result != tt.expected {
				t.Errorf("parseDecision() = %v, want %v", result, tt.expected)
			}
		})
	}
}
