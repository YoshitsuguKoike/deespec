package sbi

import (
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
)

func TestNewSBI(t *testing.T) {
	metadata := SBIMetadata{
		EstimatedHours: 2.5,
		Priority:       1,
		Sequence:       1,
		RegisteredAt:   time.Now(),
		Labels:         []string{"bug", "urgent"},
		AssignedAgent:  "claude-code",
		FilePaths:      []string{"main.go", "test.go"},
	}

	sbi, err := NewSBI("Test SBI", "Test description", nil, metadata)
	if err != nil {
		t.Fatalf("NewSBI failed: %v", err)
	}

	if sbi == nil {
		t.Fatal("Expected non-nil SBI")
	}

	if sbi.Title() != "Test SBI" {
		t.Errorf("Expected title 'Test SBI', got '%s'", sbi.Title())
	}

	if sbi.Description() != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", sbi.Description())
	}

	if sbi.Type() != model.TaskTypeSBI {
		t.Errorf("Expected type TaskTypeSBI, got %v", sbi.Type())
	}

	if sbi.Status() != model.StatusPending {
		t.Errorf("Expected status Pending, got %v", sbi.Status())
	}
}

func TestNewSBI_WithParentPBI(t *testing.T) {
	parentID := model.NewTaskID()
	metadata := SBIMetadata{
		EstimatedHours: 1.0,
		Priority:       0,
		Sequence:       1,
		RegisteredAt:   time.Now(),
	}

	sbi, err := NewSBI("Child SBI", "Child description", &parentID, metadata)
	if err != nil {
		t.Fatalf("NewSBI with parent failed: %v", err)
	}

	if !sbi.HasParentPBI() {
		t.Error("Expected SBI to have parent PBI")
	}

	if sbi.ParentTaskID() == nil {
		t.Error("Expected non-nil parent task ID")
	}

	if *sbi.ParentTaskID() != parentID {
		t.Errorf("Expected parent ID %v, got %v", parentID, *sbi.ParentTaskID())
	}
}

func TestSBI_UpdateStatus(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	// Test valid transition: Pending -> Picked
	err := sbi.UpdateStatus(model.StatusPicked)
	if err != nil {
		t.Errorf("UpdateStatus to Picked failed: %v", err)
	}
	if sbi.Status() != model.StatusPicked {
		t.Errorf("Expected status Picked, got %v", sbi.Status())
	}

	// Test valid transition: Picked -> Implementing
	err = sbi.UpdateStatus(model.StatusImplementing)
	if err != nil {
		t.Errorf("UpdateStatus to Implementing failed: %v", err)
	}
	if sbi.Status() != model.StatusImplementing {
		t.Errorf("Expected status Implementing, got %v", sbi.Status())
	}

	// Test valid transition: Implementing -> Reviewing
	err = sbi.UpdateStatus(model.StatusReviewing)
	if err != nil {
		t.Errorf("UpdateStatus to Reviewing failed: %v", err)
	}
	if sbi.Status() != model.StatusReviewing {
		t.Errorf("Expected status Reviewing, got %v", sbi.Status())
	}

	// Test valid transition: Reviewing -> Done
	err = sbi.UpdateStatus(model.StatusDone)
	if err != nil {
		t.Errorf("UpdateStatus to Done failed: %v", err)
	}
	if sbi.Status() != model.StatusDone {
		t.Errorf("Expected status Done, got %v", sbi.Status())
	}

	// Test invalid transition: Done cannot transition anywhere
	err = sbi.UpdateStatus(model.StatusPending)
	if err == nil {
		t.Error("Expected error for invalid transition from Done to Pending")
	}
}

func TestSBI_IncrementTurn(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	initialTurn := sbi.ExecutionState().CurrentTurn.Value()

	sbi.IncrementTurn()

	newTurn := sbi.ExecutionState().CurrentTurn.Value()
	newAttempt := sbi.ExecutionState().CurrentAttempt.Value()

	if newTurn != initialTurn+1 {
		t.Errorf("Expected turn %d, got %d", initialTurn+1, newTurn)
	}

	// Attempt should be reset to initial value
	if newAttempt != 1 {
		t.Errorf("Expected attempt to be reset to 1, got %d", newAttempt)
	}
}

func TestSBI_IncrementAttempt(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	initialAttempt := sbi.ExecutionState().CurrentAttempt.Value()

	sbi.IncrementAttempt()

	newAttempt := sbi.ExecutionState().CurrentAttempt.Value()

	if newAttempt != initialAttempt+1 {
		t.Errorf("Expected attempt %d, got %d", initialAttempt+1, newAttempt)
	}
}

func TestSBI_RecordAndClearError(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	errorMsg := "Test error message"
	sbi.RecordError(errorMsg)

	if sbi.ExecutionState().LastError != errorMsg {
		t.Errorf("Expected error '%s', got '%s'", errorMsg, sbi.ExecutionState().LastError)
	}

	sbi.ClearError()

	if sbi.ExecutionState().LastError != "" {
		t.Errorf("Expected empty error after clear, got '%s'", sbi.ExecutionState().LastError)
	}
}

