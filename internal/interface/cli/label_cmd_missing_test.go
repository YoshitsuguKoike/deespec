package cli

import (
	"os"
	"testing"
)

func TestSBIMetaMissing(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Setup test environment without meta.yml
	os.MkdirAll(".deespec/var", 0755)
	os.MkdirAll(".deespec/SBI-MISSING-001", 0755)

	// Create state with WIP pointing to SBI without meta.yml
	st := &State{
		WIP: "SBI-MISSING-001",
	}
	SaveState(st)

	// Try to set labels for SBI without meta.yml
	err := setLabels([]string{"test"}, false)
	if err == nil {
		t.Error("Expected error when meta.yml is missing")
	}
}
