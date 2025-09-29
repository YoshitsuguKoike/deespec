package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
)

func TestClear(t *testing.T) {
	// Setup test directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create .deespec structure
	paths := app.Paths{
		Home:      filepath.Join(tmpDir, ".deespec"),
		Var:       filepath.Join(tmpDir, ".deespec", "var"),
		State:     filepath.Join(tmpDir, ".deespec", "var", "state.json"),
		Journal:   filepath.Join(tmpDir, ".deespec", "var", "journal.ndjson"),
		Health:    filepath.Join(tmpDir, ".deespec", "var", "health.json"),
		StateLock: filepath.Join(tmpDir, ".deespec", "var", "state.lock"),
	}

	// Create directories
	os.MkdirAll(paths.Var, 0755)
	os.MkdirAll(filepath.Join(".deespec", "specs", "sbi"), 0755)

	// Create initial state without WIP
	initialState := &State{
		Version: 1,
		Current: "",
		Status:  "",
		Turn:    5,
		WIP:     "",
		Inputs:  map[string]string{"test": "value"},
	}
	stateData, _ := json.Marshal(initialState)
	os.WriteFile(paths.State, stateData, 0644)

	// Create journal file
	journalData := `{"ts":"2024-01-01T00:00:00Z","turn":1,"step":"test","ok":true}
{"ts":"2024-01-01T00:00:01Z","turn":2,"step":"test","ok":true}`
	os.WriteFile(paths.Journal, []byte(journalData), 0644)

	// Create test spec files
	specDir := filepath.Join(".deespec", "specs", "sbi", "TEST-001")
	os.MkdirAll(specDir, 0755)
	os.WriteFile(filepath.Join(specDir, "spec.md"), []byte("test spec"), 0644)

	t.Run("successful clear without WIP", func(t *testing.T) {
		opts := ClearOptions{Prune: false}
		err := Clear(paths, opts)
		if err != nil {
			t.Errorf("Clear failed: %v", err)
		}

		// Check archive was created
		archives, _ := os.ReadDir(filepath.Join(".deespec", "archives"))
		if len(archives) == 0 {
			t.Error("No archive directory created")
		}

		// Check state was reset
		newState, _ := loadState(paths.State)
		if newState.Turn != 0 {
			t.Errorf("State turn not reset, got %d", newState.Turn)
		}
		if newState.WIP != "" {
			t.Errorf("State WIP not cleared, got %s", newState.WIP)
		}

		// Check journal was cleared
		journalInfo, _ := os.Stat(paths.Journal)
		if journalInfo.Size() != 0 {
			t.Error("Journal file not cleared")
		}

		// Check specs were moved
		_, err = os.Stat(filepath.Join(".deespec", "specs", "sbi", "TEST-001"))
		if !os.IsNotExist(err) {
			t.Error("Spec directory not moved to archive")
		}
	})
}

func TestClear_WithWIP(t *testing.T) {
	// Setup test directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	paths := app.Paths{
		Home:    filepath.Join(tmpDir, ".deespec"),
		Var:     filepath.Join(tmpDir, ".deespec", "var"),
		State:   filepath.Join(tmpDir, ".deespec", "var", "state.json"),
		Journal: filepath.Join(tmpDir, ".deespec", "var", "journal.ndjson"),
		Health:  filepath.Join(tmpDir, ".deespec", "var", "health.json"),
	}

	os.MkdirAll(paths.Var, 0755)

	t.Run("WIP without lease should succeed", func(t *testing.T) {
		// Create state with WIP but no lease
		stateWithWIP := &State{
			Version: 1,
			WIP:     "TASK-001",
			Turn:    3,
		}
		stateData, _ := json.Marshal(stateWithWIP)
		os.WriteFile(paths.State, stateData, 0644)

		opts := ClearOptions{Prune: false}
		err := Clear(paths, opts)

		// Should succeed with WIP but no lease
		if err != nil {
			t.Errorf("Should succeed with WIP but no lease: %v", err)
		}
	})

	t.Run("WIP with active lease should fail", func(t *testing.T) {
		// Create state with WIP and active lease
		stateWithWIPAndLease := &State{
			Version:        1,
			WIP:            "TASK-002",
			Turn:           3,
			LeaseExpiresAt: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		stateData, _ := json.Marshal(stateWithWIPAndLease)
		os.WriteFile(paths.State, stateData, 0644)

		opts := ClearOptions{Prune: false}
		err := Clear(paths, opts)

		if err == nil {
			t.Error("Expected error when WIP with active lease exists, got nil")
		}
		if !strings.Contains(err.Error(), "active lease") {
			t.Errorf("Expected 'active lease' error, got: %v", err)
		}
	})
}

