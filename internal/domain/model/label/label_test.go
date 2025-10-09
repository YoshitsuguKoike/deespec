package label

import (
	"testing"
	"time"
)

func TestNewLabel(t *testing.T) {
	name := "test-label"
	description := "Test description"
	templatePaths := []string{"template1.md", "template2.md"}
	priority := 5

	lbl := NewLabel(name, description, templatePaths, priority)

	if lbl.Name() != name {
		t.Errorf("Name() = %v, want %v", lbl.Name(), name)
	}
	if lbl.Description() != description {
		t.Errorf("Description() = %v, want %v", lbl.Description(), description)
	}
	if len(lbl.TemplatePaths()) != len(templatePaths) {
		t.Errorf("TemplatePaths() length = %v, want %v", len(lbl.TemplatePaths()), len(templatePaths))
	}
	if lbl.Priority() != priority {
		t.Errorf("Priority() = %v, want %v", lbl.Priority(), priority)
	}
	if !lbl.IsActive() {
		t.Error("IsActive() = false, want true")
	}
	if lbl.LineCount() != 0 {
		t.Errorf("LineCount() = %v, want 0", lbl.LineCount())
	}
}

func TestReconstructLabel(t *testing.T) {
	id := 123
	name := "reconstructed-label"
	contentHashes := map[string]string{
		"template1.md": "hash1",
		"template2.md": "hash2",
	}
	parentID := 456
	lineCount := 100
	lastSyncedAt := time.Now()

	lbl := ReconstructLabel(
		id, name, "description", []string{"template1.md"},
		contentHashes, &parentID, "#FF0000", 3, true,
		lineCount, lastSyncedAt, "{}", time.Now(), time.Now(),
	)

	if lbl.ID() != id {
		t.Errorf("ID() = %v, want %v", lbl.ID(), id)
	}
	if lbl.Name() != name {
		t.Errorf("Name() = %v, want %v", lbl.Name(), name)
	}
	if lbl.LineCount() != lineCount {
		t.Errorf("LineCount() = %v, want %v", lbl.LineCount(), lineCount)
	}
	if lbl.ParentLabelID() == nil || *lbl.ParentLabelID() != parentID {
		t.Errorf("ParentLabelID() = %v, want %v", lbl.ParentLabelID(), parentID)
	}

	hash, exists := lbl.GetContentHash("template1.md")
	if !exists {
		t.Error("GetContentHash() returned false for existing hash")
	}
	if hash != "hash1" {
		t.Errorf("GetContentHash() = %v, want hash1", hash)
	}
}

func TestSetContentHash(t *testing.T) {
	lbl := NewLabel("test", "", []string{}, 0)

	lbl.SetContentHash("file1.md", "newhash123")

	hash, exists := lbl.GetContentHash("file1.md")
	if !exists {
		t.Error("SetContentHash() did not store hash")
	}
	if hash != "newhash123" {
		t.Errorf("GetContentHash() = %v, want newhash123", hash)
	}
}

func TestSetLineCount(t *testing.T) {
	lbl := NewLabel("test", "", []string{}, 0)

	lbl.SetLineCount(500)

	if lbl.LineCount() != 500 {
		t.Errorf("LineCount() = %v, want 500", lbl.LineCount())
	}
}

func TestUpdateSyncTime(t *testing.T) {
	lbl := NewLabel("test", "", []string{}, 0)
	oldTime := lbl.LastSyncedAt()

	time.Sleep(10 * time.Millisecond)
	lbl.UpdateSyncTime()

	if !lbl.LastSyncedAt().After(oldTime) {
		t.Error("UpdateSyncTime() did not update time")
	}
}

func TestActivateDeactivate(t *testing.T) {
	lbl := NewLabel("test", "", []string{}, 0)

	// Initially active
	if !lbl.IsActive() {
		t.Error("NewLabel() should create active label")
	}

	lbl.Deactivate()
	if lbl.IsActive() {
		t.Error("Deactivate() did not deactivate label")
	}

	lbl.Activate()
	if !lbl.IsActive() {
		t.Error("Activate() did not activate label")
	}
}

func TestAddRemoveTemplatePath(t *testing.T) {
	lbl := NewLabel("test", "", []string{"initial.md"}, 0)

	lbl.AddTemplatePath("new.md")
	paths := lbl.TemplatePaths()
	if len(paths) != 2 {
		t.Errorf("AddTemplatePath() resulted in %d paths, want 2", len(paths))
	}

	// Adding duplicate should not increase count
	lbl.AddTemplatePath("new.md")
	paths = lbl.TemplatePaths()
	if len(paths) != 2 {
		t.Errorf("AddTemplatePath() duplicate resulted in %d paths, want 2", len(paths))
	}

	lbl.RemoveTemplatePath("initial.md")
	paths = lbl.TemplatePaths()
	if len(paths) != 1 {
		t.Errorf("RemoveTemplatePath() resulted in %d paths, want 1", len(paths))
	}
	if paths[0] != "new.md" {
		t.Errorf("RemoveTemplatePath() left wrong path: %v", paths[0])
	}
}

func TestClearContentHashes(t *testing.T) {
	contentHashes := map[string]string{
		"file1.md": "hash1",
		"file2.md": "hash2",
	}
	lbl := ReconstructLabel(
		1, "test", "", []string{}, contentHashes, nil,
		"", 0, true, 0, time.Now(), "", time.Now(), time.Now(),
	)

	lbl.ClearContentHashes()

	if len(lbl.ContentHashes()) != 0 {
		t.Errorf("ClearContentHashes() left %d hashes, want 0", len(lbl.ContentHashes()))
	}
}

func TestSetParentLabelID(t *testing.T) {
	lbl := NewLabel("test", "", []string{}, 0)

	if lbl.ParentLabelID() != nil {
		t.Error("NewLabel() should have nil ParentLabelID")
	}

	parentID := 999
	lbl.SetParentLabelID(&parentID)

	if lbl.ParentLabelID() == nil {
		t.Error("SetParentLabelID() did not set parent ID")
	}
	if *lbl.ParentLabelID() != parentID {
		t.Errorf("ParentLabelID() = %v, want %v", *lbl.ParentLabelID(), parentID)
	}

	lbl.SetParentLabelID(nil)
	if lbl.ParentLabelID() != nil {
		t.Error("SetParentLabelID(nil) did not clear parent ID")
	}
}
