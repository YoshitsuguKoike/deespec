package cli

import (
	"testing"
)

// Test simple command creation functions to boost coverage
func TestMoreCommandConstructors(t *testing.T) {
	// Test newInitCmd
	t.Run("newInitCmd", func(t *testing.T) {
		cmd := newInitCmd()
		if cmd == nil {
			t.Fatal("newInitCmd returned nil")
		}
		if cmd.Use != "init" {
			t.Errorf("Expected Use='init', got %q", cmd.Use)
		}
	})

	// Test newStatusCmd
	t.Run("newStatusCmd", func(t *testing.T) {
		cmd := newStatusCmd()
		if cmd == nil {
			t.Fatal("newStatusCmd returned nil")
		}
		if cmd.Use != "status" {
			t.Errorf("Expected Use='status', got %q", cmd.Use)
		}
	})

	// Test newHealthCmd
	t.Run("newHealthCmd", func(t *testing.T) {
		cmd := newHealthCmd()
		if cmd == nil {
			t.Fatal("newHealthCmd returned nil")
		}
		if cmd.Use != "health" {
			t.Errorf("Expected Use='health', got %q", cmd.Use)
		}
	})

	// Test newJournalCmd
	t.Run("newJournalCmd", func(t *testing.T) {
		cmd := newJournalCmd()
		if cmd == nil {
			t.Fatal("newJournalCmd returned nil")
		}
		if cmd.Use != "journal" {
			t.Errorf("Expected Use='journal', got %q", cmd.Use)
		}
	})

	// Test newStateCmd
	t.Run("newStateCmd", func(t *testing.T) {
		cmd := newStateCmd()
		if cmd == nil {
			t.Fatal("newStateCmd returned nil")
		}
		if cmd.Use != "state" {
			t.Errorf("Expected Use='state', got %q", cmd.Use)
		}
	})

	// Test newDoctorCmd
	t.Run("newDoctorCmd", func(t *testing.T) {
		cmd := newDoctorCmd()
		if cmd == nil {
			t.Fatal("newDoctorCmd returned nil")
		}
		if cmd.Use != "doctor" {
			t.Errorf("Expected Use='doctor', got %q", cmd.Use)
		}
	})

	// Test newDoctorIntegratedCmd
	t.Run("newDoctorIntegratedCmd", func(t *testing.T) {
		cmd := newDoctorIntegratedCmd()
		if cmd == nil {
			t.Fatal("newDoctorIntegratedCmd returned nil")
		}
		if cmd.Use != "doctor" {
			t.Errorf("Expected Use='doctor', got %q", cmd.Use)
		}
	})

	// Test newLabelCmd
	t.Run("newLabelCmd", func(t *testing.T) {
		cmd := newLabelCmd()
		if cmd == nil {
			t.Fatal("newLabelCmd returned nil")
		}
		if cmd.Use != "label" {
			t.Errorf("Expected Use='label', got %q", cmd.Use)
		}
	})

	// Test NewSBIRunCommand
	t.Run("NewSBIRunCommand", func(t *testing.T) {
		cmd := NewSBIRunCommand()
		if cmd == nil {
			t.Fatal("NewSBIRunCommand returned nil")
		}
		if cmd.Use != "run" {
			t.Errorf("Expected Use='run', got %q", cmd.Use)
		}
		// Check for required flags
		if cmd.Flags().Lookup("once") == nil {
			t.Error("Expected --once flag to be defined (deprecated)")
		}
		if cmd.Flags().Lookup("auto-fb") == nil {
			t.Error("Expected --auto-fb flag to be defined")
		}
		if cmd.Flags().Lookup("interval") == nil {
			t.Error("Expected --interval flag to be defined")
		}
	})

}
