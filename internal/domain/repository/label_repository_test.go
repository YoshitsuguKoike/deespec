package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockLabelRepository is a mock implementation of LabelRepository for testing
type MockLabelRepository struct {
	mu            sync.RWMutex
	labels        map[int]*label.Label
	nextID        int
	taskLabels    map[string][]int // taskID -> []labelID
	labelTasks    map[int][]string // labelID -> []taskID
	labelChildren map[int][]*label.Label
}

// NewMockLabelRepository creates a new mock label repository
func NewMockLabelRepository() *MockLabelRepository {
	return &MockLabelRepository{
		labels:        make(map[int]*label.Label),
		nextID:        1,
		taskLabels:    make(map[string][]int),
		labelTasks:    make(map[int][]string),
		labelChildren: make(map[int][]*label.Label),
	}
}

// Save persists a label
func (m *MockLabelRepository) Save(ctx context.Context, lbl *label.Label) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lbl == nil {
		return errors.New("label cannot be nil")
	}

	// Assign ID if new label
	if lbl.ID() == 0 {
		lbl.SetID(m.nextID)
		m.nextID++
	}

	// Make a copy to avoid external modifications
	m.labels[lbl.ID()] = lbl

	// Update hierarchical structure
	if parentID := lbl.ParentLabelID(); parentID != nil {
		found := false
		for _, child := range m.labelChildren[*parentID] {
			if child.ID() == lbl.ID() {
				found = true
				break
			}
		}
		if !found {
			m.labelChildren[*parentID] = append(m.labelChildren[*parentID], lbl)
		}
	}

	return nil
}

// FindByID retrieves a label by ID
func (m *MockLabelRepository) FindByID(ctx context.Context, id int) (*label.Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lbl, exists := m.labels[id]
	if !exists {
		return nil, ErrLabelNotFound
	}
	return lbl, nil
}

// FindByName retrieves a label by name
func (m *MockLabelRepository) FindByName(ctx context.Context, name string) (*label.Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, lbl := range m.labels {
		if lbl.Name() == name {
			return lbl, nil
		}
	}
	return nil, ErrLabelNotFound
}

// FindAll retrieves all labels
func (m *MockLabelRepository) FindAll(ctx context.Context) ([]*label.Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*label.Label, 0, len(m.labels))
	for _, lbl := range m.labels {
		result = append(result, lbl)
	}
	return result, nil
}

// FindActive retrieves all active labels
func (m *MockLabelRepository) FindActive(ctx context.Context) ([]*label.Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*label.Label, 0)
	for _, lbl := range m.labels {
		if lbl.IsActive() {
			result = append(result, lbl)
		}
	}
	return result, nil
}

// Update updates a label
func (m *MockLabelRepository) Update(ctx context.Context, lbl *label.Label) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lbl == nil {
		return errors.New("label cannot be nil")
	}

	if _, exists := m.labels[lbl.ID()]; !exists {
		return ErrLabelNotFound
	}

	m.labels[lbl.ID()] = lbl
	return nil
}

// Delete removes a label
func (m *MockLabelRepository) Delete(ctx context.Context, id int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.labels[id]; !exists {
		return ErrLabelNotFound
	}

	// Remove from parent's children
	lbl := m.labels[id]
	if parentID := lbl.ParentLabelID(); parentID != nil {
		children := m.labelChildren[*parentID]
		newChildren := make([]*label.Label, 0)
		for _, child := range children {
			if child.ID() != id {
				newChildren = append(newChildren, child)
			}
		}
		m.labelChildren[*parentID] = newChildren
	}

	// Remove task associations
	delete(m.labelTasks, id)
	for taskID, labelIDs := range m.taskLabels {
		newLabelIDs := make([]int, 0)
		for _, labelID := range labelIDs {
			if labelID != id {
				newLabelIDs = append(newLabelIDs, labelID)
			}
		}
		m.taskLabels[taskID] = newLabelIDs
	}

	delete(m.labels, id)
	return nil
}

