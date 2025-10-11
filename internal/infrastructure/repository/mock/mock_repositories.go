package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockTaskRepository is a mock implementation of TaskRepository
type MockTaskRepository struct {
	mu    sync.RWMutex
	tasks map[string]task.Task
}

// NewMockTaskRepository creates a new mock task repository
func NewMockTaskRepository() *MockTaskRepository {
	return &MockTaskRepository{
		tasks: make(map[string]task.Task),
	}
}

func (m *MockTaskRepository) FindByID(ctx context.Context, id repository.TaskID) (task.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, exists := m.tasks[string(id)]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return t, nil
}

func (m *MockTaskRepository) Save(ctx context.Context, t task.Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks[t.ID().String()] = t
	return nil
}

func (m *MockTaskRepository) Delete(ctx context.Context, id repository.TaskID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tasks, string(id))
	return nil
}

func (m *MockTaskRepository) List(ctx context.Context, filter repository.TaskFilter) ([]task.Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []task.Task
	for _, t := range m.tasks {
		if m.matchesFilter(t, filter) {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *MockTaskRepository) matchesFilter(t task.Task, filter repository.TaskFilter) bool {
	// Filter by types
	if len(filter.Types) > 0 {
		found := false
		for _, filterType := range filter.Types {
			if string(t.Type()) == string(filterType) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by statuses
	if len(filter.Statuses) > 0 {
		found := false
		for _, filterStatus := range filter.Statuses {
			if string(t.Status()) == string(filterStatus) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// MockEPICRepository is a mock implementation of EPICRepository
type MockEPICRepository struct {
	mu    sync.RWMutex
	epics map[string]*epic.EPIC
}

// NewMockEPICRepository creates a new mock EPIC repository
func NewMockEPICRepository() *MockEPICRepository {
	return &MockEPICRepository{
		epics: make(map[string]*epic.EPIC),
	}
}

func (m *MockEPICRepository) Find(ctx context.Context, id repository.EPICID) (*epic.EPIC, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, exists := m.epics[string(id)]
	if !exists {
		return nil, fmt.Errorf("EPIC not found: %s", id)
	}
	return e, nil
}

func (m *MockEPICRepository) Save(ctx context.Context, e *epic.EPIC) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.epics[e.ID().String()] = e
	return nil
}

func (m *MockEPICRepository) Delete(ctx context.Context, id repository.EPICID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.epics, string(id))
	return nil
}

func (m *MockEPICRepository) List(ctx context.Context, filter repository.EPICFilter) ([]*epic.EPIC, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*epic.EPIC
	for _, e := range m.epics {
		result = append(result, e)
	}
	return result, nil
}

func (m *MockEPICRepository) FindByPBIID(ctx context.Context, pbiID repository.PBIID) (*epic.EPIC, error) {
	// Mock implementation: would need to track PBI->EPIC relationships
	// For now, return error
	return nil, fmt.Errorf("not implemented in mock")
}

// MockPBIRepository is a mock implementation of PBIRepository
type MockPBIRepository struct {
	mu   sync.RWMutex
	pbis map[string]*pbi.PBI
}

// NewMockPBIRepository creates a new mock PBI repository
func NewMockPBIRepository() *MockPBIRepository {
	return &MockPBIRepository{
		pbis: make(map[string]*pbi.PBI),
	}
}

func (m *MockPBIRepository) Find(ctx context.Context, id repository.PBIID) (*pbi.PBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.pbis[string(id)]
	if !exists {
		return nil, fmt.Errorf("PBI not found: %s", id)
	}
	return p, nil
}

func (m *MockPBIRepository) Save(ctx context.Context, p *pbi.PBI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.pbis[p.ID] = p
	return nil
}

func (m *MockPBIRepository) Delete(ctx context.Context, id repository.PBIID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.pbis, string(id))
	return nil
}

func (m *MockPBIRepository) List(ctx context.Context, filter repository.PBIFilter) ([]*pbi.PBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*pbi.PBI
	for _, p := range m.pbis {
		result = append(result, p)
	}
	return result, nil
}

func (m *MockPBIRepository) FindByEPICID(ctx context.Context, epicID repository.EPICID) ([]*pbi.PBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*pbi.PBI
	for _, p := range m.pbis {
		// Check if PBI belongs to the EPIC
		// This would need access to PBI's parent EPIC ID field
		result = append(result, p)
	}
	return result, nil
}

func (m *MockPBIRepository) FindBySBIID(ctx context.Context, sbiID repository.SBIID) (*pbi.PBI, error) {
	// Mock implementation: would need to track SBI->PBI relationships
	// For now, return error
	return nil, fmt.Errorf("not implemented in mock")
}

// MockSBIRepository is a mock implementation of SBIRepository
type MockSBIRepository struct {
	mu   sync.RWMutex
	sbis map[string]*sbi.SBI
}

// NewMockSBIRepository creates a new mock SBI repository
func NewMockSBIRepository() *MockSBIRepository {
	return &MockSBIRepository{
		sbis: make(map[string]*sbi.SBI),
	}
}

func (m *MockSBIRepository) Find(ctx context.Context, id repository.SBIID) (*sbi.SBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, exists := m.sbis[string(id)]
	if !exists {
		return nil, fmt.Errorf("SBI not found: %s", id)
	}
	return s, nil
}

func (m *MockSBIRepository) Save(ctx context.Context, s *sbi.SBI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sbis[s.ID().String()] = s
	return nil
}

func (m *MockSBIRepository) Delete(ctx context.Context, id repository.SBIID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sbis, string(id))
	return nil
}

func (m *MockSBIRepository) List(ctx context.Context, filter repository.SBIFilter) ([]*sbi.SBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*sbi.SBI
	for _, s := range m.sbis {
		result = append(result, s)
	}
	return result, nil
}

func (m *MockSBIRepository) FindByPBIID(ctx context.Context, pbiID repository.PBIID) ([]*sbi.SBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*sbi.SBI
	for _, s := range m.sbis {
		// Check if SBI belongs to the PBI
		// This would need access to SBI's parent PBI ID field
		result = append(result, s)
	}
	return result, nil
}
