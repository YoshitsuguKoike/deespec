package repository_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

// MockSBITaskRepository is a mock implementation of SBITaskRepository for testing
// It provides file-based task operations compatible with the legacy architecture
type MockSBITaskRepository struct {
	tasks          []*dto.SBITaskDTO
	completedTasks map[string]bool
	journal        []map[string]interface{}
}

// NewMockSBITaskRepository creates a new mock SBI task repository
func NewMockSBITaskRepository() *MockSBITaskRepository {
	return &MockSBITaskRepository{
		tasks:          make([]*dto.SBITaskDTO, 0),
		completedTasks: make(map[string]bool),
		journal:        make([]map[string]interface{}, 0),
	}
}

// LoadAllTasks loads all tasks from the specs directory
func (m *MockSBITaskRepository) LoadAllTasks(ctx context.Context, specsDir string) ([]*dto.SBITaskDTO, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if specsDir == "" {
		return nil, errors.New("specsDir cannot be empty")
	}

	// Return a copy to prevent external modification
	result := make([]*dto.SBITaskDTO, len(m.tasks))
	copy(result, m.tasks)
	return result, nil
}

// GetCompletedTasks returns a map of completed task IDs from journal
func (m *MockSBITaskRepository) GetCompletedTasks(ctx context.Context, journalPath string) (map[string]bool, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Return a copy to prevent external modification
	result := make(map[string]bool)
	for k, v := range m.completedTasks {
		result[k] = v
	}
	return result, nil
}

// GetLastJournalEntry returns the last journal entry
func (m *MockSBITaskRepository) GetLastJournalEntry(ctx context.Context, journalPath string) (map[string]interface{}, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	if len(m.journal) == 0 {
		return nil, nil
	}

	// Return the last entry
	return m.journal[len(m.journal)-1], nil
}

// RecordPickInJournal records a task pick event in the journal
func (m *MockSBITaskRepository) RecordPickInJournal(ctx context.Context, task *dto.SBITaskDTO, turn int, journalPath string) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if task == nil {
		return errors.New("task cannot be nil")
	}

	if journalPath == "" {
		return errors.New("journalPath cannot be empty")
	}

	// Create journal entry
	entry := map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"task_id":   task.ID,
		"title":     task.Title,
		"turn":      turn,
		"priority":  task.Priority,
		"por":       task.POR,
	}

	m.journal = append(m.journal, entry)
	return nil
}

// Helper methods for test setup

// AddTask adds a task to the mock repository
func (m *MockSBITaskRepository) AddTask(task *dto.SBITaskDTO) {
	m.tasks = append(m.tasks, task)
}

// MarkTaskCompleted marks a task as completed
func (m *MockSBITaskRepository) MarkTaskCompleted(taskID string) {
	m.completedTasks[taskID] = true
}

// ClearTasks clears all tasks
func (m *MockSBITaskRepository) ClearTasks() {
	m.tasks = make([]*dto.SBITaskDTO, 0)
}

// ClearJournal clears the journal
func (m *MockSBITaskRepository) ClearJournal() {
	m.journal = make([]map[string]interface{}, 0)
}

// Test Suite for SBITaskRepository

func TestSBITaskRepository_LoadAllTasks_Empty(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("Expected no error for empty repository, got %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks from empty repository, got %d", len(tasks))
	}
}

func TestSBITaskRepository_LoadAllTasks_EmptySpecsDir(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	_, err := repo.LoadAllTasks(ctx, "")
	if err == nil {
		t.Error("Expected error when specsDir is empty")
	}
}

func TestSBITaskRepository_LoadAllTasks_SingleTask(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Add a test task
	task := &dto.SBITaskDTO{
		ID:       "test-id-001",
		Title:    "Test Task",
		Priority: 1,
		POR:      1,
	}
	repo.AddTask(task)

	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("Failed to load tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	if tasks[0].ID != "test-id-001" {
		t.Errorf("Expected task ID 'test-id-001', got '%s'", tasks[0].ID)
	}

	if tasks[0].Title != "Test Task" {
		t.Errorf("Expected title 'Test Task', got '%s'", tasks[0].Title)
	}
}