// AttachToTask attaches a label to a task
func (m *MockLabelRepository) AttachToTask(ctx context.Context, taskID string, labelID int, position int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.labels[labelID]; !exists {
		return ErrLabelNotFound
	}

	// Check if already attached
	for i, id := range m.taskLabels[taskID] {
		if id == labelID {
			// Update position by removing and re-inserting
			m.taskLabels[taskID] = append(m.taskLabels[taskID][:i], m.taskLabels[taskID][i+1:]...)
			break
		}
	}

	// Insert at position
	if position >= len(m.taskLabels[taskID]) {
		m.taskLabels[taskID] = append(m.taskLabels[taskID], labelID)
	} else {
		m.taskLabels[taskID] = append(m.taskLabels[taskID][:position], append([]int{labelID}, m.taskLabels[taskID][position:]...)...)
	}

	// Update reverse mapping
	found := false
	for _, tID := range m.labelTasks[labelID] {
		if tID == taskID {
			found = true
			break
		}
	}
	if !found {
		m.labelTasks[labelID] = append(m.labelTasks[labelID], taskID)
	}

	return nil
}

// DetachFromTask removes a label from a task
func (m *MockLabelRepository) DetachFromTask(ctx context.Context, taskID string, labelID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove from task -> labels mapping
	labelIDs := m.taskLabels[taskID]
	newLabelIDs := make([]int, 0)
	for _, id := range labelIDs {
		if id != labelID {
			newLabelIDs = append(newLabelIDs, id)
		}
	}
	m.taskLabels[taskID] = newLabelIDs

	// Remove from label -> tasks mapping
	taskIDs := m.labelTasks[labelID]
	newTaskIDs := make([]string, 0)
	for _, id := range taskIDs {
		if id != taskID {
			newTaskIDs = append(newTaskIDs, id)
		}
	}
	m.labelTasks[labelID] = newTaskIDs

	return nil
}

// FindLabelsByTaskID retrieves labels for a task
func (m *MockLabelRepository) FindLabelsByTaskID(ctx context.Context, taskID string) ([]*label.Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	labelIDs := m.taskLabels[taskID]
	result := make([]*label.Label, 0, len(labelIDs))
	for _, id := range labelIDs {
		if lbl, exists := m.labels[id]; exists {
			result = append(result, lbl)
		}
	}
	return result, nil
}

// FindTaskIDsByLabelID retrieves task IDs for a label
func (m *MockLabelRepository) FindTaskIDsByLabelID(ctx context.Context, labelID int) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.labels[labelID]; !exists {
		return nil, ErrLabelNotFound
	}

	return m.labelTasks[labelID], nil
}

// ValidateIntegrity validates a label's template integrity
func (m *MockLabelRepository) ValidateIntegrity(ctx context.Context, labelID int) (*repository.ValidationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lbl, exists := m.labels[labelID]
	if !exists {
		return nil, ErrLabelNotFound
	}

	// For mock, just return OK status
	return &repository.ValidationResult{
		LabelID:   labelID,
		LabelName: lbl.Name(),
		Status:    repository.ValidationOK,
	}, nil
}

// ValidateAllLabels validates all labels
func (m *MockLabelRepository) ValidateAllLabels(ctx context.Context) ([]*repository.ValidationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*repository.ValidationResult, 0, len(m.labels))
	for id, lbl := range m.labels {
		result = append(result, &repository.ValidationResult{
			LabelID:   id,
			LabelName: lbl.Name(),
			Status:    repository.ValidationOK,
		})
	}
	return result, nil
}

// SyncFromFile syncs a label from file
func (m *MockLabelRepository) SyncFromFile(ctx context.Context, labelID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.labels[labelID]; !exists {
		return ErrLabelNotFound
	}

	// For mock, just return success
	return nil
}

// FindChildren retrieves child labels
func (m *MockLabelRepository) FindChildren(ctx context.Context, parentID int) ([]*label.Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.labels[parentID]; !exists {
		return nil, ErrLabelNotFound
	}

	return m.labelChildren[parentID], nil
}

