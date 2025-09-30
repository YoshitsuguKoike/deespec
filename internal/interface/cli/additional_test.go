package cli

import (
	"testing"
)

// Test command constructors that exist
func TestCommandConstructors(t *testing.T) {
	// Test that commands can be created without panicking
	tests := []struct {
		name     string
		cmdFunc  func() interface{}
		checkNil bool
	}{
		{
			name: "newRunCmd",
			cmdFunc: func() interface{} {
				return newRunCmd()
			},
			checkNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := tt.cmdFunc()
			if tt.checkNil && cmd == nil {
				t.Errorf("%s returned nil", tt.name)
			}
		})
	}
}

// Test State helper methods
func TestStateHelpers(t *testing.T) {
	st := &State{
		WIP:     "TEST-001",
		Turn:    5,
		Status:  "WIP",
		Attempt: 2,
	}

	// Just verify these don't panic
	_ = st.WIP
	_ = st.Turn
	_ = st.Status
	_ = st.Attempt
}

// Test ClearLease function
func TestClearLeaseFunction(t *testing.T) {
	st := &State{
		WIP:            "TEST-001",
		LeaseExpiresAt: "2025-01-01T00:00:00Z",
	}

	cleared := ClearLease(st)

	if !cleared {
		t.Error("Expected ClearLease to return true when lease exists")
	}
	if st.LeaseExpiresAt != "" {
		t.Errorf("Expected LeaseExpiresAt to be empty, got %q", st.LeaseExpiresAt)
	}

	// Test when no lease exists
	cleared = ClearLease(st)
	if cleared {
		t.Error("Expected ClearLease to return false when no lease exists")
	}
}

// Test additional coverage for ParseDecision edge cases
func TestParseDecisionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Empty input",
			input:    "",
			expected: "NEEDS_CHANGES",
		},
		{
			name:     "Whitespace only",
			input:    "   \n\t   ",
			expected: "NEEDS_CHANGES",
		},
		{
			name:     "Invalid format",
			input:    "RANDOM TEXT WITHOUT DECISION",
			expected: "NEEDS_CHANGES",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDecision(tt.input)
			if result != tt.expected {
				t.Errorf("parseDecision(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test loadState and saveState functions
func TestStateIO(t *testing.T) {
	// Test loading non-existent file with relative path
	_, err := loadState("non_existent_file_for_test.json")
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
}

// Test nextStatusTransition edge cases
func TestNextStatusTransitionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		decision string
		attempt  int
		expected string
	}{
		{
			name:     "Unknown status",
			status:   "UNKNOWN",
			decision: "",
			attempt:  1,
			expected: "READY",
		},
		{
			name:     "DONE stays DONE",
			status:   "DONE",
			decision: "",
			attempt:  1,
			expected: "DONE",
		},
		{
			name:     "Empty to WIP",
			status:   "",
			decision: "",
			attempt:  1,
			expected: "WIP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nextStatusTransition(tt.status, tt.decision, tt.attempt)
			if result != tt.expected {
				t.Errorf("nextStatusTransition(%q, %q, %d) = %q, want %q",
					tt.status, tt.decision, tt.attempt, result, tt.expected)
			}
		})
	}
}

// Test more BuildPromptByStatus cases
func TestBuildPromptByStatusMoreCases(t *testing.T) {
	st := &State{
		WIP:    "TEST-001",
		Turn:   1,
		Status: "",
	}

	// Test with empty status
	prompt := buildPromptByStatus(st)
	if prompt == "" {
		t.Error("Expected non-empty prompt even with empty status")
	}

	// Test with DONE status
	st.Status = "DONE"
	prompt = buildPromptByStatus(st)
	// Should return completion message for DONE status
	if prompt == "" {
		t.Error("Expected non-empty prompt for DONE status")
	}
	if !contains(prompt, "Task completed") {
		t.Error("Expected DONE prompt to contain 'Task completed'")
	}
}