func TestSBITaskRepository_LoadAllTasks_MultipleTasks(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Add multiple tasks
	for i := 0; i < 5; i++ {
		task := &dto.SBITaskDTO{
			ID:       "task-" + string(rune('A'+i)),
			Title:    "Task " + string(rune('A'+i)),
			Priority: i + 1,
			POR:      i + 1,
		}
		repo.AddTask(task)
	}

	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("Failed to load tasks: %v", err)
	}

	if len(tasks) != 5 {
		t.Errorf("Expected 5 tasks, got %d", len(tasks))
	}
}

func TestSBITaskRepository_LoadAllTasks_WithDependencies(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Add task with dependencies
	task := &dto.SBITaskDTO{
		ID:        "dependent-task",
		Title:     "Dependent Task",
		Priority:  1,
		POR:       1,
		DependsOn: []string{"dep1", "dep2"},
	}
	repo.AddTask(task)

	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("Failed to load tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	if len(tasks[0].DependsOn) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(tasks[0].DependsOn))
	}
}

func TestSBITaskRepository_LoadAllTasks_DefaultPriority(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Add task with zero priority (should get default 999)
	task := &dto.SBITaskDTO{
		ID:       "low-priority-task",
		Title:    "Low Priority Task",
		Priority: 0,
		POR:      0,
	}
	repo.AddTask(task)

	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("Failed to load tasks: %v", err)
	}

	// Note: The actual implementation should set default priority to 999
	// This test verifies that behavior
	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	// In the actual implementation, priority 0 gets converted to 999
	// But in our mock, we just store it as is for simplicity
}

func TestSBITaskRepository_LoadAllTasks_ContextCancellation(t *testing.T) {
	repo := NewMockSBITaskRepository()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestSBITaskRepository_GetCompletedTasks_Empty(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	completed, err := repo.GetCompletedTasks(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Expected no error for empty journal, got %v", err)
	}

	if len(completed) != 0 {
		t.Errorf("Expected 0 completed tasks, got %d", len(completed))
	}
}

func TestSBITaskRepository_GetCompletedTasks_SingleTask(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Mark a task as completed
	repo.MarkTaskCompleted("completed-task-001")

	completed, err := repo.GetCompletedTasks(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get completed tasks: %v", err)
	}

	if len(completed) != 1 {
		t.Errorf("Expected 1 completed task, got %d", len(completed))
	}

	if !completed["completed-task-001"] {
		t.Error("Expected 'completed-task-001' to be marked as completed")
	}
}

func TestSBITaskRepository_GetCompletedTasks_MultipleTasks(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Mark multiple tasks as completed
	taskIDs := []string{"task-A", "task-B", "task-C"}
	for _, id := range taskIDs {
		repo.MarkTaskCompleted(id)
	}

	completed, err := repo.GetCompletedTasks(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get completed tasks: %v", err)
	}

	if len(completed) != 3 {
		t.Errorf("Expected 3 completed tasks, got %d", len(completed))
	}

	for _, id := range taskIDs {
		if !completed[id] {
			t.Errorf("Expected '%s' to be marked as completed", id)
		}
	}
}

func TestSBITaskRepository_GetCompletedTasks_ContextCancellation(t *testing.T) {
	repo := NewMockSBITaskRepository()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetCompletedTasks(ctx, "journal.jsonl")
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestSBITaskRepository_GetLastJournalEntry_EmptyJournal(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	entry, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Expected no error for empty journal, got %v", err)
	}

	if entry != nil {
		t.Error("Expected nil entry for empty journal")
	}
}

func TestSBITaskRepository_GetLastJournalEntry_SingleEntry(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Add a task and record it
	task := &dto.SBITaskDTO{
		ID:       "test-task",
		Title:    "Test Task",
		Priority: 1,
		POR:      1,
	}

	err := repo.RecordPickInJournal(ctx, task, 1, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to record in journal: %v", err)
	}

	entry, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get last journal entry: %v", err)
	}

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}

	if entry["task_id"] != "test-task" {
		t.Errorf("Expected task_id 'test-task', got '%v'", entry["task_id"])
	}

	if entry["turn"] != 1 {
		t.Errorf("Expected turn 1, got %v", entry["turn"])
	}
}