// FindByParentID retrieves labels by parent ID
func (m *MockLabelRepository) FindByParentID(ctx context.Context, parentID *int) ([]*label.Label, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*label.Label, 0)
	for _, lbl := range m.labels {
		if parentID == nil && lbl.ParentLabelID() == nil {
			result = append(result, lbl)
		} else if parentID != nil && lbl.ParentLabelID() != nil && *lbl.ParentLabelID() == *parentID {
			result = append(result, lbl)
		}
	}
	return result, nil
}

// ErrLabelNotFound is returned when a label is not found
var ErrLabelNotFound = errors.New("label not found")

// Test Suite for LabelRepository

func TestLabelRepository_Save(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("backend", "Backend tasks", []string{"templates/backend.md"}, 1)

	err := repo.Save(ctx, lbl)
	if err != nil {
		t.Fatalf("Failed to save label: %v", err)
	}

	if lbl.ID() == 0 {
		t.Error("Expected label ID to be assigned")
	}

	// Verify label was saved
	found, err := repo.FindByID(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to find saved label: %v", err)
	}

	if found.Name() != "backend" {
		t.Errorf("Expected name 'backend', got '%s'", found.Name())
	}
}

func TestLabelRepository_SaveNilLabel(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	err := repo.Save(ctx, nil)
	if err == nil {
		t.Error("Expected error when saving nil label")
	}
}

func TestLabelRepository_FindByID(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("frontend", "Frontend tasks", []string{}, 2)
	repo.Save(ctx, lbl)

	found, err := repo.FindByID(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to find label: %v", err)
	}

	if found.Name() != "frontend" {
		t.Errorf("Expected name 'frontend', got '%s'", found.Name())
	}

	if found.Priority() != 2 {
		t.Errorf("Expected priority 2, got %d", found.Priority())
	}
}

func TestLabelRepository_FindByIDNotFound(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	_, err := repo.FindByID(ctx, 999)
	if err == nil {
		t.Error("Expected error when finding non-existent label")
	}

	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("Expected ErrLabelNotFound, got %v", err)
	}
}

func TestLabelRepository_FindByName(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("database", "Database tasks", []string{}, 3)
	repo.Save(ctx, lbl)

	found, err := repo.FindByName(ctx, "database")
	if err != nil {
		t.Fatalf("Failed to find label by name: %v", err)
	}

	if found.ID() != lbl.ID() {
		t.Errorf("Expected ID %d, got %d", lbl.ID(), found.ID())
	}
}

func TestLabelRepository_FindByNameNotFound(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	_, err := repo.FindByName(ctx, "nonexistent")
	if err == nil {
		t.Error("Expected error when finding non-existent label by name")
	}

	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("Expected ErrLabelNotFound, got %v", err)
	}
}

func TestLabelRepository_FindAll(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create multiple labels
	labels := []string{"backend", "frontend", "database", "testing"}
	for _, name := range labels {
		lbl := label.NewLabel(name, name+" tasks", []string{}, 1)
		repo.Save(ctx, lbl)
	}

	// Find all labels
	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("Failed to find all labels: %v", err)
	}

	if len(all) != 4 {
		t.Errorf("Expected 4 labels, got %d", len(all))
	}
}

func TestLabelRepository_FindAllEmpty(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("Failed to find all labels: %v", err)
	}

	if len(all) != 0 {
		t.Errorf("Expected 0 labels from empty repository, got %d", len(all))
	}
}

