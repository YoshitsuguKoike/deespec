package repository_test

import (
	"context"
	"sync"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockTaskRepository is a mock implementation of TaskRepository for testing
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

// FindByID retrieves a task by ID
func (m *MockTaskRepository) FindByID(ctx context.Context, id repository.TaskID) (task.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return t, nil
}

// Save persists a task
func (m *MockTaskRepository) Save(ctx context.Context, t task.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[repository.TaskID(t.ID().String())] = t
	return nil
}

// Delete removes a task
func (m *MockTaskRepository) Delete(ctx context.Context, id repository.TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tasks, id)
	return nil
}

// List retrieves tasks by filter
func (m *MockTaskRepository) List(ctx context.Context, filter repository.TaskFilter) ([]task.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []task.Task
	for _, t := range m.tasks {
		if m.matchesFilter(t, filter) {
			result = append(result, t)
		}
	}

	// Apply limit and offset
	if filter.Offset < len(result) {
		result = result[filter.Offset:]
	} else {
		result = nil
	}

	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (m *MockTaskRepository) matchesFilter(t task.Task, filter repository.TaskFilter) bool {
	// Type filter
	if len(filter.Types) > 0 {
		matched := false
		for _, taskType := range filter.Types {
			if repository.TaskType(t.Type().String()) == taskType {
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
		for _, s := range filter.Statuses {
			if repository.Status(t.Status().String()) == s {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Parent filter
	if filter.ParentID != nil {
		parentID := t.ParentTaskID()
		if parentID == nil || repository.TaskID(parentID.String()) != *filter.ParentID {
			return false
		}
	}

	if filter.HasParent != nil {
		hasParent := t.ParentTaskID() != nil
		if hasParent != *filter.HasParent {
			return false
		}
	}

	return true
}

// ErrTaskNotFound is returned when a task is not found
var ErrTaskNotFound = &TaskNotFoundError{}

type TaskNotFoundError struct{}

func (e *TaskNotFoundError) Error() string {
	return "task not found"
}