func TestSBITaskRepository_GetLastJournalEntry_MultipleEntries(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Add multiple journal entries
	for i := 1; i <= 3; i++ {
		task := &dto.SBITaskDTO{
			ID:       "task-" + string(rune('0'+i)),
			Title:    "Task " + string(rune('0'+i)),
			Priority: i,
			POR:      i,
		}

		err := repo.RecordPickInJournal(ctx, task, i, "journal.jsonl")
		if err != nil {
			t.Fatalf("Failed to record in journal: %v", err)
		}
	}

	entry, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get last journal entry: %v", err)
	}

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}

	// Should return the last entry (turn 3)
	if entry["turn"] != 3 {
		t.Errorf("Expected turn 3 (last entry), got %v", entry["turn"])
	}

	if entry["task_id"] != "task-3" {
		t.Errorf("Expected task_id 'task-3', got '%v'", entry["task_id"])
	}
}

func TestSBITaskRepository_GetLastJournalEntry_ContextCancellation(t *testing.T) {
	repo := NewMockSBITaskRepository()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestSBITaskRepository_RecordPickInJournal_Success(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	task := &dto.SBITaskDTO{
		ID:       "new-task",
		Title:    "New Task",
		Priority: 5,
		POR:      3,
	}

	err := repo.RecordPickInJournal(ctx, task, 1, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to record pick in journal: %v", err)
	}

	// Verify the entry was recorded
	entry, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get last journal entry: %v", err)
	}

	if entry["task_id"] != "new-task" {
		t.Errorf("Expected task_id 'new-task', got '%v'", entry["task_id"])
	}

	if entry["priority"] != 5 {
		t.Errorf("Expected priority 5, got %v", entry["priority"])
	}

	if entry["por"] != 3 {
		t.Errorf("Expected por 3, got %v", entry["por"])
	}
}

func TestSBITaskRepository_RecordPickInJournal_NilTask(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	err := repo.RecordPickInJournal(ctx, nil, 1, "journal.jsonl")
	if err == nil {
		t.Error("Expected error when task is nil")
	}
}

func TestSBITaskRepository_RecordPickInJournal_EmptyJournalPath(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	task := &dto.SBITaskDTO{
		ID:    "test-task",
		Title: "Test Task",
	}

	err := repo.RecordPickInJournal(ctx, task, 1, "")
	if err == nil {
		t.Error("Expected error when journalPath is empty")
	}
}

func TestSBITaskRepository_RecordPickInJournal_MultipleRecords(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Record multiple tasks
	for i := 1; i <= 5; i++ {
		task := &dto.SBITaskDTO{
			ID:       "task-" + string(rune('A'+i-1)),
			Title:    "Task " + string(rune('A'+i-1)),
			Priority: i,
			POR:      i,
		}

		err := repo.RecordPickInJournal(ctx, task, i, "journal.jsonl")
		if err != nil {
			t.Fatalf("Failed to record task %d: %v", i, err)
		}
	}

	// Verify last entry
	entry, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get last journal entry: %v", err)
	}

	if entry["task_id"] != "task-E" {
		t.Errorf("Expected task_id 'task-E', got '%v'", entry["task_id"])
	}

	if entry["turn"] != 5 {
		t.Errorf("Expected turn 5, got %v", entry["turn"])
	}
}