func TestLabelRepository_FindActive(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create labels with different active states
	lbl1 := label.NewLabel("active1", "Active label 1", []string{}, 1)
	lbl2 := label.NewLabel("active2", "Active label 2", []string{}, 1)
	lbl3 := label.NewLabel("inactive", "Inactive label", []string{}, 1)

	repo.Save(ctx, lbl1)
	repo.Save(ctx, lbl2)
	repo.Save(ctx, lbl3)

	// Deactivate one label
	lbl3.Deactivate()
	repo.Update(ctx, lbl3)

	// Find active labels
	active, err := repo.FindActive(ctx)
	if err != nil {
		t.Fatalf("Failed to find active labels: %v", err)
	}

	if len(active) != 2 {
		t.Errorf("Expected 2 active labels, got %d", len(active))
	}

	for _, lbl := range active {
		if !lbl.IsActive() {
			t.Error("Found inactive label in active results")
		}
	}
}

func TestLabelRepository_Update(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("original", "Original description", []string{}, 1)
	repo.Save(ctx, lbl)

	// Update label
	lbl.SetDescription("Updated description")
	lbl.SetPriority(5)
	lbl.SetColor("#FF0000")

	err := repo.Update(ctx, lbl)
	if err != nil {
		t.Fatalf("Failed to update label: %v", err)
	}

	// Verify updates
	found, err := repo.FindByID(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to find label: %v", err)
	}

	if found.Description() != "Updated description" {
		t.Errorf("Expected description 'Updated description', got '%s'", found.Description())
	}

	if found.Priority() != 5 {
		t.Errorf("Expected priority 5, got %d", found.Priority())
	}

	if found.Color() != "#FF0000" {
		t.Errorf("Expected color '#FF0000', got '%s'", found.Color())
	}
}

func TestLabelRepository_UpdateNotFound(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("test", "Test", []string{}, 1)
	lbl.SetID(999) // Non-existent ID

	err := repo.Update(ctx, lbl)
	if err == nil {
		t.Error("Expected error when updating non-existent label")
	}

	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("Expected ErrLabelNotFound, got %v", err)
	}
}

func TestLabelRepository_Delete(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("delete-me", "To be deleted", []string{}, 1)
	repo.Save(ctx, lbl)

	err := repo.Delete(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to delete label: %v", err)
	}

	// Verify deletion
	_, err = repo.FindByID(ctx, lbl.ID())
	if err == nil {
		t.Error("Expected error when finding deleted label")
	}

	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("Expected ErrLabelNotFound, got %v", err)
	}
}

func TestLabelRepository_DeleteNotFound(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	err := repo.Delete(ctx, 999)
	if err == nil {
		t.Error("Expected error when deleting non-existent label")
	}

	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("Expected ErrLabelNotFound, got %v", err)
	}
}

func TestLabelRepository_AttachToTask(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("urgent", "Urgent tasks", []string{}, 10)
	repo.Save(ctx, lbl)

	taskID := "task-001"
	err := repo.AttachToTask(ctx, taskID, lbl.ID(), 0)
	if err != nil {
		t.Fatalf("Failed to attach label to task: %v", err)
	}

	// Verify attachment
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to find labels by task ID: %v", err)
	}

	if len(labels) != 1 {
		t.Errorf("Expected 1 label, got %d", len(labels))
	}

	if labels[0].ID() != lbl.ID() {
		t.Errorf("Expected label ID %d, got %d", lbl.ID(), labels[0].ID())
	}
}

func TestLabelRepository_AttachMultipleLabels(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create multiple labels
	lbl1 := label.NewLabel("label1", "Label 1", []string{}, 1)
	lbl2 := label.NewLabel("label2", "Label 2", []string{}, 2)
	lbl3 := label.NewLabel("label3", "Label 3", []string{}, 3)

	repo.Save(ctx, lbl1)
	repo.Save(ctx, lbl2)
	repo.Save(ctx, lbl3)

	taskID := "task-multi"

	// Attach in order
	repo.AttachToTask(ctx, taskID, lbl1.ID(), 0)
	repo.AttachToTask(ctx, taskID, lbl2.ID(), 1)
	repo.AttachToTask(ctx, taskID, lbl3.ID(), 2)

	// Verify order
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to find labels: %v", err)
	}

	if len(labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(labels))
	}

	expectedOrder := []int{lbl1.ID(), lbl2.ID(), lbl3.ID()}
	for i, lbl := range labels {
		if lbl.ID() != expectedOrder[i] {
			t.Errorf("Position %d: expected ID %d, got %d", i, expectedOrder[i], lbl.ID())
		}
	}
}

