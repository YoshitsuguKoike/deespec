package common

import (
	"testing"
)

func TestLoggerFunctions(t *testing.T) {
	// Test InitGlobalLogger first
	InitGlobalLogger("info")

	// Test Info function
	Info("Test info message %s", "arg")

	// Test Warn function
	Warn("Test warning message %d", 42)

	// Test Error function (non-fatal)
	Error("Test error message")

	// Test Debug function
	Debug("Test debug message")

	// Test GetLogger
	logger := GetLogger()
	if logger == nil {
		t.Error("GetLogger should not return nil")
	}

	// Test InitializeLoggers
	InitializeLoggers(logger)

	// Test with different log levels
	InitGlobalLogger("debug")
	Debug("Debug level message")

	InitGlobalLogger("warn")
	Warn("Warn level message")

	InitGlobalLogger("error")
	Error("Error level message")
}
