package epic

import (
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
)

func TestNewEPIC(t *testing.T) {
	metadata := EPICMetadata{
		EstimatedStoryPoints: 50,
		Priority:             1,
		Labels:               []string{"feature", "v2"},
		AssignedAgent:        "claude-code",
	}

	epic, err := NewEPIC("Test EPIC", "Test description", metadata)
	if err != nil {
		t.Fatalf("NewEPIC failed: %v", err)
	}

	if epic == nil {
		t.Fatal("Expected non-nil EPIC")
	}

	if epic.Title() != "Test EPIC" {
		t.Errorf("Expected title 'Test EPIC', got '%s'", epic.Title())
	}

	if epic.Description() != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", epic.Description())
	}

	if epic.Type() != model.TaskTypeEPIC {
		t.Errorf("Expected type TaskTypeEPIC, got %v", epic.Type())
	}

	if epic.Status() != model.StatusPending {
		t.Errorf("Expected status Pending, got %v", epic.Status())
	}

	if epic.HasPBIs() {
		t.Error("New EPIC should not have any PBIs")
	}

	// EPIC should never have a parent
	if epic.ParentTaskID() != nil {
		t.Error("EPIC should not have a parent")
	}
}

func TestEPIC_AddPBI(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	pbiID1 := model.NewTaskID()
	pbiID2 := model.NewTaskID()

	// Add first PBI
	err := epic.AddPBI(pbiID1)
	if err != nil {
		t.Fatalf("Failed to add first PBI: %v", err)
	}

	if !epic.HasPBIs() {
		t.Error("EPIC should have PBIs after adding")
	}

	if epic.PBICount() != 1 {
		t.Errorf("Expected 1 PBI, got %d", epic.PBICount())
	}

	// Add second PBI
	err = epic.AddPBI(pbiID2)
	if err != nil {
		t.Fatalf("Failed to add second PBI: %v", err)
	}

	if epic.PBICount() != 2 {
		t.Errorf("Expected 2 PBIs, got %d", epic.PBICount())
	}

	// Try to add duplicate PBI
	err = epic.AddPBI(pbiID1)
	if err == nil {
		t.Error("Expected error when adding duplicate PBI")
	}

	if epic.PBICount() != 2 {
		t.Error("PBI count should not change when adding duplicate")
	}
}

func TestEPIC_RemovePBI(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	pbiID1 := model.NewTaskID()
	pbiID2 := model.NewTaskID()
	pbiID3 := model.NewTaskID()

	// Add PBIs
	epic.AddPBI(pbiID1)
	epic.AddPBI(pbiID2)

	// Remove existing PBI
	err := epic.RemovePBI(pbiID1)
	if err != nil {
		t.Fatalf("Failed to remove PBI: %v", err)
	}

	if epic.PBICount() != 1 {
		t.Errorf("Expected 1 PBI after removal, got %d", epic.PBICount())
	}

	// Try to remove non-existent PBI
	err = epic.RemovePBI(pbiID3)
	if err == nil {
		t.Error("Expected error when removing non-existent PBI")
	}

	// Remove last PBI
	err = epic.RemovePBI(pbiID2)
	if err != nil {
		t.Fatalf("Failed to remove last PBI: %v", err)
	}

	if epic.HasPBIs() {
		t.Error("EPIC should not have PBIs after removing all")
	}
}

func TestEPIC_PBIIDs(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	pbiID1 := model.NewTaskID()
	pbiID2 := model.NewTaskID()

	epic.AddPBI(pbiID1)
	epic.AddPBI(pbiID2)

	pbiIDs := epic.PBIIDs()

	if len(pbiIDs) != 2 {
		t.Errorf("Expected 2 PBI IDs, got %d", len(pbiIDs))
	}

	// Verify returned slice is a copy (modification shouldn't affect EPIC)
	pbiIDs[0], _ = model.NewTaskIDFromString("modified")
	if epic.PBIIDs()[0].String() == "modified" {
		t.Error("Modifying returned slice should not affect EPIC's internal state")
	}
}