func TestSBI_AddArtifact(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	artifacts := []string{"artifact1.md", "artifact2.md", "artifact3.md"}

	for _, artifact := range artifacts {
		sbi.AddArtifact(artifact)
	}

	executionState := sbi.ExecutionState()
	if len(executionState.ArtifactPaths) != len(artifacts) {
		t.Errorf("Expected %d artifacts, got %d", len(artifacts), len(executionState.ArtifactPaths))
	}

	for i, artifact := range artifacts {
		if executionState.ArtifactPaths[i] != artifact {
			t.Errorf("Artifact %d: expected '%s', got '%s'", i, artifact, executionState.ArtifactPaths[i])
		}
	}
}

func TestSBI_HasExceededMaxTurns(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	sbi.SetMaxTurns(3)

	// Initially should not exceed
	if sbi.HasExceededMaxTurns() {
		t.Error("Should not exceed max turns initially")
	}

	// Increment turns past the limit
	for i := 0; i < 4; i++ {
		sbi.IncrementTurn()
	}

	if !sbi.HasExceededMaxTurns() {
		t.Error("Should exceed max turns after incrementing past limit")
	}
}

func TestSBI_HasExceededMaxAttempts(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	sbi.SetMaxAttempts(2)

	// Initially should not exceed
	if sbi.HasExceededMaxAttempts() {
		t.Error("Should not exceed max attempts initially")
	}

	// Increment attempts past the limit
	for i := 0; i < 3; i++ {
		sbi.IncrementAttempt()
	}

	if !sbi.HasExceededMaxAttempts() {
		t.Error("Should exceed max attempts after incrementing past limit")
	}
}

func TestSBI_UpdateTitle(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Old Title", "Description", nil, metadata)

	newTitle := "New Title"
	err := sbi.UpdateTitle(newTitle)
	if err != nil {
		t.Fatalf("UpdateTitle failed: %v", err)
	}

	if sbi.Title() != newTitle {
		t.Errorf("Expected title '%s', got '%s'", newTitle, sbi.Title())
	}
}

func TestSBI_UpdateDescription(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Title", "Old Description", nil, metadata)

	newDescription := "New Description"
	sbi.UpdateDescription(newDescription)

	if sbi.Description() != newDescription {
		t.Errorf("Expected description '%s', got '%s'", newDescription, sbi.Description())
	}
}

func TestSBI_UpdateMetadata(t *testing.T) {
	oldMetadata := SBIMetadata{
		EstimatedHours: 1.0,
		Priority:       0,
	}
	sbi, _ := NewSBI("Title", "Description", nil, oldMetadata)

	newMetadata := SBIMetadata{
		EstimatedHours: 5.0,
		Priority:       2,
		Labels:         []string{"critical"},
		AssignedAgent:  "gemini-cli",
	}

	sbi.UpdateMetadata(newMetadata)

	updatedMetadata := sbi.Metadata()
	if updatedMetadata.EstimatedHours != 5.0 {
		t.Errorf("Expected EstimatedHours 5.0, got %f", updatedMetadata.EstimatedHours)
	}
	if updatedMetadata.Priority != 2 {
		t.Errorf("Expected Priority 2, got %d", updatedMetadata.Priority)
	}
	if updatedMetadata.AssignedAgent != "gemini-cli" {
		t.Errorf("Expected AssignedAgent 'gemini-cli', got '%s'", updatedMetadata.AssignedAgent)
	}
}

func TestSBI_CanDelete(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	// Can delete when pending
	if !sbi.CanDelete() {
		t.Error("Should be able to delete pending SBI")
	}

	// Cannot delete when implementing
	// Transition: Pending -> Picked -> Implementing
	sbi.UpdateStatus(model.StatusPicked)
	sbi.UpdateStatus(model.StatusImplementing)
	if sbi.CanDelete() {
		t.Error("Should not be able to delete implementing SBI")
	}

	// Can delete when done
	// Transition: Implementing -> Reviewing -> Done
	sbi.UpdateStatus(model.StatusReviewing)
	sbi.UpdateStatus(model.StatusDone)
	if !sbi.CanDelete() {
		t.Error("Should be able to delete done SBI")
	}
}

func TestSBI_IsCompleted(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	if sbi.IsCompleted() {
		t.Error("New SBI should not be completed")
	}

	// Transition: Pending -> Picked -> Implementing -> Reviewing -> Done
	sbi.UpdateStatus(model.StatusPicked)
	sbi.UpdateStatus(model.StatusImplementing)
	sbi.UpdateStatus(model.StatusReviewing)
	sbi.UpdateStatus(model.StatusDone)
	if !sbi.IsCompleted() {
		t.Error("SBI with Done status should be completed")
	}
}

