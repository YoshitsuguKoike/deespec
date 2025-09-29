package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLabelCommands(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Setup test environment
	setupTestLabelEnvironment(t)

	t.Run("SetLabels", func(t *testing.T) {
		// Create test state with WIP
		st := &State{
			WIP: "SBI-TEST-001",
		}
		SaveState(st)

		// Set labels
		err := setLabels([]string{"test", "unit", "cli"}, false)
		if err != nil {
			t.Fatalf("Failed to set labels: %v", err)
		}

		// Verify labels were saved
		meta := loadTestMeta(t, "SBI-TEST-001")
		if len(meta.Labels) != 3 {
			t.Errorf("Expected 3 labels, got %d", len(meta.Labels))
		}
	})

	t.Run("AddLabels", func(t *testing.T) {
		// Add more labels
		err := setLabels([]string{"additional"}, true)
		if err != nil {
			t.Fatalf("Failed to add labels: %v", err)
		}

		// Verify labels were appended
		meta := loadTestMeta(t, "SBI-TEST-001")
		if len(meta.Labels) != 4 {
			t.Errorf("Expected 4 labels after adding, got %d", len(meta.Labels))
		}
	})

	t.Run("ListLabels", func(t *testing.T) {
		// Update index
		err := updateLabelIndex()
		if err != nil {
			t.Fatalf("Failed to update index: %v", err)
		}

		// List labels (basic functionality test)
		err = listLabels(false)
		if err != nil {
			t.Fatalf("Failed to list labels: %v", err)
		}

		// Verify index was created
		indexPath := filepath.Join(".deespec", "var", "labels.json")
		if _, err := os.Stat(indexPath); os.IsNotExist(err) {
			t.Error("Label index file was not created")
		}
	})

	t.Run("SearchByLabels", func(t *testing.T) {
		// Search for SBIs with "test" label
		err := searchByLabels([]string{"test"}, false)
		if err != nil {
			t.Fatalf("Failed to search by labels: %v", err)
		}
	})

	t.Run("DeleteLabels", func(t *testing.T) {
		// Delete specific labels
		err := deleteLabels([]string{"unit", "cli"})
		if err != nil {
			t.Fatalf("Failed to delete labels: %v", err)
		}

		// Verify labels were removed
		meta := loadTestMeta(t, "SBI-TEST-001")
		if len(meta.Labels) != 2 {
			t.Errorf("Expected 2 labels after deletion, got %d", len(meta.Labels))
		}
	})

	t.Run("ClearLabels", func(t *testing.T) {
		// Clear all labels
		err := clearLabels()
		if err != nil {
			t.Fatalf("Failed to clear labels: %v", err)
		}

		// Verify all labels were removed
		meta := loadTestMeta(t, "SBI-TEST-001")
		if len(meta.Labels) != 0 {
			t.Errorf("Expected 0 labels after clearing, got %d", len(meta.Labels))
		}
	})

	t.Run("NoWIPError", func(t *testing.T) {
		// Delete the state file and recreate without WIP
		os.Remove(".deespec/var/state.json")

		// Create new state without WIP
		st := &State{
			Version: 0,
			WIP:     "", // No WIP
			Status:  "READY",
		}

		// Save directly using saveStateCAS
		err := saveStateCAS(".deespec/var/state.json", st, 0)
		if err != nil {
			t.Fatalf("Failed to save state: %v", err)
		}

		// Try to set labels without WIP
		err = setLabels([]string{"test"}, false)
		if err == nil {
			t.Error("Expected error when no WIP is selected")
		}
		if err != nil && err.Error() != "no WIP SBI currently selected" {
			t.Errorf("Unexpected error message: %v", err)
		}
	})
}