func TestEPIC_UpdateMetadata(t *testing.T) {
	oldMetadata := EPICMetadata{
		EstimatedStoryPoints: 30,
		Priority:             0,
	}
	epic, _ := NewEPIC("Test EPIC", "Description", oldMetadata)

	newMetadata := EPICMetadata{
		EstimatedStoryPoints: 100,
		Priority:             2,
		Labels:               []string{"urgent", "strategic"},
		AssignedAgent:        "gemini-cli",
	}

	epic.UpdateMetadata(newMetadata)

	updatedMetadata := epic.Metadata()
	if updatedMetadata.EstimatedStoryPoints != 100 {
		t.Errorf("Expected EstimatedStoryPoints 100, got %d", updatedMetadata.EstimatedStoryPoints)
	}
	if updatedMetadata.Priority != 2 {
		t.Errorf("Expected Priority 2, got %d", updatedMetadata.Priority)
	}
	if updatedMetadata.AssignedAgent != "gemini-cli" {
		t.Errorf("Expected AssignedAgent 'gemini-cli', got '%s'", updatedMetadata.AssignedAgent)
	}
	if len(updatedMetadata.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(updatedMetadata.Labels))
	}
}

func TestEPIC_UpdateTitle(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Old Title", "Description", metadata)

	newTitle := "New Title"
	err := epic.UpdateTitle(newTitle)
	if err != nil {
		t.Fatalf("UpdateTitle failed: %v", err)
	}

	if epic.Title() != newTitle {
		t.Errorf("Expected title '%s', got '%s'", newTitle, epic.Title())
	}
}

func TestEPIC_UpdateDescription(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Title", "Old Description", metadata)

	newDescription := "New Description"
	epic.UpdateDescription(newDescription)

	if epic.Description() != newDescription {
		t.Errorf("Expected description '%s', got '%s'", newDescription, epic.Description())
	}
}

func TestEPIC_UpdateStatus(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	// Valid transition: Pending -> Picked
	err := epic.UpdateStatus(model.StatusPicked)
	if err != nil {
		t.Errorf("UpdateStatus failed: %v", err)
	}

	if epic.Status() != model.StatusPicked {
		t.Errorf("Expected status Picked, got %v", epic.Status())
	}
}

func TestEPIC_CanDelete(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	// EPIC without PBIs can be deleted
	if !epic.CanDelete() {
		t.Error("EPIC without PBIs should be deletable")
	}

	// Add a PBI
	pbiID := model.NewTaskID()
	epic.AddPBI(pbiID)

	// EPIC with PBIs cannot be deleted
	if epic.CanDelete() {
		t.Error("EPIC with PBIs should not be deletable")
	}

	// Remove the PBI
	epic.RemovePBI(pbiID)

	// Now it can be deleted again
	if !epic.CanDelete() {
		t.Error("EPIC should be deletable after removing all PBIs")
	}
}

func TestEPIC_IsCompleted(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	if epic.IsCompleted() {
		t.Error("New EPIC should not be completed")
	}

	// Transition to Done
	epic.UpdateStatus(model.StatusPicked)
	epic.UpdateStatus(model.StatusImplementing)
	epic.UpdateStatus(model.StatusReviewing)
	epic.UpdateStatus(model.StatusDone)

	if !epic.IsCompleted() {
		t.Error("EPIC with Done status should be completed")
	}
}

func TestEPIC_IsFailed(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	if epic.IsFailed() {
		t.Error("New EPIC should not be failed")
	}

	// Transition to Failed
	epic.UpdateStatus(model.StatusPicked)
	epic.UpdateStatus(model.StatusImplementing)
	epic.UpdateStatus(model.StatusFailed)

	if !epic.IsFailed() {
		t.Error("EPIC with Failed status should be failed")
	}
}

