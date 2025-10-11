package execution

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// MockSBIExecutionRepository is a mock implementation for testing
type MockSBIExecutionRepository struct {
	mu         sync.RWMutex
	executions map[ExecutionID]*SBIExecution
	saveErr    error
	findErr    error
}

// NewMockSBIExecutionRepository creates a new mock repository
func NewMockSBIExecutionRepository() *MockSBIExecutionRepository {
	return &MockSBIExecutionRepository{
		executions: make(map[ExecutionID]*SBIExecution),
	}
}

// Save persists an SBI execution
func (m *MockSBIExecutionRepository) Save(execution *SBIExecution) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate saving a copy to avoid external modifications
	saved := *execution
	m.executions[execution.ID] = &saved
	return nil
}

// FindByID retrieves an execution by its ID
func (m *MockSBIExecutionRepository) FindByID(id ExecutionID) (*SBIExecution, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec, exists := m.executions[id]
	if !exists {
		return nil, fmt.Errorf("execution not found: %s", id)
	}

	// Return a copy to simulate database behavior
	result := *exec
	return &result, nil
}

// FindBySBIID retrieves the latest execution for an SBI
func (m *MockSBIExecutionRepository) FindBySBIID(sbiID string) (*SBIExecution, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *SBIExecution
	for _, exec := range m.executions {
		if exec.SBIID == sbiID {
			if latest == nil || exec.StartedAt.After(latest.StartedAt) {
				latest = exec
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("no execution found for SBI: %s", sbiID)
	}

	result := *latest
	return &result, nil
}

// FindActive retrieves all active (not completed) executions
func (m *MockSBIExecutionRepository) FindActive() ([]*SBIExecution, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*SBIExecution
	for _, exec := range m.executions {
		if !exec.IsCompleted() {
			result := *exec
			active = append(active, &result)
		}
	}

	return active, nil
}

// FindByStatus retrieves executions with a specific status
func (m *MockSBIExecutionRepository) FindByStatus(status ExecutionStatus) ([]*SBIExecution, error) {
	if m.findErr != nil {
		return nil, m.findErr
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SBIExecution
	for _, exec := range m.executions {
		if exec.Status == status {
			result := *exec
			results = append(results, &result)
		}
	}

	return results, nil
}

// Update updates an existing execution
func (m *MockSBIExecutionRepository) Update(execution *SBIExecution) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.executions[execution.ID]; !exists {
		return fmt.Errorf("execution not found: %s", execution.ID)
	}

	updated := *execution
	m.executions[execution.ID] = &updated
	return nil
}

// Delete removes an execution
func (m *MockSBIExecutionRepository) Delete(id ExecutionID) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.executions[id]; !exists {
		return fmt.Errorf("execution not found: %s", id)
	}

	delete(m.executions, id)
	return nil
}

// SetSaveError configures the mock to return an error on save operations
func (m *MockSBIExecutionRepository) SetSaveError(err error) {
	m.saveErr = err
}

// SetFindError configures the mock to return an error on find operations
func (m *MockSBIExecutionRepository) SetFindError(err error) {
	m.findErr = err
}

// Clear removes all stored executions
func (m *MockSBIExecutionRepository) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions = make(map[ExecutionID]*SBIExecution)
}

// Count returns the number of executions in the mock repository
func (m *MockSBIExecutionRepository) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.executions)
}

// TestRepositorySave tests the Save method
func TestRepositorySave(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")

	err := repo.Save(exec)
	if err != nil {
		t.Fatalf("Failed to save execution: %v", err)
	}

	if repo.Count() != 1 {
		t.Errorf("Expected 1 execution in repository, got %d", repo.Count())
	}
}

// TestRepositorySaveError tests Save method with error condition
func TestRepositorySaveError(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	repo.SetSaveError(fmt.Errorf("database error"))

	exec := NewSBIExecution("SBI-001")
	err := repo.Save(exec)

	if err == nil {
		t.Error("Expected error from Save, got nil")
	}
}

// TestRepositoryFindByID tests the FindByID method
func TestRepositoryFindByID(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")

	repo.Save(exec)

	found, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Fatalf("Failed to find execution: %v", err)
	}

	if found.ID != exec.ID {
		t.Errorf("Expected ID %s, got %s", exec.ID, found.ID)
	}
	if found.SBIID != exec.SBIID {
		t.Errorf("Expected SBIID %s, got %s", exec.SBIID, found.SBIID)
	}
}