func TestClear_WithActiveLease(t *testing.T) {
	// Setup test directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	paths := app.Paths{
		Home:    filepath.Join(tmpDir, ".deespec"),
		Var:     filepath.Join(tmpDir, ".deespec", "var"),
		State:   filepath.Join(tmpDir, ".deespec", "var", "state.json"),
		Journal: filepath.Join(tmpDir, ".deespec", "var", "journal.ndjson"),
		Health:  filepath.Join(tmpDir, ".deespec", "var", "health.json"),
	}

	os.MkdirAll(paths.Var, 0755)

	// Create state with active lease
	futureTime := time.Now().Add(1 * time.Hour).Format(time.RFC3339)
	stateWithLease := &State{
		Version:        1,
		WIP:            "",
		LeaseExpiresAt: futureTime,
	}
	stateData, _ := json.Marshal(stateWithLease)
	os.WriteFile(paths.State, stateData, 0644)

	opts := ClearOptions{Prune: false}
	err := Clear(paths, opts)

	if err == nil {
		t.Error("Expected error when active lease exists, got nil")
	}
	if !strings.Contains(err.Error(), "active lease") {
		t.Errorf("Expected 'active lease' error, got: %v", err)
	}
}

func TestClear_NoState(t *testing.T) {
	// Setup test directory
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	paths := app.Paths{
		Home:    filepath.Join(tmpDir, ".deespec"),
		Var:     filepath.Join(tmpDir, ".deespec", "var"),
		State:   filepath.Join(tmpDir, ".deespec", "var", "state.json"),
		Journal: filepath.Join(tmpDir, ".deespec", "var", "journal.ndjson"),
		Health:  filepath.Join(tmpDir, ".deespec", "var", "health.json"),
	}

	os.MkdirAll(paths.Var, 0755)

	// No state file exists - should proceed
	opts := ClearOptions{Prune: false}
	err := Clear(paths, opts)

	if err != nil {
		t.Errorf("Clear should succeed when no state exists: %v", err)
	}

	// Check archive was created
	archives, _ := os.ReadDir(filepath.Join(".deespec", "archives"))
	if len(archives) == 0 {
		t.Error("No archive directory created")
	}
}

func TestArchiveJournal(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	archiveDir := filepath.Join(tmpDir, "archive")
	os.MkdirAll(archiveDir, 0755)

	// Create test journal
	testData := "test journal entry\n"
	os.WriteFile(journalPath, []byte(testData), 0644)

	err := archiveJournal(journalPath, archiveDir)
	if err != nil {
		t.Errorf("archiveJournal failed: %v", err)
	}

	// Check archived file exists
	archivedPath := filepath.Join(archiveDir, "journal.ndjson")
	archivedData, err := os.ReadFile(archivedPath)
	if err != nil {
		t.Errorf("Failed to read archived journal: %v", err)
	}
	if string(archivedData) != testData {
		t.Error("Archived journal content doesn't match")
	}

	// Check original was cleared
	originalInfo, _ := os.Stat(journalPath)
	if originalInfo.Size() != 0 {
		t.Error("Original journal not cleared")
	}
}

func TestArchiveJournal_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	archiveDir := filepath.Join(tmpDir, "archive")
	os.MkdirAll(archiveDir, 0755)

	// No journal file exists
	err := archiveJournal(journalPath, archiveDir)
	if err != nil {
		t.Errorf("archiveJournal should succeed when file doesn't exist: %v", err)
	}
}

func TestArchiveSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Create specs directory with content
	specsDir := filepath.Join(".deespec", "specs", "sbi")
	os.MkdirAll(specsDir, 0755)
	testFile := filepath.Join(specsDir, "TEST-001.md")
	os.WriteFile(testFile, []byte("test spec"), 0644)

	archiveDir := filepath.Join(".deespec", "archives", "test")
	os.MkdirAll(archiveDir, 0755)

	err := archiveSpecs(archiveDir)
	if err != nil {
		t.Errorf("archiveSpecs failed: %v", err)
	}

	// Check file was moved
	_, err = os.Stat(testFile)
	if !os.IsNotExist(err) {
		t.Error("Original spec file not moved")
	}

	// Check archived file exists
	archivedFile := filepath.Join(archiveDir, "specs", "sbi", "TEST-001.md")
	if _, err := os.Stat(archivedFile); os.IsNotExist(err) {
		t.Error("Spec file not in archive")
	}
}

func TestArchiveSpecs_NoSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	archiveDir := filepath.Join(".deespec", "archives", "test")
	os.MkdirAll(archiveDir, 0755)

	// No specs directory
	err := archiveSpecs(archiveDir)
	if err != nil {
		t.Errorf("archiveSpecs should succeed when no specs exist: %v", err)
	}
}
