package version

import (
	"testing"
)

func TestNewCommand(t *testing.T) {
	cmd := NewCommand()

	if cmd == nil {
		t.Fatal("NewCommand() returned nil")
	}

	if cmd.Use != "version" {
		t.Errorf("Expected Use='version', got '%s'", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Short description should not be empty")
	}

	if cmd.Long == "" {
		t.Error("Long description should not be empty")
	}

	if cmd.Run == nil {
		t.Error("Run function should not be nil")
	}
}

func TestVersionCommand_Properties(t *testing.T) {
	cmd := NewCommand()

	// Test command properties
	tests := []struct {
		name     string
		property string
		value    string
		isEmpty  bool
	}{
		{"Use property", "Use", cmd.Use, cmd.Use == ""},
		{"Short property", "Short", cmd.Short, cmd.Short == ""},
		{"Long property", "Long", cmd.Long, cmd.Long == ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.isEmpty {
				t.Errorf("%s should not be empty", tt.property)
			}
		})
	}
}

func TestVersionCommand_CanExecute(t *testing.T) {
	// Test that command can be executed without panicking
	// We don't capture output since it uses fmt.Printf directly,
	// but we verify the command doesn't crash
	cmd := NewCommand()

	// Create a test that will execute the Run function
	// This verifies that the command can run without errors
	if cmd.Run != nil {
		// Call Run with empty args - this should not panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Command.Run panicked: %v", r)
			}
		}()

		// Execute the command's Run function
		cmd.Run(cmd, []string{})
	}
}