// TestRepositoryFindByIDNotFound tests FindByID with non-existent ID
func TestRepositoryFindByIDNotFound(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	_, err := repo.FindByID(ExecutionID("non-existent"))
	if err == nil {
		t.Error("Expected error when finding non-existent execution, got nil")
	}
}

// TestRepositoryFindByIDError tests FindByID with error condition
func TestRepositoryFindByIDError(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	repo.SetFindError(fmt.Errorf("database error"))

	_, err := repo.FindByID(ExecutionID("any-id"))
	if err == nil {
		t.Error("Expected error from FindByID, got nil")
	}
}

// TestRepositoryFindBySBIID tests the FindBySBIID method
func TestRepositoryFindBySBIID(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	sbiID := "SBI-001"

	// Create multiple executions for the same SBI
	exec1 := NewSBIExecution(sbiID)
	time.Sleep(10 * time.Millisecond)
	exec2 := NewSBIExecution(sbiID)

	repo.Save(exec1)
	repo.Save(exec2)

	// Should return the latest one
	found, err := repo.FindBySBIID(sbiID)
	if err != nil {
		t.Fatalf("Failed to find execution by SBI ID: %v", err)
	}

	if found.ID != exec2.ID {
		t.Errorf("Expected to find latest execution %s, got %s", exec2.ID, found.ID)
	}
}

// TestRepositoryFindBySBIIDNotFound tests FindBySBIID with non-existent SBI ID
func TestRepositoryFindBySBIIDNotFound(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	_, err := repo.FindBySBIID("non-existent-sbi")
	if err == nil {
		t.Error("Expected error when finding non-existent SBI, got nil")
	}
}

// TestRepositoryFindActive tests the FindActive method
func TestRepositoryFindActive(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	// Create active execution
	exec1 := NewSBIExecution("SBI-001")
	exec1.TransitionTo(StepImplementTry)

	// Create completed execution
	exec2 := NewSBIExecution("SBI-002")
	exec2.TransitionTo(StepImplementTry)
	exec2.TransitionTo(StepFirstReview)
	exec2.TransitionTo(StepDone)

	repo.Save(exec1)
	repo.Save(exec2)

	active, err := repo.FindActive()
	if err != nil {
		t.Fatalf("Failed to find active executions: %v", err)
	}

	if len(active) != 1 {
		t.Errorf("Expected 1 active execution, got %d", len(active))
	}

	if len(active) > 0 && active[0].ID != exec1.ID {
		t.Errorf("Expected active execution %s, got %s", exec1.ID, active[0].ID)
	}
}

// TestRepositoryFindActiveEmpty tests FindActive with no active executions
func TestRepositoryFindActiveEmpty(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	// Create only completed executions
	exec := NewSBIExecution("SBI-001")
	exec.TransitionTo(StepImplementTry)
	exec.TransitionTo(StepFirstReview)
	exec.TransitionTo(StepDone)
	repo.Save(exec)

	active, err := repo.FindActive()
	if err != nil {
		t.Fatalf("Failed to find active executions: %v", err)
	}

	if len(active) != 0 {
		t.Errorf("Expected 0 active executions, got %d", len(active))
	}
}

// TestRepositoryFindByStatus tests the FindByStatus method
func TestRepositoryFindByStatus(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	// Create executions with different statuses
	exec1 := NewSBIExecution("SBI-001")
	exec1.TransitionTo(StepImplementTry) // StatusWIP

	exec2 := NewSBIExecution("SBI-002")
	exec2.TransitionTo(StepImplementTry)
	exec2.TransitionTo(StepFirstReview) // StatusReview

	exec3 := NewSBIExecution("SBI-003")
	exec3.TransitionTo(StepImplementTry) // StatusWIP

	repo.Save(exec1)
	repo.Save(exec2)
	repo.Save(exec3)

	// Find WIP executions
	wipExecs, err := repo.FindByStatus(StatusWIP)
	if err != nil {
		t.Fatalf("Failed to find WIP executions: %v", err)
	}

	if len(wipExecs) != 2 {
		t.Errorf("Expected 2 WIP executions, got %d", len(wipExecs))
	}

	// Find Review executions
	reviewExecs, err := repo.FindByStatus(StatusReview)
	if err != nil {
		t.Fatalf("Failed to find Review executions: %v", err)
	}

	if len(reviewExecs) != 1 {
		t.Errorf("Expected 1 Review execution, got %d", len(reviewExecs))
	}
}

