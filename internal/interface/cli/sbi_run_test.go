package cli

import (
	"testing"

	"go.uber.org/goleak"
)

func TestNewSBIRunCommand(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))
	// Test that NewSBIRunCommand creates a valid command
	cmd := NewSBIRunCommand()
	if cmd == nil {
		t.Fatal("NewSBIRunCommand returned nil")
	}
	if cmd.Use != "run" {
		t.Errorf("Expected Use='run', got %q", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Expected non-empty Short description")
	}

	// Check that it has the required flags
	onceFlag := cmd.Flags().Lookup("once")
	if onceFlag == nil {
		t.Error("Expected --once flag to be defined (deprecated)")
	}

	autoFBFlag := cmd.Flags().Lookup("auto-fb")
	if autoFBFlag == nil {
		t.Error("Expected --auto-fb flag to be defined")
	}

	intervalFlag := cmd.Flags().Lookup("interval")
	if intervalFlag == nil {
		t.Error("Expected --interval flag to be defined")
	}
}

func TestNewSBIRunCommand_NotNil(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))
	// Test that the command constructor doesn't return nil
	tests := []struct {
		name     string
		cmdFunc  func() interface{}
		checkNil bool
	}{
		{
			name: "NewSBIRunCommand",
			cmdFunc: func() interface{} {
				return NewSBIRunCommand()
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
