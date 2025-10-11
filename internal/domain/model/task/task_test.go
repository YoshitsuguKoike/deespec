package task

import (
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
)

func TestNewBaseTask(t *testing.T) {
	tests := []struct {
		name        string
		taskType    model.TaskType
		title       string
		description string
		parentID    *model.TaskID
		wantErr     bool
	}{
		{
			name:        "Valid SBI task",
			taskType:    model.TaskTypeSBI,
			title:       "Test SBI",
			description: "Test description",
			parentID:    nil,
			wantErr:     false,
		},
		{
			name:        "Valid PBI task with parent",
			taskType:    model.TaskTypePBI,
			title:       "Test PBI",
			description: "Test description",
			parentID:    func() *model.TaskID { id := model.NewTaskID(); return &id }(),
			wantErr:     false,
		},
		{
			name:        "Invalid task type",
			taskType:    model.TaskType("INVALID"),
			title:       "Test",
			description: "Test",
			parentID:    nil,
			wantErr:     true,
		},
		{
			name:        "Empty title",
			taskType:    model.TaskTypeSBI,
			title:       "",
			description: "Test",
			parentID:    nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := NewBaseTask(tt.taskType, tt.title, tt.description, tt.parentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewBaseTask() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify task was created correctly
			if task.Type() != tt.taskType {
				t.Errorf("Type() = %v, want %v", task.Type(), tt.taskType)
			}

			if task.Title() != tt.title {
				t.Errorf("Title() = %v, want %v", task.Title(), tt.title)
			}

			if task.Description() != tt.description {
				t.Errorf("Description() = %v, want %v", task.Description(), tt.description)
			}

			if task.Status() != model.StatusPending {
				t.Errorf("Initial Status() = %v, want %v", task.Status(), model.StatusPending)
			}

			if task.CurrentStep() != model.StepPick {
				t.Errorf("Initial CurrentStep() = %v, want %v", task.CurrentStep(), model.StepPick)
			}

			// Verify timestamps
			if task.CreatedAt().Value().IsZero() {
				t.Error("CreatedAt should not be zero")
			}

			if task.UpdatedAt().Value().IsZero() {
				t.Error("UpdatedAt should not be zero")
			}

			// Verify parent ID
			if tt.parentID != nil {
				if task.ParentTaskID() == nil {
					t.Error("ParentTaskID should not be nil when parentID provided")
				} else if !task.ParentTaskID().Equals(*tt.parentID) {
					t.Error("ParentTaskID does not match provided parentID")
				}
			} else {
				if task.ParentTaskID() != nil {
					t.Error("ParentTaskID should be nil when no parentID provided")
				}
			}
		})
	}
}

func TestReconstructBaseTask(t *testing.T) {
	id := model.NewTaskID()
	parentID := model.NewTaskID()
	createdAt := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC)

	task := ReconstructBaseTask(
		id,
		model.TaskTypeSBI,
		"Reconstructed Task",
		"Reconstructed Description",
		model.StatusImplementing,
		model.StepImplement,
		&parentID,
		createdAt,
		updatedAt,
	)

	if !task.ID().Equals(id) {
		t.Error("ID does not match")
	}

	if task.Type() != model.TaskTypeSBI {
		t.Errorf("Type = %v, want %v", task.Type(), model.TaskTypeSBI)
	}

	if task.Title() != "Reconstructed Task" {
		t.Errorf("Title = %v, want 'Reconstructed Task'", task.Title())
	}

	if task.Description() != "Reconstructed Description" {
		t.Errorf("Description = %v, want 'Reconstructed Description'", task.Description())
	}

	if task.Status() != model.StatusImplementing {
		t.Errorf("Status = %v, want %v", task.Status(), model.StatusImplementing)
	}

	if task.CurrentStep() != model.StepImplement {
		t.Errorf("CurrentStep = %v, want %v", task.CurrentStep(), model.StepImplement)
	}

	if !task.CreatedAt().Value().Equal(createdAt) {
		t.Error("CreatedAt does not match")
	}

	if !task.UpdatedAt().Value().Equal(updatedAt) {
		t.Error("UpdatedAt does not match")
	}

	if task.ParentTaskID() == nil {
		t.Error("ParentTaskID should not be nil")
	} else if !task.ParentTaskID().Equals(parentID) {
		t.Error("ParentTaskID does not match")
	}
}