func TestReconstructEPIC(t *testing.T) {
	id := model.NewTaskID()
	pbiID1 := model.NewTaskID()
	pbiID2 := model.NewTaskID()
	createdAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)

	metadata := EPICMetadata{
		EstimatedStoryPoints: 80,
		Priority:             2,
		Labels:               []string{"strategic"},
		AssignedAgent:        "codex",
	}

	epic := ReconstructEPIC(
		id,
		"Reconstructed Title",
		"Reconstructed Description",
		model.StatusImplementing,
		model.StepImplement,
		[]model.TaskID{pbiID1, pbiID2},
		metadata,
		createdAt,
		updatedAt,
	)

	if !epic.ID().Equals(id) {
		t.Errorf("Expected ID %v, got %v", id, epic.ID())
	}

	if epic.Title() != "Reconstructed Title" {
		t.Errorf("Expected title 'Reconstructed Title', got '%s'", epic.Title())
	}

	if epic.Status() != model.StatusImplementing {
		t.Errorf("Expected status Implementing, got %v", epic.Status())
	}

	if epic.CurrentStep() != model.StepImplement {
		t.Errorf("Expected step Implement, got %v", epic.CurrentStep())
	}

	// EPIC should not have a parent
	if epic.ParentTaskID() != nil {
		t.Error("EPIC should not have a parent")
	}

	if epic.PBICount() != 2 {
		t.Errorf("Expected 2 PBIs, got %d", epic.PBICount())
	}

	if epic.Metadata().EstimatedStoryPoints != 80 {
		t.Errorf("Expected EstimatedStoryPoints 80, got %d", epic.Metadata().EstimatedStoryPoints)
	}

	if !epic.CreatedAt().Value().Equal(createdAt) {
		t.Error("CreatedAt does not match")
	}

	if !epic.UpdatedAt().Value().Equal(updatedAt) {
		t.Error("UpdatedAt does not match")
	}
}

func TestEPIC_Timestamps(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	createdAt := epic.CreatedAt()
	if createdAt.Value().IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	updatedAt := epic.UpdatedAt()
	if updatedAt.Value().IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	if updatedAt.Value().Before(createdAt.Value()) {
		t.Error("UpdatedAt should not be before CreatedAt")
	}
}

func TestEPIC_MultiplePBIOperations(t *testing.T) {
	metadata := EPICMetadata{}
	epic, _ := NewEPIC("Test EPIC", "Description", metadata)

	// Add multiple PBIs
	pbiIDs := make([]model.TaskID, 5)
	for i := 0; i < 5; i++ {
		pbiIDs[i] = model.NewTaskID()
		err := epic.AddPBI(pbiIDs[i])
		if err != nil {
			t.Fatalf("Failed to add PBI %d: %v", i, err)
		}
	}

	if epic.PBICount() != 5 {
		t.Errorf("Expected 5 PBIs, got %d", epic.PBICount())
	}

	// Remove some PBIs
	epic.RemovePBI(pbiIDs[1])
	epic.RemovePBI(pbiIDs[3])

	if epic.PBICount() != 3 {
		t.Errorf("Expected 3 PBIs after removal, got %d", epic.PBICount())
	}

	// Verify correct PBIs remain
	remainingPBIs := epic.PBIIDs()
	expectedRemaining := []model.TaskID{pbiIDs[0], pbiIDs[2], pbiIDs[4]}

	for _, expected := range expectedRemaining {
		found := false
		for _, remaining := range remainingPBIs {
			if remaining.Equals(expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected PBI %v not found in remaining PBIs", expected)
		}
	}
}

func TestEPIC_NoParent(t *testing.T) {
	// Verify that EPIC never has a parent, both on creation and reconstruction
	metadata := EPICMetadata{}

	// Test NewEPIC
	epic1, _ := NewEPIC("Test EPIC", "Description", metadata)
	if epic1.ParentTaskID() != nil {
		t.Error("New EPIC should not have a parent")
	}

	// Test ReconstructEPIC
	id := model.NewTaskID()
	createdAt := time.Now()
	updatedAt := time.Now()

	epic2 := ReconstructEPIC(
		id,
		"Reconstructed",
		"Description",
		model.StatusPending,
		model.StepPick,
		[]model.TaskID{},
		metadata,
		createdAt,
		updatedAt,
	)

	if epic2.ParentTaskID() != nil {
		t.Error("Reconstructed EPIC should not have a parent")
	}
}