func TestLabelRepository_DetachFromTask(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("detach-me", "To be detached", []string{}, 1)
	repo.Save(ctx, lbl)

	taskID := "task-detach"
	repo.AttachToTask(ctx, taskID, lbl.ID(), 0)

	// Detach
	err := repo.DetachFromTask(ctx, taskID, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to detach label: %v", err)
	}

	// Verify detachment
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to find labels: %v", err)
	}

	if len(labels) != 0 {
		t.Errorf("Expected 0 labels after detachment, got %d", len(labels))
	}
}

func TestLabelRepository_FindTaskIDsByLabelID(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("popular", "Popular label", []string{}, 1)
	repo.Save(ctx, lbl)

	// Attach to multiple tasks
	taskIDs := []string{"task-001", "task-002", "task-003"}
	for _, taskID := range taskIDs {
		repo.AttachToTask(ctx, taskID, lbl.ID(), 0)
	}

	// Find tasks
	foundTasks, err := repo.FindTaskIDsByLabelID(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to find task IDs: %v", err)
	}

	if len(foundTasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(foundTasks))
	}
}

func TestLabelRepository_FindTaskIDsByLabelIDNotFound(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	_, err := repo.FindTaskIDsByLabelID(ctx, 999)
	if err == nil {
		t.Error("Expected error when finding tasks for non-existent label")
	}

	if !errors.Is(err, ErrLabelNotFound) {
		t.Errorf("Expected ErrLabelNotFound, got %v", err)
	}
}

func TestLabelRepository_ValidateIntegrity(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("validate", "Validate me", []string{"template.md"}, 1)
	repo.Save(ctx, lbl)

	result, err := repo.ValidateIntegrity(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to validate integrity: %v", err)
	}

	if result.LabelID != lbl.ID() {
		t.Errorf("Expected label ID %d, got %d", lbl.ID(), result.LabelID)
	}

	if result.Status != repository.ValidationOK {
		t.Errorf("Expected status OK, got %s", result.Status)
	}
}

func TestLabelRepository_ValidateAllLabels(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create multiple labels
	for i := 0; i < 5; i++ {
		lbl := label.NewLabel("label"+string(rune('A'+i)), "Description", []string{}, 1)
		repo.Save(ctx, lbl)
	}

	results, err := repo.ValidateAllLabels(ctx)
	if err != nil {
		t.Fatalf("Failed to validate all labels: %v", err)
	}

	if len(results) != 5 {
		t.Errorf("Expected 5 validation results, got %d", len(results))
	}

	for _, result := range results {
		if result.Status != repository.ValidationOK {
			t.Errorf("Expected status OK for label %d, got %s", result.LabelID, result.Status)
		}
	}
}

func TestLabelRepository_SyncFromFile(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("sync", "Sync me", []string{"template.md"}, 1)
	repo.Save(ctx, lbl)

	err := repo.SyncFromFile(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to sync from file: %v", err)
	}
}

func TestLabelRepository_FindChildren(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create parent label
	parent := label.NewLabel("parent", "Parent label", []string{}, 1)
	repo.Save(ctx, parent)

	// Create child labels
	child1 := label.NewLabel("child1", "Child 1", []string{}, 1)
	parentID := parent.ID()
	child1.SetParentLabelID(&parentID)
	repo.Save(ctx, child1)

	child2 := label.NewLabel("child2", "Child 2", []string{}, 1)
	child2.SetParentLabelID(&parentID)
	repo.Save(ctx, child2)

	// Find children
	children, err := repo.FindChildren(ctx, parent.ID())
	if err != nil {
		t.Fatalf("Failed to find children: %v", err)
	}

	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}
}

