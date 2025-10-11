package clear

import (
	"os"
	"path/filepath"
	"testing"
)

// Note: Main Clear() tests have been removed as they relied on state.json
// The new DB-based implementation requires integration tests with a real database.
// Archive-related tests are retained as they don't depend on state.json.

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