func TestSBI_IsFailed(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	if sbi.IsFailed() {
		t.Error("New SBI should not be failed")
	}

	// Transition: Pending -> Picked -> Implementing -> Failed
	sbi.UpdateStatus(model.StatusPicked)
	sbi.UpdateStatus(model.StatusImplementing)
	sbi.UpdateStatus(model.StatusFailed)
	if !sbi.IsFailed() {
		t.Error("SBI with Failed status should be failed")
	}
}

func TestSBI_SetAndGetSequence(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	sequence := 42
	sbi.SetSequence(sequence)

	if sbi.Sequence() != sequence {
		t.Errorf("Expected sequence %d, got %d", sequence, sbi.Sequence())
	}
}

func TestSBI_SetAndGetRegisteredAt(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	registeredAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	sbi.SetRegisteredAt(registeredAt)

	if !sbi.RegisteredAt().Equal(registeredAt) {
		t.Errorf("Expected RegisteredAt %v, got %v", registeredAt, sbi.RegisteredAt())
	}
}

func TestSBI_SetAndGetPriority(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	priority := 2
	sbi.SetPriority(priority)

	if sbi.Priority() != priority {
		t.Errorf("Expected priority %d, got %d", priority, sbi.Priority())
	}
}

func TestSBI_SetAndGetMaxTurns(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	maxTurns := 20
	sbi.SetMaxTurns(maxTurns)

	if sbi.ExecutionState().MaxTurns != maxTurns {
		t.Errorf("Expected MaxTurns %d, got %d", maxTurns, sbi.ExecutionState().MaxTurns)
	}
}

func TestSBI_SetAndGetMaxAttempts(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	maxAttempts := 5
	sbi.SetMaxAttempts(maxAttempts)

	if sbi.ExecutionState().MaxAttempts != maxAttempts {
		t.Errorf("Expected MaxAttempts %d, got %d", maxAttempts, sbi.ExecutionState().MaxAttempts)
	}
}

func TestReconstructSBI(t *testing.T) {
	id := model.NewTaskID()
	parentID := model.NewTaskID()
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now()

	metadata := SBIMetadata{
		EstimatedHours: 3.5,
		Priority:       1,
		Sequence:       10,
		RegisteredAt:   createdAt,
		Labels:         []string{"feature"},
		AssignedAgent:  "codex",
	}

	execution := &ExecutionState{
		CurrentTurn:    model.NewTurn(),
		CurrentAttempt: model.NewAttempt(),
		MaxTurns:       15,
		MaxAttempts:    5,
		LastError:      "previous error",
		ArtifactPaths:  []string{"artifact1.md"},
	}

	sbi := ReconstructSBI(
		id,
		"Reconstructed Title",
		"Reconstructed Description",
		model.StatusImplementing,
		model.StepImplement,
		&parentID,
		metadata,
		execution,
		createdAt,
		updatedAt,
	)

	if sbi.ID() != id {
		t.Errorf("Expected ID %v, got %v", id, sbi.ID())
	}

	if sbi.Title() != "Reconstructed Title" {
		t.Errorf("Expected title 'Reconstructed Title', got '%s'", sbi.Title())
	}

	if sbi.Status() != model.StatusImplementing {
		t.Errorf("Expected status Implementing, got %v", sbi.Status())
	}

	if sbi.CurrentStep() != model.StepImplement {
		t.Errorf("Expected step Implement, got %v", sbi.CurrentStep())
	}

	if !sbi.HasParentPBI() {
		t.Error("Expected reconstructed SBI to have parent PBI")
	}

	if sbi.Metadata().EstimatedHours != 3.5 {
		t.Errorf("Expected EstimatedHours 3.5, got %f", sbi.Metadata().EstimatedHours)
	}

	if sbi.ExecutionState().MaxTurns != 15 {
		t.Errorf("Expected MaxTurns 15, got %d", sbi.ExecutionState().MaxTurns)
	}

	if sbi.ExecutionState().LastError != "previous error" {
		t.Errorf("Expected LastError 'previous error', got '%s'", sbi.ExecutionState().LastError)
	}
}

func TestSBI_UpdateStep(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	err := sbi.UpdateStep(model.StepReview)
	if err != nil {
		t.Fatalf("UpdateStep failed: %v", err)
	}

	if sbi.CurrentStep() != model.StepReview {
		t.Errorf("Expected step Review, got %v", sbi.CurrentStep())
	}
}

func TestSBI_Timestamps(t *testing.T) {
	metadata := SBIMetadata{}
	sbi, _ := NewSBI("Test", "Description", nil, metadata)

	createdAt := sbi.CreatedAt()
	if createdAt.Value().IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	updatedAt := sbi.UpdatedAt()
	if updatedAt.Value().IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	// UpdatedAt should be >= CreatedAt
	if updatedAt.Value().Before(createdAt.Value()) {
		t.Error("UpdatedAt should not be before CreatedAt")
	}
}
