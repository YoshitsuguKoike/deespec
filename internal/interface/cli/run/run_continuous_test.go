package run

import (
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"go.uber.org/goleak"
)

func TestParseInterval(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))

	tests := []struct {
		name     string
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			name:     "Empty string uses default",
			input:    "",
			expected: service.DefaultRunInterval,
			hasError: false,
		},
		{
			name:     "Valid duration",
			input:    "10s",
			expected: 10 * time.Second,
			hasError: false,
		},
		{
			name:     "Below minimum uses minimum",
			input:    "500ms",
			expected: service.MinInterval,
			hasError: false,
		},
		{
			name:     "Above maximum uses maximum",
			input:    "1h",
			expected: service.MaxInterval,
			hasError: false,
		},
		{
			name:     "Invalid format",
			input:    "invalid",
			expected: 0,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseInterval(tt.input)
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected %v, got %v", tt.expected, result)
				}
			}
		})
	}
}

func TestSetupSignalHandler(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli/run.SetupSignalHandler.func1"))

	// Test that signal handler setup doesn't panic
	ctx, cancel := SetupSignalHandler()
	if ctx == nil {
		t.Error("Expected non-nil context")
	}
	if cancel == nil {
		t.Error("Expected non-nil cancel function")
	}

	// Clean up
	cancel()

	// Give some time for goroutines to clean up
	time.Sleep(50 * time.Millisecond)
}
