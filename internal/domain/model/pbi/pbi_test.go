package pbi

import (
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
)

func TestNewPBI(t *testing.T) {
	metadata := PBIMetadata{
		StoryPoints:        5,
		Priority:           1,
		Labels:             []string{"feature", "backend"},
		AssignedAgent:      "claude-code",
		AcceptanceCriteria: []string{"Criterion 1", "Criterion 2"},
	}

	pbi, err := NewPBI("Test PBI", "Test description", nil, metadata)
	if err != nil {
		t.Fatalf("NewPBI failed: %v", err)
	}

	if pbi == nil {
		t.Fatal("Expected non-nil PBI")
	}

	if pbi.Title() != "Test PBI" {
		t.Errorf("Expected title 'Test PBI', got '%s'", pbi.Title())
	}

	if pbi.Description() != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", pbi.Description())
	}

	if pbi.Type() != model.TaskTypePBI {
		t.Errorf("Expected type TaskTypePBI, got %v", pbi.Type())
	}

	if pbi.Status() != model.StatusPending {
		t.Errorf("Expected status Pending, got %v", pbi.Status())
	}

	if pbi.HasSBIs() {
		t.Error("New PBI should not have any SBIs")
	}
}

func TestNewPBI_WithParentEPIC(t *testing.T) {
	parentID := model.NewTaskID()
	metadata := PBIMetadata{StoryPoints: 3}

	pbi, err := NewPBI("Child PBI", "Child description", &parentID, metadata)
	if err != nil {
		t.Fatalf("NewPBI with parent failed: %v", err)
	}

	if !pbi.HasParentEPIC() {
		t.Error("Expected PBI to have parent EPIC")
	}

	if pbi.ParentTaskID() == nil {
		t.Error("Expected non-nil parent task ID")
	}

	if !pbi.ParentTaskID().Equals(parentID) {
		t.Errorf("Expected parent ID %v, got %v", parentID, *pbi.ParentTaskID())
	}
}

func TestPBI_AddSBI(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	sbiID1 := model.NewTaskID()
	sbiID2 := model.NewTaskID()

	// Add first SBI
	err := pbi.AddSBI(sbiID1)
	if err != nil {
		t.Fatalf("Failed to add first SBI: %v", err)
	}

	if !pbi.HasSBIs() {
		t.Error("PBI should have SBIs after adding")
	}

	if pbi.SBICount() != 1 {
		t.Errorf("Expected 1 SBI, got %d", pbi.SBICount())
	}

	// Add second SBI
	err = pbi.AddSBI(sbiID2)
	if err != nil {
		t.Fatalf("Failed to add second SBI: %v", err)
	}

	if pbi.SBICount() != 2 {
		t.Errorf("Expected 2 SBIs, got %d", pbi.SBICount())
	}

	// Try to add duplicate SBI
	err = pbi.AddSBI(sbiID1)
	if err == nil {
		t.Error("Expected error when adding duplicate SBI")
	}

	if pbi.SBICount() != 2 {
		t.Error("SBI count should not change when adding duplicate")
	}
}

func TestPBI_RemoveSBI(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	sbiID1 := model.NewTaskID()
	sbiID2 := model.NewTaskID()
	sbiID3 := model.NewTaskID()

	// Add SBIs
	pbi.AddSBI(sbiID1)
	pbi.AddSBI(sbiID2)

	// Remove existing SBI
	err := pbi.RemoveSBI(sbiID1)
	if err != nil {
		t.Fatalf("Failed to remove SBI: %v", err)
	}

	if pbi.SBICount() != 1 {
		t.Errorf("Expected 1 SBI after removal, got %d", pbi.SBICount())
	}

	// Try to remove non-existent SBI
	err = pbi.RemoveSBI(sbiID3)
	if err == nil {
		t.Error("Expected error when removing non-existent SBI")
	}

	// Remove last SBI
	err = pbi.RemoveSBI(sbiID2)
	if err != nil {
		t.Fatalf("Failed to remove last SBI: %v", err)
	}

	if pbi.HasSBIs() {
		t.Error("PBI should not have SBIs after removing all")
	}
}

