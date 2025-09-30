package cli

import (
	"testing"
)

func TestValidatePlaceholders(t *testing.T) {
	tests := []struct {
		name         string
		template     string
		stepID       string
		expectErrors bool
	}{
		{
			name:         "No placeholders",
			template:     "This is a regular template",
			stepID:       "test-step",
			expectErrors: false,
		},
		{
			name:         "Valid placeholders",
			template:     "This has {PLACEHOLDER} in it",
			stepID:       "test-step",
			expectErrors: true, // Unknown placeholders are treated as errors
		},
		{
			name:         "Empty template",
			template:     "",
			stepID:       "test-step",
			expectErrors: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, warnings := validatePlaceholders(tt.template, tt.stepID)
			if tt.expectErrors && len(errors) == 0 {
				t.Error("Expected errors but got none")
			}
			if !tt.expectErrors && len(errors) > 0 {
				t.Errorf("Unexpected errors: %v", errors)
			}
			_ = warnings
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		substr   string
		expected bool
	}{
		{
			name:     "Substring exists",
			str:      "apple banana orange",
			substr:   "banana",
			expected: true,
		},
		{
			name:     "Substring doesn't exist",
			str:      "apple banana orange",
			substr:   "grape",
			expected: false,
		},
		{
			name:     "Empty string",
			str:      "",
			substr:   "apple",
			expected: false,
		},
		{
			name:     "Empty substring",
			str:      "apple banana",
			substr:   "",
			expected: true,
		},
		{
			name:     "Case sensitive",
			str:      "Apple Banana",
			substr:   "apple",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.str, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, want %v",
					tt.str, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		substr   string
		expected bool
	}{
		{
			name:     "Substring exists",
			str:      "apple banana orange",
			substr:   "ban",
			expected: true,
		},
		{
			name:     "Substring doesn't exist",
			str:      "apple banana orange",
			substr:   "gra",
			expected: false,
		},
		{
			name:     "Empty substring",
			str:      "apple banana",
			substr:   "",
			expected: true,
		},
		{
			name:     "Empty string",
			str:      "",
			substr:   "test",
			expected: false,
		},
		{
			name:     "Full match",
			str:      "testcase",
			substr:   "testcase",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsHelper(tt.str, tt.substr)
			if result != tt.expected {
				t.Errorf("containsHelper(%q, %q) = %v, want %v",
					tt.str, tt.substr, result, tt.expected)
			}
		})
	}
}