func TestLabelRepository_FindByParentID(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create parent and children
	parent := label.NewLabel("parent", "Parent", []string{}, 1)
	repo.Save(ctx, parent)

	parentID := parent.ID()
	for i := 0; i < 3; i++ {
		child := label.NewLabel("child"+string(rune('A'+i)), "Child", []string{}, 1)
		child.SetParentLabelID(&parentID)
		repo.Save(ctx, child)
	}

	// Find by parent ID
	children, err := repo.FindByParentID(ctx, &parentID)
	if err != nil {
		t.Fatalf("Failed to find by parent ID: %v", err)
	}

	if len(children) != 3 {
		t.Errorf("Expected 3 children, got %d", len(children))
	}

	for _, child := range children {
		if child.ParentLabelID() == nil || *child.ParentLabelID() != parentID {
			t.Error("Child has incorrect parent ID")
		}
	}
}

func TestLabelRepository_FindByParentIDNull(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create root labels (no parent)
	for i := 0; i < 3; i++ {
		lbl := label.NewLabel("root"+string(rune('A'+i)), "Root", []string{}, 1)
		repo.Save(ctx, lbl)
	}

	// Create one child label
	parent := label.NewLabel("parent", "Parent", []string{}, 1)
	repo.Save(ctx, parent)

	child := label.NewLabel("child", "Child", []string{}, 1)
	parentID := parent.ID()
	child.SetParentLabelID(&parentID)
	repo.Save(ctx, child)

	// Find root labels (parentID = nil)
	roots, err := repo.FindByParentID(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to find root labels: %v", err)
	}

	if len(roots) != 4 {
		t.Errorf("Expected 4 root labels (including parent), got %d", len(roots))
	}

	for _, root := range roots {
		if root.ParentLabelID() != nil {
			t.Errorf("Root label should have nil parent, got %d", *root.ParentLabelID())
		}
	}
}

func TestLabelRepository_Concurrency(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	errorChan := make(chan error, 20)

	// Concurrent saves
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			lbl := label.NewLabel("label"+string(rune('A'+index)), "Description", []string{}, index)
			err := repo.Save(ctx, lbl)
			if err != nil {
				errorChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent save failed: %v", err)
	}

	// Verify all labels were saved
	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("Failed to find all labels: %v", err)
	}

	if len(all) != 20 {
		t.Errorf("Expected 20 labels, got %d", len(all))
	}
}