func TestBaseTask_UpdateStatus(t *testing.T) {
	task, _ := NewBaseTask(model.TaskTypeSBI, "Test", "Description", nil)
	originalUpdatedAt := task.UpdatedAt()

	// Allow a small delay to ensure UpdatedAt changes
	time.Sleep(1 * time.Millisecond)

	// Test valid transition: Pending -> Picked
	err := task.UpdateStatus(model.StatusPicked)
	if err != nil {
		t.Errorf("UpdateStatus(Picked) failed: %v", err)
	}

	if task.Status() != model.StatusPicked {
		t.Errorf("Status = %v, want %v", task.Status(), model.StatusPicked)
	}

	// Step should automatically update based on status
	if task.CurrentStep() != model.StepPick {
		t.Errorf("CurrentStep = %v, want %v", task.CurrentStep(), model.StepPick)
	}

	// UpdatedAt should have changed
	if !task.UpdatedAt().After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after status change")
	}

	// Test invalid transition
	err = task.UpdateStatus(model.StatusDone)
	if err == nil {
		t.Error("Expected error for invalid transition Picked -> Done")
	}

	// Status should not change on invalid transition
	if task.Status() != model.StatusPicked {
		t.Error("Status should not change on invalid transition")
	}

	// Test invalid status
	err = task.UpdateStatus(model.Status("INVALID"))
	if err == nil {
		t.Error("Expected error for invalid status")
	}
}

func TestBaseTask_UpdateStep(t *testing.T) {
	task, _ := NewBaseTask(model.TaskTypeSBI, "Test", "Description", nil)
	originalUpdatedAt := task.UpdatedAt()

	time.Sleep(1 * time.Millisecond)

	// Update to valid step
	err := task.UpdateStep(model.StepImplement)
	if err != nil {
		t.Errorf("UpdateStep(Implement) failed: %v", err)
	}

	if task.CurrentStep() != model.StepImplement {
		t.Errorf("CurrentStep = %v, want %v", task.CurrentStep(), model.StepImplement)
	}

	// UpdatedAt should have changed
	if !task.UpdatedAt().After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after step change")
	}

	// Test invalid step
	err = task.UpdateStep(model.Step("INVALID"))
	if err == nil {
		t.Error("Expected error for invalid step")
	}
}

func TestBaseTask_UpdateTitle(t *testing.T) {
	task, _ := NewBaseTask(model.TaskTypeSBI, "Original Title", "Description", nil)
	originalUpdatedAt := task.UpdatedAt()

	time.Sleep(1 * time.Millisecond)

	// Update to new title
	newTitle := "Updated Title"
	err := task.UpdateTitle(newTitle)
	if err != nil {
		t.Errorf("UpdateTitle() failed: %v", err)
	}

	if task.Title() != newTitle {
		t.Errorf("Title = %v, want %v", task.Title(), newTitle)
	}

	// UpdatedAt should have changed
	if !task.UpdatedAt().After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after title change")
	}

	// Test empty title
	err = task.UpdateTitle("")
	if err == nil {
		t.Error("Expected error for empty title")
	}

	// Title should not change on error
	if task.Title() != newTitle {
		t.Error("Title should not change when update fails")
	}
}

func TestBaseTask_UpdateDescription(t *testing.T) {
	task, _ := NewBaseTask(model.TaskTypeSBI, "Title", "Original Description", nil)
	originalUpdatedAt := task.UpdatedAt()

	time.Sleep(1 * time.Millisecond)

	// Update to new description
	newDescription := "Updated Description"
	task.UpdateDescription(newDescription)

	if task.Description() != newDescription {
		t.Errorf("Description = %v, want %v", task.Description(), newDescription)
	}

	// UpdatedAt should have changed
	if !task.UpdatedAt().After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after description change")
	}

	// Empty description should be allowed (no error returned)
	task.UpdateDescription("")
	if task.Description() != "" {
		t.Error("Description should be empty")
	}
}

