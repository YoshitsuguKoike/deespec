package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockTaskRepository is a mock implementation of TaskRepository for testing
// It provides a unified interface for managing EPIC, PBI, and SBI tasks
type MockTaskRepository struct {
	mu    sync.RWMutex
	tasks map[repository.TaskID]task.Task
}

// NewMockTaskRepository creates a new mock task repository
func NewMockTaskRepository() *MockTaskRepository {
	return &MockTaskRepository{
		tasks: make(map[repository.TaskID]task.Task),
	}
}

// FindByID retrieves a task by its ID (works for EPIC/PBI/SBI)
func (m *MockTaskRepository) FindByID(ctx context.Context, id repository.TaskID) (task.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

// Save persists a task entity
func (m *MockTaskRepository) Save(ctx context.Context, t task.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t == nil {
		return errors.New("task cannot be nil")
	}

	id := repository.TaskID(t.ID().String())
	m.tasks[id] = t
	return nil
}

// Delete removes a task
func (m *MockTaskRepository) Delete(ctx context.Context, id repository.TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, exists := m.tasks[id]
	if !exists {
		return ErrTaskNotFound
	}

	// Type-specific deletion rules
	switch concrete := t.(type) {
	case *epic.EPIC:
		if !concrete.CanDelete() {
			return errors.New("cannot delete EPIC with child PBIs")
		}
	case *pbi.PBI:
		if !concrete.CanDelete() {
			return errors.New("cannot delete PBI with child SBIs")
		}
	case *sbi.SBI:
		if !concrete.CanDelete() {
			return errors.New("cannot delete SBI that is currently being executed")
		}
	}

	delete(m.tasks, id)
	return nil
}

// List retrieves tasks by filter criteria
func (m *MockTaskRepository) List(ctx context.Context, filter repository.TaskFilter) ([]task.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []task.Task
	for _, t := range m.tasks {
		if m.matchesFilter(t, filter) {
			result = append(result, t)
		}
	}

	// Apply offset
	if filter.Offset < len(result) {
		result = result[filter.Offset:]
	} else {
		result = nil
	}

	// Apply limit
	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (m *MockTaskRepository) matchesFilter(t task.Task, filter repository.TaskFilter) bool {
	// Type filter
	if len(filter.Types) > 0 {
		matched := false
		for _, filterType := range filter.Types {
			taskType := m.convertModelTypeToRepositoryType(t.Type())
			if taskType == filterType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Status filter
	if len(filter.Statuses) > 0 {
		matched := false
		for _, filterStatus := range filter.Statuses {
			if repository.Status(t.Status().String()) == filterStatus {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Step filter
	if len(filter.Steps) > 0 {
		matched := false
		for _, filterStep := range filter.Steps {
			if repository.Step(t.CurrentStep().String()) == filterStep {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Parent ID filter
	if filter.ParentID != nil {
		if t.ParentTaskID() == nil {
			return false
		}
		if repository.TaskID(t.ParentTaskID().String()) != *filter.ParentID {
			return false
		}
	}

	// Has parent filter
	if filter.HasParent != nil {
		hasParent := t.ParentTaskID() != nil
		if hasParent != *filter.HasParent {
			return false
		}
	}

	return true
}

func (m *MockTaskRepository) convertModelTypeToRepositoryType(mt model.TaskType) repository.TaskType {
	switch mt {
	case model.TaskTypeEPIC:
		return repository.TaskTypeEPIC
	case model.TaskTypePBI:
		return repository.TaskTypePBI
	case model.TaskTypeSBI:
		return repository.TaskTypeSBI
	default:
		return ""
	}
}

// ErrTaskNotFound is returned when a task is not found
var ErrTaskNotFound = errors.New("task not found")

// Test Suite for TaskRepository

func TestTaskRepository_FindByID_EPIC(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create and save an EPIC
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{
		EstimatedStoryPoints: 10,
		Priority:             1,
		Labels:               []string{"backend"},
	})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	taskID := repository.TaskID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Test finding the EPIC through unified interface
	found, err := repo.FindByID(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to find task: %v", err)
	}

	if found.ID().String() != e.ID().String() {
		t.Errorf("Expected task ID %s, got %s", e.ID().String(), found.ID().String())
	}

	if found.Type() != model.TaskTypeEPIC {
		t.Errorf("Expected task type EPIC, got %s", found.Type())
	}

	if found.Title() != "Test EPIC" {
		t.Errorf("Expected title 'Test EPIC', got '%s'", found.Title())
	}
}

func TestTaskRepository_FindByID_PBI(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create and save a PBI
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{
		StoryPoints: 5,
		Priority:    2,
	})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	taskID := repository.TaskID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Test finding the PBI through unified interface
	found, err := repo.FindByID(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to find task: %v", err)
	}

	if found.Type() != model.TaskTypePBI {
		t.Errorf("Expected task type PBI, got %s", found.Type())
	}

	if found.Title() != "Test PBI" {
		t.Errorf("Expected title 'Test PBI', got '%s'", found.Title())
	}
}

func TestTaskRepository_FindByID_SBI(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create and save an SBI
	s, err := sbi.NewSBI("Test SBI", "Description", nil, sbi.SBIMetadata{
		EstimatedHours: 4.0,
		Priority:       1,
	})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	taskID := repository.TaskID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Test finding the SBI through unified interface
	found, err := repo.FindByID(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to find task: %v", err)
	}

	if found.Type() != model.TaskTypeSBI {
		t.Errorf("Expected task type SBI, got %s", found.Type())
	}

	if found.Title() != "Test SBI" {
		t.Errorf("Expected title 'Test SBI', got '%s'", found.Title())
	}
}

func TestTaskRepository_FindByIDNotFound(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Try to find non-existent task
	_, err := repo.FindByID(ctx, repository.TaskID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when finding non-existent task")
	}

	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Expected ErrTaskNotFound, got %v", err)
	}
}

func TestTaskRepository_SaveNil(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Try to save nil task
	err := repo.Save(ctx, nil)
	if err == nil {
		t.Error("Expected error when saving nil task")
	}
}

func TestTaskRepository_SaveMultipleTypes(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create tasks of different types
	e, err := epic.NewEPIC("EPIC 1", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	p, err := pbi.NewPBI("PBI 1", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	s, err := sbi.NewSBI("SBI 1", "Description", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	// Save all tasks through unified interface
	tasks := []task.Task{e, p, s}
	for _, tsk := range tasks {
		err = repo.Save(ctx, tsk)
		if err != nil {
			t.Fatalf("Failed to save task: %v", err)
		}
	}

	// Verify all tasks can be retrieved
	for _, tsk := range tasks {
		taskID := repository.TaskID(tsk.ID().String())
		found, err := repo.FindByID(ctx, taskID)
		if err != nil {
			t.Fatalf("Failed to find task: %v", err)
		}

		if found.ID().String() != tsk.ID().String() {
			t.Errorf("Expected task ID %s, got %s", tsk.ID().String(), found.ID().String())
		}
	}
}

func TestTaskRepository_Delete_EPIC(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create EPIC without PBIs
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	taskID := repository.TaskID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Delete the EPIC
	err = repo.Delete(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// Verify task is deleted
	_, err = repo.FindByID(ctx, taskID)
	if err == nil {
		t.Error("Expected error when finding deleted task")
	}
}

func TestTaskRepository_Delete_EPICWithPBIs(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create EPIC with a PBI
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	pbiID := model.NewTaskID()
	err = e.AddPBI(pbiID)
	if err != nil {
		t.Fatalf("Failed to add PBI: %v", err)
	}

	taskID := repository.TaskID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Try to delete EPIC with PBIs (should fail)
	err = repo.Delete(ctx, taskID)
	if err == nil {
		t.Error("Expected error when deleting EPIC with PBIs")
	}
}

func TestTaskRepository_Delete_PBI(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create PBI without SBIs
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	taskID := repository.TaskID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Delete the PBI
	err = repo.Delete(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// Verify task is deleted
	_, err = repo.FindByID(ctx, taskID)
	if err == nil {
		t.Error("Expected error when finding deleted task")
	}
}

func TestTaskRepository_Delete_SBI(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create SBI in deletable state
	s, err := sbi.NewSBI("Test SBI", "Description", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	taskID := repository.TaskID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Delete the SBI
	err = repo.Delete(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to delete task: %v", err)
	}

	// Verify task is deleted
	_, err = repo.FindByID(ctx, taskID)
	if err == nil {
		t.Error("Expected error when finding deleted task")
	}
}

func TestTaskRepository_DeleteNotFound(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Try to delete non-existent task
	err := repo.Delete(ctx, repository.TaskID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when deleting non-existent task")
	}

	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("Expected ErrTaskNotFound, got %v", err)
	}
}

func TestTaskRepository_ListAll(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create tasks of different types
	e, _ := epic.NewEPIC("EPIC 1", "Description", epic.EPICMetadata{})
	p, _ := pbi.NewPBI("PBI 1", "Description", nil, pbi.PBIMetadata{})
	s, _ := sbi.NewSBI("SBI 1", "Description", nil, sbi.SBIMetadata{})

	repo.Save(ctx, e)
	repo.Save(ctx, p)
	repo.Save(ctx, s)

	// List all tasks
	tasks, err := repo.List(ctx, repository.TaskFilter{})
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
}

func TestTaskRepository_ListByType(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create multiple tasks of different types
	for i := 0; i < 2; i++ {
		e, _ := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		repo.Save(ctx, e)
	}

	for i := 0; i < 3; i++ {
		p, _ := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	for i := 0; i < 4; i++ {
		s, _ := sbi.NewSBI("SBI "+string(rune('A'+i)), "Description", nil, sbi.SBIMetadata{})
		repo.Save(ctx, s)
	}

	// Filter by EPIC type
	filter := repository.TaskFilter{
		Types: []repository.TaskType{repository.TaskTypeEPIC},
	}

	epics, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 2 {
		t.Errorf("Expected 2 EPICs, got %d", len(epics))
	}

	for _, tsk := range epics {
		if tsk.Type() != model.TaskTypeEPIC {
			t.Errorf("Expected type EPIC, got %s", tsk.Type())
		}
	}

	// Filter by PBI type
	filter = repository.TaskFilter{
		Types: []repository.TaskType{repository.TaskTypePBI},
	}

	pbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 3 {
		t.Errorf("Expected 3 PBIs, got %d", len(pbis))
	}

	// Filter by SBI type
	filter = repository.TaskFilter{
		Types: []repository.TaskType{repository.TaskTypeSBI},
	}

	sbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 4 {
		t.Errorf("Expected 4 SBIs, got %d", len(sbis))
	}
}

func TestTaskRepository_ListByMultipleTypes(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create tasks of different types
	e, _ := epic.NewEPIC("EPIC 1", "Description", epic.EPICMetadata{})
	p, _ := pbi.NewPBI("PBI 1", "Description", nil, pbi.PBIMetadata{})
	s, _ := sbi.NewSBI("SBI 1", "Description", nil, sbi.SBIMetadata{})

	repo.Save(ctx, e)
	repo.Save(ctx, p)
	repo.Save(ctx, s)

	// Filter by EPIC and PBI types
	filter := repository.TaskFilter{
		Types: []repository.TaskType{
			repository.TaskTypeEPIC,
			repository.TaskTypePBI,
		},
	}

	tasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks (EPIC + PBI), got %d", len(tasks))
	}

	// Verify task types
	for _, tsk := range tasks {
		taskType := tsk.Type()
		if taskType != model.TaskTypeEPIC && taskType != model.TaskTypePBI {
			t.Errorf("Unexpected task type %s in filtered results", taskType)
		}
	}
}

func TestTaskRepository_ListByStatus(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create tasks with different statuses
	tasks := []task.Task{}

	// PENDING tasks
	for i := 0; i < 2; i++ {
		p, _ := pbi.NewPBI("PBI Pending "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
		tasks = append(tasks, p)
	}

	// IN_PROGRESS tasks
	for i := 0; i < 3; i++ {
		p, _ := pbi.NewPBI("PBI In Progress "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		p.UpdateStatus(model.StatusPicked)
		repo.Save(ctx, p)
		tasks = append(tasks, p)
	}

	// Filter by PENDING status
	filter := repository.TaskFilter{
		Statuses: []repository.Status{repository.Status(model.StatusPending.String())},
	}

	pendingTasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(pendingTasks) != 2 {
		t.Errorf("Expected 2 PENDING tasks, got %d", len(pendingTasks))
	}

	for _, tsk := range pendingTasks {
		if tsk.Status() != model.StatusPending {
			t.Errorf("Expected status PENDING, got %s", tsk.Status())
		}
	}

	// Filter by PICKED status
	filter = repository.TaskFilter{
		Statuses: []repository.Status{repository.Status(model.StatusPicked.String())},
	}

	pickedTasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(pickedTasks) != 3 {
		t.Errorf("Expected 3 PICKED tasks, got %d", len(pickedTasks))
	}
}

func TestTaskRepository_ListByStep(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create tasks in different steps
	// PICK step (PENDING status)
	for i := 0; i < 2; i++ {
		p, _ := pbi.NewPBI("PBI Pick "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	// IMPLEMENT step (IMPLEMENTING status)
	for i := 0; i < 3; i++ {
		p, _ := pbi.NewPBI("PBI Implement "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		p.UpdateStatus(model.StatusPicked)
		p.UpdateStatus(model.StatusImplementing)
		repo.Save(ctx, p)
	}

	// Filter by pick step
	filter := repository.TaskFilter{
		Steps: []repository.Step{repository.Step(model.StepPick.String())},
	}

	pickTasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(pickTasks) != 2 {
		t.Errorf("Expected 2 pick step tasks, got %d", len(pickTasks))
	}

	// Filter by implement step
	filter = repository.TaskFilter{
		Steps: []repository.Step{repository.Step(model.StepImplement.String())},
	}

	implementTasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(implementTasks) != 3 {
		t.Errorf("Expected 3 implement step tasks, got %d", len(implementTasks))
	}
}

func TestTaskRepository_ListByParentID(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create parent EPIC
	parentEPIC, _ := epic.NewEPIC("Parent EPIC", "Description", epic.EPICMetadata{})
	repo.Save(ctx, parentEPIC)

	// Create PBIs with and without parent
	epicID := parentEPIC.ID()
	for i := 0; i < 3; i++ {
		p, _ := pbi.NewPBI("PBI with parent "+string(rune('A'+i)), "Description", &epicID, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	for i := 0; i < 2; i++ {
		p, _ := pbi.NewPBI("PBI without parent "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	// Filter by parent ID
	parentIDFilter := repository.TaskID(epicID.String())
	filter := repository.TaskFilter{
		ParentID: &parentIDFilter,
	}

	childTasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(childTasks) != 3 {
		t.Errorf("Expected 3 child tasks, got %d", len(childTasks))
	}

	for _, tsk := range childTasks {
		if tsk.ParentTaskID() == nil {
			t.Error("Expected task to have parent ID")
		} else if tsk.ParentTaskID().String() != epicID.String() {
			t.Errorf("Expected parent ID %s, got %s", epicID.String(), tsk.ParentTaskID().String())
		}
	}
}

func TestTaskRepository_ListByHasParent(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create parent EPIC
	parentEPIC, _ := epic.NewEPIC("Parent EPIC", "Description", epic.EPICMetadata{})
	repo.Save(ctx, parentEPIC)

	epicID := parentEPIC.ID()

	// Create PBIs with parent
	for i := 0; i < 3; i++ {
		p, _ := pbi.NewPBI("PBI with parent "+string(rune('A'+i)), "Description", &epicID, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	// Create PBIs without parent
	for i := 0; i < 2; i++ {
		p, _ := pbi.NewPBI("PBI without parent "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	// Filter tasks with parent
	hasParent := true
	filter := repository.TaskFilter{
		HasParent: &hasParent,
	}

	tasksWithParent, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasksWithParent) != 3 {
		t.Errorf("Expected 3 tasks with parent, got %d", len(tasksWithParent))
	}

	for _, tsk := range tasksWithParent {
		if tsk.ParentTaskID() == nil {
			t.Error("Expected task to have parent")
		}
	}

	// Filter tasks without parent
	noParent := false
	filter = repository.TaskFilter{
		HasParent: &noParent,
	}

	tasksWithoutParent, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	// Should include the parent EPIC (1) + 2 standalone PBIs = 3
	if len(tasksWithoutParent) != 3 {
		t.Errorf("Expected 3 tasks without parent, got %d", len(tasksWithoutParent))
	}

	for _, tsk := range tasksWithoutParent {
		if tsk.ParentTaskID() != nil {
			t.Error("Expected task to have no parent")
		}
	}
}

func TestTaskRepository_ListWithPagination(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create 10 tasks
	for i := 0; i < 10; i++ {
		p, _ := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	// Test with limit
	filter := repository.TaskFilter{
		Limit: 3,
	}

	tasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks with limit=3, got %d", len(tasks))
	}

	// Test with offset
	filter = repository.TaskFilter{
		Offset: 7,
	}

	tasks, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks with offset=7, got %d", len(tasks))
	}

	// Test with both limit and offset
	filter = repository.TaskFilter{
		Limit:  5,
		Offset: 3,
	}

	tasks, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 5 {
		t.Errorf("Expected 5 tasks with limit=5 and offset=3, got %d", len(tasks))
	}
}

func TestTaskRepository_ListWithCombinedFilters(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create parent EPIC
	parentEPIC, _ := epic.NewEPIC("Parent EPIC", "Description", epic.EPICMetadata{})
	repo.Save(ctx, parentEPIC)

	epicID := parentEPIC.ID()

	// Create 10 PBIs with mixed properties
	for i := 0; i < 10; i++ {
		var parent *model.TaskID
		if i < 5 {
			parent = &epicID
		}

		p, _ := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", parent, pbi.PBIMetadata{})

		// Every other PBI gets PICKED status
		if i%2 == 0 {
			p.UpdateStatus(model.StatusPicked)
		}

		repo.Save(ctx, p)
	}

	// Filter: PBI type + PICKED status + with parent + pagination
	parentIDFilter := repository.TaskID(epicID.String())
	filter := repository.TaskFilter{
		Types:    []repository.TaskType{repository.TaskTypePBI},
		Statuses: []repository.Status{repository.Status(model.StatusPicked.String())},
		ParentID: &parentIDFilter,
		Limit:    2,
		Offset:   0,
	}

	tasks, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	// Should have 3 PICKED PBIs with parent (indices 0, 2, 4), with limit 2, we get 2
	if len(tasks) != 2 {
		t.Errorf("Expected 2 tasks with combined filter, got %d", len(tasks))
	}

	for _, tsk := range tasks {
		if tsk.Type() != model.TaskTypePBI {
			t.Errorf("Expected type PBI, got %s", tsk.Type())
		}
		if tsk.Status() != model.StatusPicked {
			t.Errorf("Expected status PICKED, got %s", tsk.Status())
		}
		if tsk.ParentTaskID() == nil {
			t.Error("Expected task to have parent")
		}
	}
}

func TestTaskRepository_Concurrency(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	errorChan := make(chan error, 30)

	// Concurrent writes of different task types
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			e, err := epic.NewEPIC("EPIC "+string(rune('A'+index)), "Description", epic.EPICMetadata{})
			if err != nil {
				errorChan <- err
				return
			}

			err = repo.Save(ctx, e)
			if err != nil {
				errorChan <- err
			}
		}(i)

		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			p, err := pbi.NewPBI("PBI "+string(rune('A'+index)), "Description", nil, pbi.PBIMetadata{})
			if err != nil {
				errorChan <- err
				return
			}

			err = repo.Save(ctx, p)
			if err != nil {
				errorChan <- err
			}
		}(i)

		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			s, err := sbi.NewSBI("SBI "+string(rune('A'+index)), "Description", nil, sbi.SBIMetadata{})
			if err != nil {
				errorChan <- err
				return
			}

			err = repo.Save(ctx, s)
			if err != nil {
				errorChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify all tasks were saved
	tasks, err := repo.List(ctx, repository.TaskFilter{})
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 30 {
		t.Errorf("Expected 30 tasks after concurrent saves, got %d", len(tasks))
	}
}

func TestTaskRepository_ConcurrentReadsAndWrites(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 5; i++ {
		p, _ := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 30)

	// Concurrent reads
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := repo.List(ctx, repository.TaskFilter{})
			if err != nil {
				errorChan <- err
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			s, err := sbi.NewSBI("New SBI "+string(rune('A'+index)), "Description", nil, sbi.SBIMetadata{})
			if err != nil {
				errorChan <- err
				return
			}

			err = repo.Save(ctx, s)
			if err != nil {
				errorChan <- err
			}
		}(i + 10)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent operation failed: %v", err)
	}

	// Verify final state
	tasks, err := repo.List(ctx, repository.TaskFilter{})
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 20 {
		t.Errorf("Expected 20 tasks after concurrent operations, got %d", len(tasks))
	}
}

func TestTaskRepository_EmptyList(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// List from empty repository
	tasks, err := repo.List(ctx, repository.TaskFilter{})
	if err != nil {
		t.Fatalf("Failed to list tasks: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks from empty repository, got %d", len(tasks))
	}
}

func TestTaskRepository_StatusTransitions(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create task and test status transitions through unified interface
	p, _ := pbi.NewPBI("Status Test", "Description", nil, pbi.PBIMetadata{})
	repo.Save(ctx, p)

	taskID := repository.TaskID(p.ID().String())

	// Test valid status transitions
	transitions := []model.Status{
		model.StatusPending,
		model.StatusPicked,
		model.StatusImplementing,
		model.StatusReviewing,
		model.StatusDone,
	}

	for i, expectedStatus := range transitions {
		// First transition is already set (PENDING)
		if i > 0 {
			err := p.UpdateStatus(expectedStatus)
			if err != nil {
				t.Fatalf("Failed to transition to %s: %v", expectedStatus.String(), err)
			}

			err = repo.Save(ctx, p)
			if err != nil {
				t.Fatalf("Failed to save task: %v", err)
			}
		}

		// Verify status through unified interface
		found, err := repo.FindByID(ctx, taskID)
		if err != nil {
			t.Fatalf("Failed to find task: %v", err)
		}

		if found.Status() != expectedStatus {
			t.Errorf("Transition %d: Expected status %s, got %s", i, expectedStatus.String(), found.Status().String())
		}
	}
}

func TestTaskRepository_UnifiedInterfaceOperations(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create tasks of different types
	e, _ := epic.NewEPIC("Test EPIC", "EPIC Description", epic.EPICMetadata{})
	p, _ := pbi.NewPBI("Test PBI", "PBI Description", nil, pbi.PBIMetadata{})
	s, _ := sbi.NewSBI("Test SBI", "SBI Description", nil, sbi.SBIMetadata{})

	tasks := []task.Task{e, p, s}

	// Save all through unified interface
	for _, tsk := range tasks {
		err := repo.Save(ctx, tsk)
		if err != nil {
			t.Fatalf("Failed to save task: %v", err)
		}
	}

	// Retrieve and verify all through unified interface
	for _, original := range tasks {
		taskID := repository.TaskID(original.ID().String())
		found, err := repo.FindByID(ctx, taskID)
		if err != nil {
			t.Fatalf("Failed to find task: %v", err)
		}

		// Verify common interface methods
		if found.ID().String() != original.ID().String() {
			t.Errorf("ID mismatch: expected %s, got %s", original.ID().String(), found.ID().String())
		}

		if found.Title() != original.Title() {
			t.Errorf("Title mismatch: expected %s, got %s", original.Title(), found.Title())
		}

		if found.Description() != original.Description() {
			t.Errorf("Description mismatch: expected %s, got %s", original.Description(), found.Description())
		}

		if found.Status() != original.Status() {
			t.Errorf("Status mismatch: expected %s, got %s", original.Status(), found.Status())
		}

		if found.Type() != original.Type() {
			t.Errorf("Type mismatch: expected %s, got %s", original.Type(), found.Type())
		}
	}
}

func TestTaskRepository_PaginationEdgeCases(t *testing.T) {
	repo := NewMockTaskRepository()
	ctx := context.Background()

	// Create 5 tasks
	for i := 0; i < 5; i++ {
		p, _ := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		repo.Save(ctx, p)
	}

	tests := []struct {
		name          string
		filter        repository.TaskFilter
		expectedCount int
		description   string
	}{
		{
			name:          "Offset exceeds total",
			filter:        repository.TaskFilter{Offset: 10},
			expectedCount: 0,
			description:   "Offset larger than total should return empty",
		},
		{
			name:          "Limit exceeds remaining",
			filter:        repository.TaskFilter{Limit: 10},
			expectedCount: 5,
			description:   "Limit larger than total should return all",
		},
		{
			name:          "Offset equals total",
			filter:        repository.TaskFilter{Offset: 5},
			expectedCount: 0,
			description:   "Offset equal to total should return empty",
		},
		{
			name:          "Limit is zero",
			filter:        repository.TaskFilter{Limit: 0},
			expectedCount: 5,
			description:   "Limit of 0 should return all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasks, err := repo.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Failed to list tasks: %v", err)
			}

			if len(tasks) != tt.expectedCount {
				t.Errorf("%s: expected %d tasks, got %d", tt.description, tt.expectedCount, len(tasks))
			}
		})
	}
}