func TestLabelIndex(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	t.Run("LoadEmptyIndex", func(t *testing.T) {
		index, err := loadLabelIndex()
		if err != nil {
			t.Fatalf("Failed to load empty index: %v", err)
		}
		if len(index.Labels) != 0 {
			t.Error("Expected empty index")
		}
	})

	t.Run("SaveAndLoadIndex", func(t *testing.T) {
		// Create test index
		index := &LabelIndex{
			Labels: map[string][]string{
				"test":     {"SBI-001", "SBI-002"},
				"critical": {"SBI-001"},
			},
		}

		// Ensure var directory exists
		os.MkdirAll(".deespec/var", 0755)

		// Save index
		err := saveLabelIndex(index)
		if err != nil {
			t.Fatalf("Failed to save index: %v", err)
		}

		// Load and verify
		loaded, err := loadLabelIndex()
		if err != nil {
			t.Fatalf("Failed to load index: %v", err)
		}

		if len(loaded.Labels) != 2 {
			t.Errorf("Expected 2 labels in index, got %d", len(loaded.Labels))
		}

		if len(loaded.Labels["test"]) != 2 {
			t.Errorf("Expected 2 SBIs for 'test' label, got %d", len(loaded.Labels["test"]))
		}
	})

	t.Run("UpdateIndex", func(t *testing.T) {
		// Create test SBI structure
		setupTestLabelEnvironment(t)

		// Create SBI with labels
		sbiDir := filepath.Join(".deespec", "SBI-UPDATE-001")
		os.MkdirAll(sbiDir, 0755)

		meta := &SBIMeta{
			ID:          "SBI-UPDATE-001",
			Description: "Test SBI",
			Status:      "WIP",
			Labels:      []string{"update", "test"},
		}

		metaPath := filepath.Join(sbiDir, "meta.yml")
		data, _ := yaml.Marshal(meta)
		os.WriteFile(metaPath, data, 0644)

		// Update index
		err := updateLabelIndex()
		if err != nil {
			t.Fatalf("Failed to update index: %v", err)
		}

		// Verify index contains the labels
		index, err := loadLabelIndex()
		if err != nil {
			t.Fatalf("Failed to load updated index: %v", err)
		}

		if _, ok := index.Labels["update"]; !ok {
			t.Error("Index doesn't contain 'update' label")
		}

		found := false
		for _, sbi := range index.Labels["update"] {
			if sbi == "SBI-UPDATE-001" {
				found = true
				break
			}
		}
		if !found {
			t.Error("SBI-UPDATE-001 not found in 'update' label")
		}
	})
}

func TestSBIMetaOperations(t *testing.T) {
	tmpDir := t.TempDir()
	metaPath := filepath.Join(tmpDir, "test_meta.yml")

	t.Run("SaveAndLoad", func(t *testing.T) {
		// Create test meta
		meta := &SBIMeta{
			ID:          "SBI-META-001",
			Description: "Test meta operations",
			Status:      "READY",
			Labels:      []string{"meta", "test"},
		}

		// Save meta
		err := saveSBIMeta(metaPath, meta)
		if err != nil {
			t.Fatalf("Failed to save meta: %v", err)
		}

		// Load and verify
		loaded, err := loadSBIMeta(metaPath)
		if err != nil {
			t.Fatalf("Failed to load meta: %v", err)
		}

		if loaded.ID != meta.ID {
			t.Errorf("ID mismatch: got %s, want %s", loaded.ID, meta.ID)
		}

		if len(loaded.Labels) != 2 {
			t.Errorf("Labels count mismatch: got %d, want 2", len(loaded.Labels))
		}
	})

	t.Run("LoadNonExistent", func(t *testing.T) {
		_, err := loadSBIMeta(filepath.Join(tmpDir, "nonexistent.yml"))
		if err == nil {
			t.Error("Expected error when loading non-existent file")
		}
	})
}

// Helper functions for tests

func setupTestLabelEnvironment(t *testing.T) {
	// Create .deespec directory structure
	os.MkdirAll(".deespec/var", 0755)
	os.MkdirAll(".deespec/SBI-TEST-001", 0755)

	// Create test meta.yml
	meta := &SBIMeta{
		ID:          "SBI-TEST-001",
		Description: "Test SBI for label commands",
		Status:      "WIP",
		Labels:      nil,
	}

	metaPath := filepath.Join(".deespec", "SBI-TEST-001", "meta.yml")
	data, err := yaml.Marshal(meta)
	if err != nil {
		t.Fatalf("Failed to marshal test meta: %v", err)
	}

	err = os.WriteFile(metaPath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test meta: %v", err)
	}
}

func loadTestMeta(t *testing.T, sbiID string) *SBIMeta {
	metaPath := filepath.Join(".deespec", sbiID, "meta.yml")
	meta, err := loadSBIMeta(metaPath)
	if err != nil {
		t.Fatalf("Failed to load meta for %s: %v", sbiID, err)
	}
	return meta
}

// Test for JSON marshaling
func TestLabelIndexJSON(t *testing.T) {
	index := &LabelIndex{
		Labels: map[string][]string{
			"performance": {"SBI-001", "SBI-002", "SBI-003"},
			"critical":    {"SBI-001"},
			"database":    {"SBI-002", "SBI-004"},
		},
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal index: %v", err)
	}

	// Unmarshal back
	var loaded LabelIndex
	err = json.Unmarshal(data, &loaded)
	if err != nil {
		t.Fatalf("Failed to unmarshal index: %v", err)
	}

	// Verify
	if len(loaded.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(loaded.Labels))
	}

	if len(loaded.Labels["performance"]) != 3 {
		t.Errorf("Expected 3 SBIs for 'performance', got %d", len(loaded.Labels["performance"]))
	}
}
