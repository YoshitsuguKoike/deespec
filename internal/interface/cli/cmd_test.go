package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNewClearCmd(t *testing.T) {
	cmd := newClearCmd()

	if cmd == nil {
		t.Fatal("Expected non-nil command")
	}

	if cmd.Use != "clear" {
		t.Errorf("Expected Use to be 'clear', got %s", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("Expected Short description to be set")
	}

	// Check that prune flag exists
	pruneFlag := cmd.Flags().Lookup("prune")
	if pruneFlag == nil {
		t.Error("Expected --prune flag to be registered")
	}

	// Verify it's a cobra command
	if _, ok := interface{}(cmd).(*cobra.Command); !ok {
		t.Error("Expected *cobra.Command type")
	}
}

func TestCommandsHaveRunE(t *testing.T) {
	// Test that commands have RunE functions
	clearCmd := newClearCmd()
	if clearCmd.RunE == nil {
		t.Error("Clear command missing RunE function")
	}
}

func TestCommandHelp(t *testing.T) {
	// Test that commands have help text
	clearCmd := newClearCmd()
	if clearCmd.Long == "" {
		t.Error("Clear command missing Long description")
	}
}

func TestClearOptions(t *testing.T) {
	opts := ClearOptions{
		Prune: true,
	}

	if !opts.Prune {
		t.Error("Expected Prune option to be true")
	}

	opts2 := ClearOptions{}
	if opts2.Prune {
		t.Error("Expected default Prune option to be false")
	}
}