// TestRepositoryFindByStatusEmpty tests FindByStatus with no matching executions
func TestRepositoryFindByStatusEmpty(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	exec := NewSBIExecution("SBI-001") // StatusReady
	repo.Save(exec)

	wipExecs, err := repo.FindByStatus(StatusWIP)
	if err != nil {
		t.Fatalf("Failed to find WIP executions: %v", err)
	}

	if len(wipExecs) != 0 {
		t.Errorf("Expected 0 WIP executions, got %d", len(wipExecs))
	}
}

// TestRepositoryUpdate tests the Update method
func TestRepositoryUpdate(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")

	repo.Save(exec)

	// Update the execution
	exec.TransitionTo(StepImplementTry)
	err := repo.Update(exec)
	if err != nil {
		t.Fatalf("Failed to update execution: %v", err)
	}

	// Verify the update
	updated, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Fatalf("Failed to find updated execution: %v", err)
	}

	if updated.Step != StepImplementTry {
		t.Errorf("Expected step %s, got %s", StepImplementTry, updated.Step)
	}
}

// TestRepositoryUpdateNotFound tests Update with non-existent execution
func TestRepositoryUpdateNotFound(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")

	err := repo.Update(exec)
	if err == nil {
		t.Error("Expected error when updating non-existent execution, got nil")
	}
}

// TestRepositoryUpdateError tests Update with error condition
func TestRepositoryUpdateError(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")
	repo.Save(exec)

	repo.SetSaveError(fmt.Errorf("database error"))

	err := repo.Update(exec)
	if err == nil {
		t.Error("Expected error from Update, got nil")
	}
}

// TestRepositoryDelete tests the Delete method
func TestRepositoryDelete(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")

	repo.Save(exec)

	if repo.Count() != 1 {
		t.Fatalf("Expected 1 execution before delete, got %d", repo.Count())
	}

	err := repo.Delete(exec.ID)
	if err != nil {
		t.Fatalf("Failed to delete execution: %v", err)
	}

	if repo.Count() != 0 {
		t.Errorf("Expected 0 executions after delete, got %d", repo.Count())
	}

	// Verify deletion
	_, err = repo.FindByID(exec.ID)
	if err == nil {
		t.Error("Expected error when finding deleted execution, got nil")
	}
}

// TestRepositoryDeleteNotFound tests Delete with non-existent execution
func TestRepositoryDeleteNotFound(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	err := repo.Delete(ExecutionID("non-existent"))
	if err == nil {
		t.Error("Expected error when deleting non-existent execution, got nil")
	}
}

// TestRepositoryDeleteError tests Delete with error condition
func TestRepositoryDeleteError(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")
	repo.Save(exec)

	repo.SetSaveError(fmt.Errorf("database error"))

	err := repo.Delete(exec.ID)
	if err == nil {
		t.Error("Expected error from Delete, got nil")
	}
}

// TestRepositoryConcurrency tests concurrent access to the repository
func TestRepositoryConcurrency(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	var wg sync.WaitGroup
	numGoroutines := 10

	// Concurrent saves
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			exec := NewSBIExecution(fmt.Sprintf("SBI-%03d", index))
			repo.Save(exec)
		}(i)
	}

	wg.Wait()

	if repo.Count() != numGoroutines {
		t.Errorf("Expected %d executions after concurrent saves, got %d", numGoroutines, repo.Count())
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			repo.FindActive()
			repo.FindByStatus(StatusReady)
		}(i)
	}

	wg.Wait()
}

// TestRepositoryMultipleSBIs tests managing multiple SBI executions
func TestRepositoryMultipleSBIs(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	// Create executions for different SBIs
	sbiIDs := []string{"SBI-001", "SBI-002", "SBI-003"}
	for _, sbiID := range sbiIDs {
		exec := NewSBIExecution(sbiID)
		repo.Save(exec)
	}

	// Verify each SBI can be found
	for _, sbiID := range sbiIDs {
		exec, err := repo.FindBySBIID(sbiID)
		if err != nil {
			t.Errorf("Failed to find SBI %s: %v", sbiID, err)
		}
		if exec.SBIID != sbiID {
			t.Errorf("Expected SBIID %s, got %s", sbiID, exec.SBIID)
		}
	}
}