func TestPBI_SBIIDs(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	sbiID1 := model.NewTaskID()
	sbiID2 := model.NewTaskID()

	pbi.AddSBI(sbiID1)
	pbi.AddSBI(sbiID2)

	sbiIDs := pbi.SBIIDs()

	if len(sbiIDs) != 2 {
		t.Errorf("Expected 2 SBI IDs, got %d", len(sbiIDs))
	}

	// Verify returned slice is a copy (modification shouldn't affect PBI)
	sbiIDs[0], _ = model.NewTaskIDFromString("modified")
	if pbi.SBIIDs()[0].String() == "modified" {
		t.Error("Modifying returned slice should not affect PBI's internal state")
	}
}

func TestPBI_UpdateMetadata(t *testing.T) {
	oldMetadata := PBIMetadata{
		StoryPoints: 3,
		Priority:    0,
	}
	pbi, _ := NewPBI("Test PBI", "Description", nil, oldMetadata)

	newMetadata := PBIMetadata{
		StoryPoints:        8,
		Priority:           2,
		Labels:             []string{"urgent"},
		AssignedAgent:      "gemini-cli",
		AcceptanceCriteria: []string{"Must work offline"},
	}

	pbi.UpdateMetadata(newMetadata)

	updatedMetadata := pbi.Metadata()
	if updatedMetadata.StoryPoints != 8 {
		t.Errorf("Expected StoryPoints 8, got %d", updatedMetadata.StoryPoints)
	}
	if updatedMetadata.Priority != 2 {
		t.Errorf("Expected Priority 2, got %d", updatedMetadata.Priority)
	}
	if updatedMetadata.AssignedAgent != "gemini-cli" {
		t.Errorf("Expected AssignedAgent 'gemini-cli', got '%s'", updatedMetadata.AssignedAgent)
	}
	if len(updatedMetadata.AcceptanceCriteria) != 1 {
		t.Errorf("Expected 1 acceptance criterion, got %d", len(updatedMetadata.AcceptanceCriteria))
	}
}

func TestPBI_UpdateTitle(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Old Title", "Description", nil, metadata)

	newTitle := "New Title"
	err := pbi.UpdateTitle(newTitle)
	if err != nil {
		t.Fatalf("UpdateTitle failed: %v", err)
	}

	if pbi.Title() != newTitle {
		t.Errorf("Expected title '%s', got '%s'", newTitle, pbi.Title())
	}
}

func TestPBI_UpdateDescription(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Title", "Old Description", nil, metadata)

	newDescription := "New Description"
	pbi.UpdateDescription(newDescription)

	if pbi.Description() != newDescription {
		t.Errorf("Expected description '%s', got '%s'", newDescription, pbi.Description())
	}
}

func TestPBI_UpdateStatus(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	// Valid transition: Pending -> Picked
	err := pbi.UpdateStatus(model.StatusPicked)
	if err != nil {
		t.Errorf("UpdateStatus failed: %v", err)
	}

	if pbi.Status() != model.StatusPicked {
		t.Errorf("Expected status Picked, got %v", pbi.Status())
	}
}

func TestPBI_CanDelete(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	// PBI without SBIs can be deleted
	if !pbi.CanDelete() {
		t.Error("PBI without SBIs should be deletable")
	}

	// Add an SBI
	sbiID := model.NewTaskID()
	pbi.AddSBI(sbiID)

	// PBI with SBIs cannot be deleted
	if pbi.CanDelete() {
		t.Error("PBI with SBIs should not be deletable")
	}

	// Remove the SBI
	pbi.RemoveSBI(sbiID)

	// Now it can be deleted again
	if !pbi.CanDelete() {
		t.Error("PBI should be deletable after removing all SBIs")
	}
}

func TestPBI_IsCompleted(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	if pbi.IsCompleted() {
		t.Error("New PBI should not be completed")
	}

	// Transition to Done
	pbi.UpdateStatus(model.StatusPicked)
	pbi.UpdateStatus(model.StatusImplementing)
	pbi.UpdateStatus(model.StatusReviewing)
	pbi.UpdateStatus(model.StatusDone)

	if !pbi.IsCompleted() {
		t.Error("PBI with Done status should be completed")
	}
}