func TestLabelRepository_ConcurrentReadsAndWrites(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 5; i++ {
		lbl := label.NewLabel("label"+string(rune('A'+i)), "Description", []string{}, i)
		repo.Save(ctx, lbl)
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 50)

	// Concurrent reads
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := repo.FindAll(ctx)
			if err != nil {
				errorChan <- err
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			lbl := label.NewLabel("new"+string(rune('A'+index)), "New", []string{}, index)
			err := repo.Save(ctx, lbl)
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
	all, err := repo.FindAll(ctx)
	if err != nil {
		t.Fatalf("Failed to find all labels: %v", err)
	}

	if len(all) != 30 {
		t.Errorf("Expected 30 labels, got %d", len(all))
	}
}

func TestLabelRepository_ActivateDeactivate(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("toggle", "Toggle me", []string{}, 1)
	repo.Save(ctx, lbl)

	// Verify initially active
	if !lbl.IsActive() {
		t.Error("Expected label to be active initially")
	}

	// Deactivate
	lbl.Deactivate()
	repo.Update(ctx, lbl)

	found, _ := repo.FindByID(ctx, lbl.ID())
	if found.IsActive() {
		t.Error("Expected label to be inactive after deactivation")
	}

	// Reactivate
	lbl.Activate()
	repo.Update(ctx, lbl)

	found, _ = repo.FindByID(ctx, lbl.ID())
	if !found.IsActive() {
		t.Error("Expected label to be active after reactivation")
	}
}

func TestLabelRepository_TemplatePaths(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	paths := []string{"templates/a.md", "templates/b.md", "templates/c.md"}
	lbl := label.NewLabel("multi-template", "Multiple templates", paths, 1)
	repo.Save(ctx, lbl)

	found, err := repo.FindByID(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to find label: %v", err)
	}

	foundPaths := found.TemplatePaths()
	if len(foundPaths) != 3 {
		t.Errorf("Expected 3 template paths, got %d", len(foundPaths))
	}
}

func TestLabelRepository_DeleteWithChildren(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	// Create parent with children
	parent := label.NewLabel("parent", "Parent", []string{}, 1)
	repo.Save(ctx, parent)

	parentID := parent.ID()
	child := label.NewLabel("child", "Child", []string{}, 1)
	child.SetParentLabelID(&parentID)
	repo.Save(ctx, child)

	// Delete parent
	err := repo.Delete(ctx, parent.ID())
	if err != nil {
		t.Fatalf("Failed to delete parent: %v", err)
	}

	// Verify parent is deleted
	_, err = repo.FindByID(ctx, parent.ID())
	if err == nil {
		t.Error("Expected error when finding deleted parent")
	}
}

func TestLabelRepository_DeleteWithTaskAssociations(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("associated", "Has tasks", []string{}, 1)
	repo.Save(ctx, lbl)

	// Attach to tasks
	repo.AttachToTask(ctx, "task-001", lbl.ID(), 0)
	repo.AttachToTask(ctx, "task-002", lbl.ID(), 0)

	// Delete label
	err := repo.Delete(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to delete label: %v", err)
	}

	// Verify associations are cleaned up
	labels, err := repo.FindLabelsByTaskID(ctx, "task-001")
	if err != nil {
		t.Fatalf("Failed to find labels for task: %v", err)
	}

	if len(labels) != 0 {
		t.Errorf("Expected 0 labels for task after label deletion, got %d", len(labels))
	}
}

func TestLabelRepository_UpdatePosition(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl1 := label.NewLabel("label1", "Label 1", []string{}, 1)
	lbl2 := label.NewLabel("label2", "Label 2", []string{}, 2)

	repo.Save(ctx, lbl1)
	repo.Save(ctx, lbl2)

	taskID := "task-position"

	// Attach labels
	repo.AttachToTask(ctx, taskID, lbl1.ID(), 0)
	repo.AttachToTask(ctx, taskID, lbl2.ID(), 1)

	// Re-attach label1 at position 1 (should move it)
	repo.AttachToTask(ctx, taskID, lbl1.ID(), 1)

	// Verify new order
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	if err != nil {
		t.Fatalf("Failed to find labels: %v", err)
	}

	if len(labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(labels))
	}

	// lbl1 should now be at the end
	if labels[1].ID() != lbl1.ID() {
		t.Errorf("Expected label1 at position 1, got label ID %d", labels[1].ID())
	}
}

func TestLabelRepository_EmptyTemplatePaths(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("no-templates", "No templates", []string{}, 1)
	repo.Save(ctx, lbl)

	found, err := repo.FindByID(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to find label: %v", err)
	}

	if len(found.TemplatePaths()) != 0 {
		t.Errorf("Expected 0 template paths, got %d", len(found.TemplatePaths()))
	}
}

func TestLabelRepository_ContentHashes(t *testing.T) {
	repo := NewMockLabelRepository()
	ctx := context.Background()

	lbl := label.NewLabel("hashed", "With hashes", []string{"template.md"}, 1)
	lbl.SetContentHash("template.md", "abc123def456")
	repo.Save(ctx, lbl)

	found, err := repo.FindByID(ctx, lbl.ID())
	if err != nil {
		t.Fatalf("Failed to find label: %v", err)
	}

	hash, exists := found.GetContentHash("template.md")
	if !exists {
		t.Error("Expected content hash to exist")
	}

	if hash != "abc123def456" {
		t.Errorf("Expected hash 'abc123def456', got '%s'", hash)
	}
}