// TestRepositoryLifecycleIntegration tests a complete repository lifecycle
func TestRepositoryLifecycleIntegration(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	sbiID := "SBI-LIFECYCLE-001"

	// Create and save new execution
	exec := NewSBIExecution(sbiID)
	if err := repo.Save(exec); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Progress through implementation
	exec.TransitionTo(StepImplementTry)
	if err := repo.Update(exec); err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Verify it's active
	active, err := repo.FindActive()
	if err != nil || len(active) != 1 {
		t.Fatalf("Expected 1 active execution, got %d (err: %v)", len(active), err)
	}

	// Complete the execution
	exec.TransitionTo(StepFirstReview)
	exec.ApplyDecision(DecisionSucceeded)
	exec.TransitionTo(StepDone)
	if err := repo.Update(exec); err != nil {
		t.Fatalf("Failed to update to done: %v", err)
	}

	// Verify it's no longer active
	active, err = repo.FindActive()
	if err != nil || len(active) != 0 {
		t.Errorf("Expected 0 active executions after completion, got %d", len(active))
	}

	// Verify we can still find by ID
	found, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Fatalf("Failed to find completed execution: %v", err)
	}
	if !found.IsCompleted() {
		t.Error("Found execution should be completed")
	}

	// Clean up
	if err := repo.Delete(exec.ID); err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	if repo.Count() != 0 {
		t.Errorf("Expected empty repository after cleanup, got %d executions", repo.Count())
	}
}

// TestRepositoryIsolation tests that returned executions are isolated from repository
func TestRepositoryIsolation(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-001")

	repo.Save(exec)

	// Get execution from repository
	found, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Fatalf("Failed to find execution: %v", err)
	}

	// Modify the returned execution
	originalStep := found.Step
	found.Step = StepImplementTry
	found.Attempt = 99

	// Get execution again from repository
	refound, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Fatalf("Failed to refind execution: %v", err)
	}

	// Verify repository data wasn't affected by external modification
	if refound.Step != originalStep {
		t.Errorf("Repository data was modified externally: expected step %s, got %s", originalStep, refound.Step)
	}
	if refound.Attempt != 0 {
		t.Errorf("Repository data was modified externally: expected attempt 0, got %d", refound.Attempt)
	}
}

// TestRepositoryBoundaryConditions tests edge cases and boundary conditions
func TestRepositoryBoundaryConditions(t *testing.T) {
	t.Run("Save with nil execution", func(t *testing.T) {
		repo := NewMockSBIExecutionRepository()
		// This would cause panic in real scenarios, but testing the behavior
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when saving nil execution")
			}
		}()
		repo.Save(nil)
	})

	t.Run("FindByID with empty string", func(t *testing.T) {
		repo := NewMockSBIExecutionRepository()
		_, err := repo.FindByID(ExecutionID(""))
		if err == nil {
			t.Error("Expected error when finding with empty ID")
		}
	})

	t.Run("FindBySBIID with empty string", func(t *testing.T) {
		repo := NewMockSBIExecutionRepository()
		_, err := repo.FindBySBIID("")
		if err == nil {
			t.Error("Expected error when finding with empty SBI ID")
		}
	})

	t.Run("FindByStatus with invalid status", func(t *testing.T) {
		repo := NewMockSBIExecutionRepository()
		exec := NewSBIExecution("SBI-001")
		repo.Save(exec)

		results, err := repo.FindByStatus(ExecutionStatus("INVALID"))
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("Expected 0 results for invalid status, got %d", len(results))
		}
	})
}

// TestRepositoryFindBySBIIDTimestamp tests timestamp-based ordering in FindBySBIID
func TestRepositoryFindBySBIIDTimestamp(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	sbiID := "SBI-TIMESTAMP-TEST"

	// Create executions with intentional time gaps
	exec1 := NewSBIExecution(sbiID)
	repo.Save(exec1)

	time.Sleep(50 * time.Millisecond)

	exec2 := NewSBIExecution(sbiID)
	repo.Save(exec2)

	time.Sleep(50 * time.Millisecond)

	exec3 := NewSBIExecution(sbiID)
	repo.Save(exec3)

	// Should return the latest one (exec3)
	found, err := repo.FindBySBIID(sbiID)
	if err != nil {
		t.Fatalf("Failed to find execution: %v", err)
	}

	if found.ID != exec3.ID {
		t.Errorf("Expected latest execution %s, got %s", exec3.ID, found.ID)
	}

	// Verify it's actually the latest by timestamp
	if !found.StartedAt.After(exec1.StartedAt) || !found.StartedAt.After(exec2.StartedAt) {
		t.Error("Found execution does not have the latest timestamp")
	}
}

