package cli

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestParseInterval(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	tests := []struct {
		name     string
		input    string
		expected time.Duration
		hasError bool
	}{
		{
			name:     "Empty string uses default",
			input:    "",
			expected: DefaultRunInterval,
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
			expected: MinInterval,
			hasError: false,
		},
		{
			name:     "Above maximum uses maximum",
			input:    "1h",
			expected: MaxInterval,
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
			result, err := parseInterval(tt.input)
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

func TestErrorClassification(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	tests := []struct {
		name     string
		err      error
		isTemp   bool
		isConfig bool
		isCrit   bool
	}{
		{
			name:     "Temporary error",
			err:      errors.New("connection refused"),
			isTemp:   true,
			isConfig: false,
			isCrit:   false,
		},
		{
			name:     "Configuration error",
			err:      errors.New("invalid config format"),
			isTemp:   false,
			isConfig: true,
			isCrit:   false,
		},
		{
			name:     "Critical error",
			err:      errors.New("out of memory"),
			isTemp:   false,
			isConfig: false,
			isCrit:   true,
		},
		{
			name:     "Unknown error",
			err:      errors.New("unknown issue"),
			isTemp:   false,
			isConfig: false,
			isCrit:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if isTemporaryError(tt.err) != tt.isTemp {
				t.Errorf("isTemporaryError() = %v, want %v", isTemporaryError(tt.err), tt.isTemp)
			}
			if isConfigurationError(tt.err) != tt.isConfig {
				t.Errorf("isConfigurationError() = %v, want %v", isConfigurationError(tt.err), tt.isConfig)
			}
			if isCriticalError(tt.err) != tt.isCrit {
				t.Errorf("isCriticalError() = %v, want %v", isCriticalError(tt.err), tt.isCrit)
			}
		})
	}
}

func TestCalculateNextInterval(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	baseInterval := 5 * time.Second

	tests := []struct {
		name              string
		consecutiveErrors int
		expectedMin       time.Duration
		expectedMax       time.Duration
	}{
		{
			name:              "No errors",
			consecutiveErrors: 0,
			expectedMin:       baseInterval,
			expectedMax:       baseInterval,
		},
		{
			name:              "One error",
			consecutiveErrors: 1,
			expectedMin:       2 * baseInterval,
			expectedMax:       2 * baseInterval,
		},
		{
			name:              "Multiple errors",
			consecutiveErrors: 3,
			expectedMin:       10 * time.Second, // Max backoff is now 10 seconds
			expectedMax:       10 * time.Second,
		},
		{
			name:              "Many errors hit max",
			consecutiveErrors: 10,
			expectedMin:       10 * time.Second, // Max backoff is now 10 seconds
			expectedMax:       10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateNextInterval(baseInterval, tt.consecutiveErrors)
			if result < tt.expectedMin || result > tt.expectedMax {
				t.Errorf("calculateNextInterval() = %v, want between %v and %v",
					result, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestErrorStats(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	stats := &ErrorStats{}

	// Test initial state
	if stats.TotalExecutions != 0 {
		t.Errorf("Expected TotalExecutions = 0, got %d", stats.TotalExecutions)
	}

	// Test statistics
	stats.TotalExecutions = 10
	stats.SuccessfulRuns = 8
	stats.TemporaryErrors = 1
	stats.ConfigErrors = 1

	// Report should not panic
	stats.Report()
}

func TestSetupSignalHandler(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	// Test that signal handler setup doesn't panic
	ctx, cancel := setupSignalHandler()
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

func TestHandleExecutionError(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	stats := &ErrorStats{}

	tests := []struct {
		name           string
		err            error
		shouldContinue bool
	}{
		{
			name:           "Temporary error should continue",
			err:            errors.New("connection refused"),
			shouldContinue: true,
		},
		{
			name:           "Config error should continue",
			err:            errors.New("invalid config"),
			shouldContinue: true,
		},
		{
			name:           "Critical error should stop",
			err:            errors.New("out of memory"),
			shouldContinue: false,
		},
		{
			name:           "Unknown error should continue",
			err:            errors.New("something went wrong"),
			shouldContinue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handleExecutionError(tt.err, stats)
			if result != tt.shouldContinue {
				t.Errorf("handleExecutionError() = %v, want %v", result, tt.shouldContinue)
			}
		})
	}
}