func TestPBI_IsFailed(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	if pbi.IsFailed() {
		t.Error("New PBI should not be failed")
	}

	// Transition to Failed
	pbi.UpdateStatus(model.StatusPicked)
	pbi.UpdateStatus(model.StatusImplementing)
	pbi.UpdateStatus(model.StatusFailed)

	if !pbi.IsFailed() {
		t.Error("PBI with Failed status should be failed")
	}
}

func TestReconstructPBI(t *testing.T) {
	id := model.NewTaskID()
	parentID := model.NewTaskID()
	sbiID1 := model.NewTaskID()
	sbiID2 := model.NewTaskID()
	createdAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)

	metadata := PBIMetadata{
		StoryPoints:        8,
		Priority:           2,
		Labels:             []string{"feature"},
		AssignedAgent:      "codex",
		AcceptanceCriteria: []string{"Criterion 1"},
	}

	pbi := ReconstructPBI(
		id,
		"Reconstructed Title",
		"Reconstructed Description",
		model.StatusImplementing,
		model.StepImplement,
		&parentID,
		[]model.TaskID{sbiID1, sbiID2},
		metadata,
		createdAt,
		updatedAt,
	)

	if !pbi.ID().Equals(id) {
		t.Errorf("Expected ID %v, got %v", id, pbi.ID())
	}

	if pbi.Title() != "Reconstructed Title" {
		t.Errorf("Expected title 'Reconstructed Title', got '%s'", pbi.Title())
	}

	if pbi.Status() != model.StatusImplementing {
		t.Errorf("Expected status Implementing, got %v", pbi.Status())
	}

	if pbi.CurrentStep() != model.StepImplement {
		t.Errorf("Expected step Implement, got %v", pbi.CurrentStep())
	}

	if !pbi.HasParentEPIC() {
		t.Error("Expected reconstructed PBI to have parent EPIC")
	}

	if pbi.SBICount() != 2 {
		t.Errorf("Expected 2 SBIs, got %d", pbi.SBICount())
	}

	if pbi.Metadata().StoryPoints != 8 {
		t.Errorf("Expected StoryPoints 8, got %d", pbi.Metadata().StoryPoints)
	}

	if !pbi.CreatedAt().Value().Equal(createdAt) {
		t.Error("CreatedAt does not match")
	}

	if !pbi.UpdatedAt().Value().Equal(updatedAt) {
		t.Error("UpdatedAt does not match")
	}
}

func TestPBI_Timestamps(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	createdAt := pbi.CreatedAt()
	if createdAt.Value().IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	updatedAt := pbi.UpdatedAt()
	if updatedAt.Value().IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	if updatedAt.Value().Before(createdAt.Value()) {
		t.Error("UpdatedAt should not be before CreatedAt")
	}
}

func TestPBI_MultipleSBIOperations(t *testing.T) {
	metadata := PBIMetadata{}
	pbi, _ := NewPBI("Test PBI", "Description", nil, metadata)

	// Add multiple SBIs
	sbiIDs := make([]model.TaskID, 5)
	for i := 0; i < 5; i++ {
		sbiIDs[i] = model.NewTaskID()
		err := pbi.AddSBI(sbiIDs[i])
		if err != nil {
			t.Fatalf("Failed to add SBI %d: %v", i, err)
		}
	}

	if pbi.SBICount() != 5 {
		t.Errorf("Expected 5 SBIs, got %d", pbi.SBICount())
	}

	// Remove some SBIs
	pbi.RemoveSBI(sbiIDs[1])
	pbi.RemoveSBI(sbiIDs[3])

	if pbi.SBICount() != 3 {
		t.Errorf("Expected 3 SBIs after removal, got %d", pbi.SBICount())
	}

	// Verify correct SBIs remain
	remainingSBIs := pbi.SBIIDs()
	expectedRemaining := []model.TaskID{sbiIDs[0], sbiIDs[2], sbiIDs[4]}

	for _, expected := range expectedRemaining {
		found := false
		for _, remaining := range remainingSBIs {
			if remaining.Equals(expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected SBI %v not found in remaining SBIs", expected)
		}
	}
}