// TestRepositoryMixedStatusOperations tests complex status filtering scenarios
func TestRepositoryMixedStatusOperations(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	// Create a complex scenario with multiple executions in various states
	exec1 := NewSBIExecution("SBI-001")
	exec1.TransitionTo(StepImplementTry)
	repo.Save(exec1)

	exec2 := NewSBIExecution("SBI-002")
	exec2.TransitionTo(StepImplementTry)
	exec2.TransitionTo(StepFirstReview)
	repo.Save(exec2)

	exec3 := NewSBIExecution("SBI-003")
	exec3.TransitionTo(StepImplementTry)
	exec3.TransitionTo(StepFirstReview)
	exec3.TransitionTo(StepDone)
	repo.Save(exec3)

	exec4 := NewSBIExecution("SBI-004")
	repo.Save(exec4) // StatusReady

	// Test FindActive excludes only Done
	active, err := repo.FindActive()
	if err != nil {
		t.Fatalf("Failed to find active executions: %v", err)
	}
	if len(active) != 3 {
		t.Errorf("Expected 3 active executions (Ready, WIP, Review), got %d", len(active))
	}

	// Test FindByStatus for each status
	readyExecs, _ := repo.FindByStatus(StatusReady)
	if len(readyExecs) != 1 {
		t.Errorf("Expected 1 Ready execution, got %d", len(readyExecs))
	}

	wipExecs, _ := repo.FindByStatus(StatusWIP)
	if len(wipExecs) != 1 {
		t.Errorf("Expected 1 WIP execution, got %d", len(wipExecs))
	}

	reviewExecs, _ := repo.FindByStatus(StatusReview)
	if len(reviewExecs) != 1 {
		t.Errorf("Expected 1 Review execution, got %d", len(reviewExecs))
	}

	doneExecs, _ := repo.FindByStatus(StatusDone)
	if len(doneExecs) != 1 {
		t.Errorf("Expected 1 Done execution, got %d", len(doneExecs))
	}
}

// TestRepositorySaveUpdateRaceCondition tests potential race conditions between Save and Update
func TestRepositorySaveUpdateRaceCondition(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-RACE-001")

	var wg sync.WaitGroup
	errors := make([]error, 0)
	var mu sync.Mutex

	// First save the execution
	repo.Save(exec)

	// Concurrent updates to the same execution
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(step ExecutionStep) {
			defer wg.Done()

			// Get a copy
			found, err := repo.FindByID(exec.ID)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
				return
			}

			// Try to update
			err = repo.Update(found)
			if err != nil {
				mu.Lock()
				errors = append(errors, err)
				mu.Unlock()
			}
		}(StepImplementTry)
	}

	wg.Wait()

	if len(errors) > 0 {
		t.Errorf("Encountered %d errors during concurrent updates: %v", len(errors), errors[0])
	}

	// Verify the execution still exists and is valid
	found, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Fatalf("Execution was lost during concurrent updates: %v", err)
	}
	if found == nil {
		t.Error("Found execution is nil")
	}
}

// TestRepositoryFindActiveWithMixedCompletionStates tests edge cases in completion detection
func TestRepositoryFindActiveWithMixedCompletionStates(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	// Create executions at various completion stages
	exec1 := NewSBIExecution("SBI-001")
	exec1.TransitionTo(StepImplementTry)
	exec1.TransitionTo(StepFirstReview)
	exec1.ApplyDecision(DecisionSucceeded)
	// Not yet transitioned to Done
	repo.Save(exec1)

	exec2 := NewSBIExecution("SBI-002")
	exec2.TransitionTo(StepImplementTry)
	exec2.TransitionTo(StepFirstReview)
	exec2.ApplyDecision(DecisionSucceeded)
	exec2.TransitionTo(StepDone) // Fully completed
	repo.Save(exec2)

	exec3 := NewSBIExecution("SBI-003")
	exec3.TransitionTo(StepImplementTry)
	exec3.TransitionTo(StepFirstReview)
	// Decision pending
	repo.Save(exec3)

	active, err := repo.FindActive()
	if err != nil {
		t.Fatalf("Failed to find active executions: %v", err)
	}

	// exec1 and exec3 should be active (not Done), exec2 should not be
	if len(active) != 2 {
		t.Errorf("Expected 2 active executions, got %d", len(active))
	}

	// Verify exec2 is not in active list
	for _, exec := range active {
		if exec.ID == exec2.ID {
			t.Error("Completed execution should not be in active list")
		}
	}
}