func TestBaseTask_DeriveStepFromStatus(t *testing.T) {
	task, _ := NewBaseTask(model.TaskTypeSBI, "Test", "Description", nil)

	tests := []struct {
		status       model.Status
		expectedStep model.Step
	}{
		{model.StatusPending, model.StepPick},
		{model.StatusPicked, model.StepPick},
		{model.StatusImplementing, model.StepImplement},
		{model.StatusReviewing, model.StepReview},
		{model.StatusDone, model.StepDone},
		{model.StatusFailed, model.StepDone},
	}

	for _, tt := range tests {
		t.Run(tt.status.String(), func(t *testing.T) {
			// We need to test through UpdateStatus since deriveStepFromStatus is private
			// First, get to a state where we can transition to the target status
			task, _ = NewBaseTask(model.TaskTypeSBI, "Test", "Description", nil)

			// Create a valid path to the target status
			switch tt.status {
			case model.StatusPending:
				// Already pending, do nothing
			case model.StatusPicked:
				task.UpdateStatus(model.StatusPicked)
			case model.StatusImplementing:
				task.UpdateStatus(model.StatusPicked)
				task.UpdateStatus(model.StatusImplementing)
			case model.StatusReviewing:
				task.UpdateStatus(model.StatusPicked)
				task.UpdateStatus(model.StatusImplementing)
				task.UpdateStatus(model.StatusReviewing)
			case model.StatusDone:
				task.UpdateStatus(model.StatusPicked)
				task.UpdateStatus(model.StatusImplementing)
				task.UpdateStatus(model.StatusReviewing)
				task.UpdateStatus(model.StatusDone)
			case model.StatusFailed:
				task.UpdateStatus(model.StatusPicked)
				task.UpdateStatus(model.StatusImplementing)
				task.UpdateStatus(model.StatusFailed)
			}

			if task.CurrentStep() != tt.expectedStep {
				t.Errorf("For status %v, expected step %v, got %v",
					tt.status, tt.expectedStep, task.CurrentStep())
			}
		})
	}
}

func TestBaseTask_CompleteStatusTransitionFlow(t *testing.T) {
	// Test complete happy path: Pending -> Picked -> Implementing -> Reviewing -> Done
	task, err := NewBaseTask(model.TaskTypeSBI, "Test Task", "Test Description", nil)
	if err != nil {
		t.Fatalf("Failed to create task: %v", err)
	}

	// Verify initial state
	if task.Status() != model.StatusPending {
		t.Errorf("Initial status should be Pending, got %v", task.Status())
	}

	// Transition: Pending -> Picked
	if err := task.UpdateStatus(model.StatusPicked); err != nil {
		t.Errorf("Failed to transition to Picked: %v", err)
	}
	if task.Status() != model.StatusPicked || task.CurrentStep() != model.StepPick {
		t.Error("Status or step incorrect after Picked transition")
	}

	// Transition: Picked -> Implementing
	if err := task.UpdateStatus(model.StatusImplementing); err != nil {
		t.Errorf("Failed to transition to Implementing: %v", err)
	}
	if task.Status() != model.StatusImplementing || task.CurrentStep() != model.StepImplement {
		t.Error("Status or step incorrect after Implementing transition")
	}

	// Transition: Implementing -> Reviewing
	if err := task.UpdateStatus(model.StatusReviewing); err != nil {
		t.Errorf("Failed to transition to Reviewing: %v", err)
	}
	if task.Status() != model.StatusReviewing || task.CurrentStep() != model.StepReview {
		t.Error("Status or step incorrect after Reviewing transition")
	}

	// Transition: Reviewing -> Done
	if err := task.UpdateStatus(model.StatusDone); err != nil {
		t.Errorf("Failed to transition to Done: %v", err)
	}
	if task.Status() != model.StatusDone || task.CurrentStep() != model.StepDone {
		t.Error("Status or step incorrect after Done transition")
	}

	// Done should not transition anywhere
	if err := task.UpdateStatus(model.StatusPending); err == nil {
		t.Error("Done should not transition to any other status")
	}
}

func TestBaseTask_FailureFlow(t *testing.T) {
	// Test failure path: Pending -> Picked -> Implementing -> Failed -> Pending
	task, _ := NewBaseTask(model.TaskTypeSBI, "Test Task", "Test Description", nil)

	// Get to Implementing
	task.UpdateStatus(model.StatusPicked)
	task.UpdateStatus(model.StatusImplementing)

	// Transition to Failed
	if err := task.UpdateStatus(model.StatusFailed); err != nil {
		t.Errorf("Failed to transition to Failed: %v", err)
	}
	if task.Status() != model.StatusFailed || task.CurrentStep() != model.StepDone {
		t.Error("Status or step incorrect after Failed transition")
	}

	// Failed can go back to Pending
	if err := task.UpdateStatus(model.StatusPending); err != nil {
		t.Errorf("Failed should be able to transition to Pending: %v", err)
	}
	if task.Status() != model.StatusPending {
		t.Error("Status should be Pending after retry")
	}
}