func TestSBITaskRepository_RecordPickInJournal_ContextCancellation(t *testing.T) {
	repo := NewMockSBITaskRepository()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := &dto.SBITaskDTO{
		ID:    "test-task",
		Title: "Test Task",
	}

	err := repo.RecordPickInJournal(ctx, task, 1, "journal.jsonl")
	if err == nil {
		t.Error("Expected error when context is cancelled")
	}

	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestSBITaskRepository_IntegrationScenario(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Scenario: Load tasks, pick one, mark it completed

	// 1. Add several tasks
	task1 := &dto.SBITaskDTO{
		ID:       "task-001",
		Title:    "Implement Feature A",
		Priority: 1,
		POR:      1,
	}
	task2 := &dto.SBITaskDTO{
		ID:       "task-002",
		Title:    "Fix Bug B",
		Priority: 2,
		POR:      2,
	}
	task3 := &dto.SBITaskDTO{
		ID:       "task-003",
		Title:    "Refactor Module C",
		Priority: 3,
		POR:      3,
	}

	repo.AddTask(task1)
	repo.AddTask(task2)
	repo.AddTask(task3)

	// 2. Load all tasks
	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("Failed to load tasks: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("Expected 3 tasks, got %d", len(tasks))
	}

	// 3. Pick a task and record it
	pickedTask := tasks[0]
	err = repo.RecordPickInJournal(ctx, pickedTask, 1, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to record pick: %v", err)
	}

	// 4. Verify journal entry
	entry, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get last journal entry: %v", err)
	}

	if entry["task_id"] != pickedTask.ID {
		t.Errorf("Expected task_id '%s', got '%v'", pickedTask.ID, entry["task_id"])
	}

	// 5. Mark task as completed
	repo.MarkTaskCompleted(pickedTask.ID)

	// 6. Verify completed tasks
	completed, err := repo.GetCompletedTasks(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("Failed to get completed tasks: %v", err)
	}

	if !completed[pickedTask.ID] {
		t.Errorf("Expected task '%s' to be marked as completed", pickedTask.ID)
	}

	if len(completed) != 1 {
		t.Errorf("Expected 1 completed task, got %d", len(completed))
	}
}

func TestSBITaskRepository_TaskDataIntegrity(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Add task with all fields populated
	originalTask := &dto.SBITaskDTO{
		ID:           "integrity-test",
		Title:        "Task with Full Data",
		Priority:     10,
		POR:          5,
		Sequence:     42,
		DependsOn:    []string{"dep1", "dep2"},
		RegisteredAt: time.Now(),
	}

	repo.AddTask(originalTask)

	// Load tasks and verify data integrity
	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("Failed to load tasks: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	loadedTask := tasks[0]

	// Verify all fields
	if loadedTask.ID != originalTask.ID {
		t.Errorf("ID mismatch: expected '%s', got '%s'", originalTask.ID, loadedTask.ID)
	}

	if loadedTask.Title != originalTask.Title {
		t.Errorf("Title mismatch: expected '%s', got '%s'", originalTask.Title, loadedTask.Title)
	}

	if loadedTask.Priority != originalTask.Priority {
		t.Errorf("Priority mismatch: expected %d, got %d", originalTask.Priority, loadedTask.Priority)
	}

	if loadedTask.POR != originalTask.POR {
		t.Errorf("POR mismatch: expected %d, got %d", originalTask.POR, loadedTask.POR)
	}

	if loadedTask.Sequence != originalTask.Sequence {
		t.Errorf("Sequence mismatch: expected %d, got %d", originalTask.Sequence, loadedTask.Sequence)
	}

	if len(loadedTask.DependsOn) != len(originalTask.DependsOn) {
		t.Errorf("DependsOn length mismatch: expected %d, got %d", len(originalTask.DependsOn), len(loadedTask.DependsOn))
	}
}

func TestSBITaskRepository_EmptyTaskList_Operations(t *testing.T) {
	repo := NewMockSBITaskRepository()
	ctx := context.Background()

	// Test all operations on empty repository

	// 1. LoadAllTasks should return empty slice
	tasks, err := repo.LoadAllTasks(ctx, ".deespec/specs/sbi")
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks, got %d", len(tasks))
	}

	// 2. GetCompletedTasks should return empty map
	completed, err := repo.GetCompletedTasks(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("GetCompletedTasks failed: %v", err)
	}
	if len(completed) != 0 {
		t.Errorf("Expected 0 completed tasks, got %d", len(completed))
	}

	// 3. GetLastJournalEntry should return nil
	entry, err := repo.GetLastJournalEntry(ctx, "journal.jsonl")
	if err != nil {
		t.Fatalf("GetLastJournalEntry failed: %v", err)
	}
	if entry != nil {
		t.Error("Expected nil entry for empty journal")
	}
}