// TestRepositoryClearOperation tests the Clear helper method
func TestRepositoryClearOperation(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	// Add multiple executions
	for i := 0; i < 5; i++ {
		exec := NewSBIExecution(fmt.Sprintf("SBI-%03d", i))
		repo.Save(exec)
	}

	if repo.Count() != 5 {
		t.Fatalf("Expected 5 executions before clear, got %d", repo.Count())
	}

	// Clear the repository
	repo.Clear()

	if repo.Count() != 0 {
		t.Errorf("Expected 0 executions after clear, got %d", repo.Count())
	}

	// Verify all executions are gone
	active, err := repo.FindActive()
	if err != nil {
		t.Fatalf("Failed to find active after clear: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("Expected no active executions after clear, got %d", len(active))
	}
}

// TestRepositoryMultipleSBISameTime tests handling of same SBI with very close timestamps
func TestRepositoryMultipleSBISameTime(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	sbiID := "SBI-SAMETIME"

	// Create multiple executions very quickly
	executions := make([]*SBIExecution, 3)
	for i := 0; i < 3; i++ {
		executions[i] = NewSBIExecution(sbiID)
		repo.Save(executions[i])
	}

	// FindBySBIID should return one of them (the latest based on timestamp)
	found, err := repo.FindBySBIID(sbiID)
	if err != nil {
		t.Fatalf("Failed to find execution: %v", err)
	}

	// Verify it's one of our executions
	foundInList := false
	for _, exec := range executions {
		if exec.ID == found.ID {
			foundInList = true
			break
		}
	}
	if !foundInList {
		t.Error("Returned execution is not from our saved executions")
	}
}

// TestRepositoryErrorPersistence tests that errors are properly maintained across operations
func TestRepositoryErrorPersistence(t *testing.T) {
	repo := NewMockSBIExecutionRepository()

	testError := fmt.Errorf("persistent test error")
	repo.SetFindError(testError)

	// All find operations should return the same error
	_, err1 := repo.FindByID(ExecutionID("any"))
	_, err2 := repo.FindBySBIID("any")
	_, err3 := repo.FindActive()
	_, err4 := repo.FindByStatus(StatusReady)

	if err1 == nil || err2 == nil || err3 == nil || err4 == nil {
		t.Error("Expected all find operations to return error")
	}

	// Clear the error
	repo.SetFindError(nil)

	// Now save an execution and verify find works
	exec := NewSBIExecution("SBI-001")
	repo.Save(exec)

	_, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Errorf("Expected find to work after clearing error, got: %v", err)
	}
}

// TestRepositoryUpdatePreservesOtherFields tests that Update doesn't accidentally overwrite unrelated data
func TestRepositoryUpdatePreservesOtherFields(t *testing.T) {
	repo := NewMockSBIExecutionRepository()
	exec := NewSBIExecution("SBI-PRESERVE-001")

	// Save initial execution
	repo.Save(exec)
	originalID := exec.ID
	originalSBIID := exec.SBIID
	originalStartedAt := exec.StartedAt

	// Get a copy and modify it
	toUpdate, err := repo.FindByID(exec.ID)
	if err != nil {
		t.Fatalf("Failed to find execution: %v", err)
	}

	// Transition the copy
	toUpdate.TransitionTo(StepImplementTry)
	toUpdate.TransitionTo(StepFirstReview)

	// Update in repository
	err = repo.Update(toUpdate)
	if err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Retrieve and verify all fields
	updated, err := repo.FindByID(originalID)
	if err != nil {
		t.Fatalf("Failed to find updated execution: %v", err)
	}

	// Verify identity fields are preserved
	if updated.ID != originalID {
		t.Errorf("ID was changed: expected %s, got %s", originalID, updated.ID)
	}
	if updated.SBIID != originalSBIID {
		t.Errorf("SBIID was changed: expected %s, got %s", originalSBIID, updated.SBIID)
	}
	if updated.StartedAt != originalStartedAt {
		t.Errorf("StartedAt was changed: expected %v, got %v", originalStartedAt, updated.StartedAt)
	}

	// Verify state fields were updated
	if updated.Step != StepFirstReview {
		t.Errorf("Step was not updated: expected %s, got %s", StepFirstReview, updated.Step)
	}
}
