package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockSBIRepository is a mock implementation of SBIRepository for testing
type MockSBIRepository struct {
	mu           sync.RWMutex
	sbis         map[repository.SBIID]*sbi.SBI
	pbiMap       map[repository.SBIID]repository.PBIID // Maps SBI IDs to PBI IDs
	nextSequence int
}

// NewMockSBIRepository creates a new mock SBI repository
func NewMockSBIRepository() *MockSBIRepository {
	return &MockSBIRepository{
		sbis:         make(map[repository.SBIID]*sbi.SBI),
		pbiMap:       make(map[repository.SBIID]repository.PBIID),
		nextSequence: 1,
	}
}

// Find retrieves an SBI by its ID
func (m *MockSBIRepository) Find(ctx context.Context, id repository.SBIID) (*sbi.SBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, exists := m.sbis[id]
	if !exists {
		return nil, ErrSBINotFound
	}
	return s, nil
}

// Save persists an SBI entity
func (m *MockSBIRepository) Save(ctx context.Context, s *sbi.SBI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s == nil {
		return errors.New("sbi cannot be nil")
	}

	id := repository.SBIID(s.ID().String())
	m.sbis[id] = s

	// Update PBI mapping if SBI has a parent PBI
	if s.HasParentPBI() {
		m.pbiMap[id] = repository.PBIID(s.ParentTaskID().String())
	} else {
		delete(m.pbiMap, id)
	}

	return nil
}

// Delete removes an SBI
func (m *MockSBIRepository) Delete(ctx context.Context, id repository.SBIID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sbis[id]
	if !exists {
		return ErrSBINotFound
	}

	// Check if SBI can be deleted
	if !s.CanDelete() {
		return errors.New("cannot delete SBI that is currently being executed")
	}

	delete(m.sbis, id)
	delete(m.pbiMap, id)
	return nil
}

// List retrieves SBIs by filter
func (m *MockSBIRepository) List(ctx context.Context, filter repository.SBIFilter) ([]*sbi.SBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*sbi.SBI
	for _, s := range m.sbis {
		if m.matchesFilter(s, filter) {
			result = append(result, s)
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

// FindByPBIID retrieves all SBIs belonging to a PBI
func (m *MockSBIRepository) FindByPBIID(ctx context.Context, pbiID repository.PBIID) ([]*sbi.SBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*sbi.SBI
	for sbiID, pid := range m.pbiMap {
		if pid == pbiID {
			if s, exists := m.sbis[sbiID]; exists {
				result = append(result, s)
			}
		}
	}

	return result, nil
}

// GetNextSequence retrieves the next available sequence number
func (m *MockSBIRepository) GetNextSequence(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	seq := m.nextSequence
	m.nextSequence++
	return seq, nil
}

// ResetSBIState resets an SBI to a specific status (for testing/maintenance)
func (m *MockSBIRepository) ResetSBIState(ctx context.Context, id repository.SBIID, toStatus string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, exists := m.sbis[id]
	if !exists {
		return ErrSBINotFound
	}

	// Parse status string and update
	var status model.Status
	switch toStatus {
	case "PENDING":
		status = model.StatusPending
	case "PICKED":
		status = model.StatusPicked
	case "IMPLEMENTING":
		status = model.StatusImplementing
	case "REVIEWING":
		status = model.StatusReviewing
	case "DONE":
		status = model.StatusDone
	case "FAILED":
		status = model.StatusFailed
	default:
		return errors.New("invalid status: " + toStatus)
	}

	return s.UpdateStatus(status)
}

func (m *MockSBIRepository) matchesFilter(s *sbi.SBI, filter repository.SBIFilter) bool {
	// PBI ID filter
	if filter.PBIID != nil {
		sbiID := repository.SBIID(s.ID().String())
		pbiID, exists := m.pbiMap[sbiID]
		if !exists || pbiID != *filter.PBIID {
			return false
		}
	}

	// Labels filter
	if len(filter.Labels) > 0 {
		sbiLabels := s.Metadata().Labels
		matched := false
		for _, filterLabel := range filter.Labels {
			for _, sbiLabel := range sbiLabels {
				if sbiLabel == filterLabel {
					matched = true
					break
				}
			}
			if matched {
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
		for _, status := range filter.Statuses {
			if s.Status() == status {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// ErrSBINotFound is returned when an SBI is not found
var ErrSBINotFound = errors.New("sbi not found")

// Test Suite for SBIRepository

func TestSBIRepository_Find(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create and save an SBI
	s, err := sbi.NewSBI("Test SBI", "Description", nil, sbi.SBIMetadata{
		EstimatedHours: 4.0,
		Priority:       1,
		Labels:         []string{"backend"},
		AssignedAgent:  "claude-code",
	})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Test finding the SBI
	found, err := repo.Find(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to find SBI: %v", err)
	}

	if found.ID().String() != s.ID().String() {
		t.Errorf("Expected SBI ID %s, got %s", s.ID().String(), found.ID().String())
	}

	if found.Title() != "Test SBI" {
		t.Errorf("Expected title 'Test SBI', got '%s'", found.Title())
	}
}

func TestSBIRepository_FindNotFound(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Try to find non-existent SBI
	_, err := repo.Find(ctx, repository.SBIID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when finding non-existent SBI")
	}

	if !errors.Is(err, ErrSBINotFound) {
		t.Errorf("Expected ErrSBINotFound, got %v", err)
	}
}

func TestSBIRepository_Save(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	tests := []struct {
		name        string
		title       string
		description string
		metadata    sbi.SBIMetadata
	}{
		{
			name:        "Basic SBI",
			title:       "Feature Implementation",
			description: "Implement feature X",
			metadata: sbi.SBIMetadata{
				EstimatedHours: 2.0,
				Priority:       0,
			},
		},
		{
			name:        "SBI with labels and agent",
			title:       "Bug Fix",
			description: "Fix bug in authentication",
			metadata: sbi.SBIMetadata{
				EstimatedHours: 1.5,
				Priority:       2,
				Labels:         []string{"bug", "security"},
				AssignedAgent:  "gemini-cli",
			},
		},
		{
			name:        "SBI with file paths",
			title:       "Refactoring",
			description: "Refactor database layer",
			metadata: sbi.SBIMetadata{
				EstimatedHours: 5.0,
				Priority:       1,
				FilePaths:      []string{"db/connection.go", "db/repository.go"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := sbi.NewSBI(tt.title, tt.description, nil, tt.metadata)
			if err != nil {
				t.Fatalf("Failed to create SBI: %v", err)
			}

			err = repo.Save(ctx, s)
			if err != nil {
				t.Fatalf("Failed to save SBI: %v", err)
			}

			// Verify saved SBI
			sbiID := repository.SBIID(s.ID().String())
			found, err := repo.Find(ctx, sbiID)
			if err != nil {
				t.Fatalf("Failed to find saved SBI: %v", err)
			}

			if found.Title() != tt.title {
				t.Errorf("Expected title '%s', got '%s'", tt.title, found.Title())
			}

			if found.Description() != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, found.Description())
			}
		})
	}
}

func TestSBIRepository_SaveNil(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	err := repo.Save(ctx, nil)
	if err == nil {
		t.Error("Expected error when saving nil SBI")
	}
}

func TestSBIRepository_SaveWithParentPBI(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBI with parent PBI
	pbiID := model.NewTaskID()
	s, err := sbi.NewSBI("Test SBI", "Description", &pbiID, sbi.SBIMetadata{
		EstimatedHours: 3.0,
		Priority:       1,
	})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Verify SBI has parent PBI
	sbiID := repository.SBIID(s.ID().String())
	found, err := repo.Find(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to find SBI: %v", err)
	}

	if !found.HasParentPBI() {
		t.Error("Expected SBI to have parent PBI")
	}

	if found.ParentTaskID().String() != pbiID.String() {
		t.Errorf("Expected parent PBI ID %s, got %s", pbiID.String(), found.ParentTaskID().String())
	}
}

func TestSBIRepository_Delete(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create and save an SBI
	s, err := sbi.NewSBI("Delete Me", "To be deleted", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Delete the SBI
	err = repo.Delete(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to delete SBI: %v", err)
	}

	// Verify SBI is deleted
	_, err = repo.Find(ctx, sbiID)
	if err == nil {
		t.Error("Expected error when finding deleted SBI")
	}

	if !errors.Is(err, ErrSBINotFound) {
		t.Errorf("Expected ErrSBINotFound, got %v", err)
	}
}

func TestSBIRepository_DeleteImplementing(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBI and set status to IMPLEMENTING
	s, err := sbi.NewSBI("Running SBI", "Currently executing", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	// Transition to IMPLEMENTING
	s.UpdateStatus(model.StatusPicked)
	s.UpdateStatus(model.StatusImplementing)

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Try to delete SBI that is IMPLEMENTING
	err = repo.Delete(ctx, sbiID)
	if err == nil {
		t.Error("Expected error when deleting SBI that is IMPLEMENTING")
	}
}

func TestSBIRepository_DeleteNotFound(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Try to delete non-existent SBI
	err := repo.Delete(ctx, repository.SBIID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when deleting non-existent SBI")
	}

	if !errors.Is(err, ErrSBINotFound) {
		t.Errorf("Expected ErrSBINotFound, got %v", err)
	}
}

func TestSBIRepository_List(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create multiple SBIs with different statuses
	statuses := []model.Status{
		model.StatusPending,
		model.StatusPicked,
		model.StatusImplementing,
	}

	for i, status := range statuses {
		s, err := sbi.NewSBI(
			"SBI "+string(rune('A'+i)),
			"Description",
			nil,
			sbi.SBIMetadata{Priority: i},
		)
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		// Apply status transitions
		if status != model.StatusPending {
			s.UpdateStatus(model.StatusPicked)
		}
		if status == model.StatusImplementing {
			s.UpdateStatus(model.StatusImplementing)
		}

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}
	}

	// Test listing all SBIs
	allSBIs, err := repo.List(ctx, repository.SBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(allSBIs) != 3 {
		t.Errorf("Expected 3 SBIs, got %d", len(allSBIs))
	}
}

func TestSBIRepository_ListWithStatusFilter(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBIs with different statuses
	statusTransitions := [][]model.Status{
		{model.StatusPending},
		{model.StatusPending, model.StatusPicked},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing},
	}

	for i, transitions := range statusTransitions {
		s, err := sbi.NewSBI("SBI "+string(rune('A'+i)), "Description", nil, sbi.SBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		// Apply status transitions
		for j := 1; j < len(transitions); j++ {
			err = s.UpdateStatus(transitions[j])
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}
	}

	// Filter by IMPLEMENTING status
	filter := repository.SBIFilter{
		Statuses: []model.Status{model.StatusImplementing},
	}

	implementingSBIs, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(implementingSBIs) != 2 {
		t.Errorf("Expected 2 IMPLEMENTING SBIs, got %d", len(implementingSBIs))
	}

	for _, s := range implementingSBIs {
		if s.Status() != model.StatusImplementing {
			t.Errorf("Expected status IMPLEMENTING, got %s", s.Status().String())
		}
	}
}

func TestSBIRepository_ListWithLabelFilter(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBIs with different labels
	testSBIs := []struct {
		title  string
		labels []string
	}{
		{"Backend Task", []string{"backend", "api"}},
		{"Frontend Task", []string{"frontend", "ui"}},
		{"Database Task", []string{"backend", "database"}},
	}

	for _, tc := range testSBIs {
		s, err := sbi.NewSBI(tc.title, "Description", nil, sbi.SBIMetadata{
			Labels: tc.labels,
		})
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}
	}

	// Filter by backend label
	filter := repository.SBIFilter{
		Labels: []string{"backend"},
	}

	backendSBIs, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(backendSBIs) != 2 {
		t.Errorf("Expected 2 backend SBIs, got %d", len(backendSBIs))
	}
}

func TestSBIRepository_ListWithPagination(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create 5 SBIs
	for i := 0; i < 5; i++ {
		s, err := sbi.NewSBI("SBI "+string(rune('A'+i)), "Description", nil, sbi.SBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}
	}

	// Test with limit
	filter := repository.SBIFilter{
		Limit: 2,
	}

	sbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 2 {
		t.Errorf("Expected 2 SBIs with limit=2, got %d", len(sbis))
	}

	// Test with offset
	filter = repository.SBIFilter{
		Offset: 3,
	}

	sbis, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 2 {
		t.Errorf("Expected 2 SBIs with offset=3, got %d", len(sbis))
	}

	// Test with both limit and offset
	filter = repository.SBIFilter{
		Limit:  2,
		Offset: 1,
	}

	sbis, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 2 {
		t.Errorf("Expected 2 SBIs with limit=2 and offset=1, got %d", len(sbis))
	}
}

func TestSBIRepository_FindByPBIID(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBIs with same parent PBI
	pbiID := model.NewTaskID()

	for i := 0; i < 3; i++ {
		s, err := sbi.NewSBI("SBI "+string(rune('A'+i)), "Description", &pbiID, sbi.SBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}
	}

	// Find SBIs by PBI ID
	sbis, err := repo.FindByPBIID(ctx, repository.PBIID(pbiID.String()))
	if err != nil {
		t.Fatalf("Failed to find SBIs by PBI ID: %v", err)
	}

	if len(sbis) != 3 {
		t.Errorf("Expected 3 SBIs for PBI, got %d", len(sbis))
	}

	for _, s := range sbis {
		if s.ParentTaskID().String() != pbiID.String() {
			t.Errorf("Expected parent PBI ID %s, got %s", pbiID.String(), s.ParentTaskID().String())
		}
	}
}

func TestSBIRepository_FindByPBIIDNotFound(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Try to find SBIs by non-existent PBI ID
	sbis, err := repo.FindByPBIID(ctx, repository.PBIID("non-existent-pbi"))
	if err != nil {
		t.Fatalf("Expected no error for non-existent PBI, got %v", err)
	}

	if len(sbis) != 0 {
		t.Errorf("Expected 0 SBIs for non-existent PBI, got %d", len(sbis))
	}
}

func TestSBIRepository_GetNextSequence(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Get next sequence multiple times
	sequences := make([]int, 5)
	for i := 0; i < 5; i++ {
		seq, err := repo.GetNextSequence(ctx)
		if err != nil {
			t.Fatalf("Failed to get next sequence: %v", err)
		}
		sequences[i] = seq
	}

	// Verify sequences are sequential
	for i := 0; i < 5; i++ {
		expected := i + 1
		if sequences[i] != expected {
			t.Errorf("Expected sequence %d, got %d", expected, sequences[i])
		}
	}
}

func TestSBIRepository_ResetSBIState(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBI with IMPLEMENTING status
	s, err := sbi.NewSBI("Test SBI", "Description", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	s.UpdateStatus(model.StatusPicked)
	s.UpdateStatus(model.StatusImplementing)

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Reset to PENDING
	err = repo.ResetSBIState(ctx, sbiID, "PENDING")
	if err != nil {
		t.Fatalf("Failed to reset SBI state: %v", err)
	}

	// Verify status was reset
	found, err := repo.Find(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to find SBI: %v", err)
	}

	if found.Status() != model.StatusPending {
		t.Errorf("Expected status PENDING after reset, got %s", found.Status().String())
	}
}

func TestSBIRepository_ResetSBIStateInvalidStatus(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	s, err := sbi.NewSBI("Test SBI", "Description", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Try to reset with invalid status
	err = repo.ResetSBIState(ctx, sbiID, "INVALID_STATUS")
	if err == nil {
		t.Error("Expected error when resetting to invalid status")
	}
}

func TestSBIRepository_ResetSBIStateNotFound(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Try to reset non-existent SBI
	err := repo.ResetSBIState(ctx, repository.SBIID("non-existent-id"), "PENDING")
	if err == nil {
		t.Error("Expected error when resetting non-existent SBI")
	}

	if !errors.Is(err, ErrSBINotFound) {
		t.Errorf("Expected ErrSBINotFound, got %v", err)
	}
}

func TestSBIRepository_UpdateSBI(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create and save an SBI
	s, err := sbi.NewSBI("Original Title", "Original Description", nil, sbi.SBIMetadata{
		EstimatedHours: 2.0,
		Priority:       1,
	})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Update the SBI
	err = s.UpdateTitle("Updated Title")
	if err != nil {
		t.Fatalf("Failed to update title: %v", err)
	}

	s.UpdateDescription("Updated Description")
	s.UpdateMetadata(sbi.SBIMetadata{
		EstimatedHours: 5.0,
		Priority:       2,
		Labels:         []string{"updated"},
	})

	// Save the updated SBI
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save updated SBI: %v", err)
	}

	// Verify the update
	found, err := repo.Find(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to find SBI: %v", err)
	}

	if found.Title() != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", found.Title())
	}

	if found.Description() != "Updated Description" {
		t.Errorf("Expected description 'Updated Description', got '%s'", found.Description())
	}

	metadata := found.Metadata()
	if metadata.EstimatedHours != 5.0 {
		t.Errorf("Expected estimated hours 5.0, got %f", metadata.EstimatedHours)
	}

	if metadata.Priority != 2 {
		t.Errorf("Expected priority 2, got %d", metadata.Priority)
	}
}

func TestSBIRepository_ExecutionState(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBI
	s, err := sbi.NewSBI("Test SBI", "Description", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	// Modify execution state
	s.IncrementTurn()
	s.IncrementAttempt()
	s.RecordError("test error")
	s.AddArtifact("artifacts/artifact.md")

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Verify execution state was persisted
	found, err := repo.Find(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to find SBI: %v", err)
	}

	execState := found.ExecutionState()
	// Turn starts at 0, so after one IncrementTurn() it should be 1
	if execState.CurrentTurn.Value() != 1 {
		t.Errorf("Expected turn 1, got %d", execState.CurrentTurn.Value())
	}

	// After IncrementTurn, attempt is reset to 1, then IncrementAttempt makes it 2
	if execState.CurrentAttempt.Value() != 2 {
		t.Errorf("Expected attempt 2, got %d", execState.CurrentAttempt.Value())
	}

	if execState.LastError != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", execState.LastError)
	}

	if len(execState.ArtifactPaths) != 1 {
		t.Errorf("Expected 1 artifact, got %d", len(execState.ArtifactPaths))
	}
}

func TestSBIRepository_Concurrency(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	errorChan := make(chan error, 10)

	// Concurrent saves
	for i := 0; i < 10; i++ {
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
		t.Errorf("Concurrent save failed: %v", err)
	}

	// Verify all SBIs were saved
	sbis, err := repo.List(ctx, repository.SBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 10 {
		t.Errorf("Expected 10 SBIs after concurrent saves, got %d", len(sbis))
	}
}

func TestSBIRepository_ConcurrentReadsAndWrites(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Pre-populate
	for i := 0; i < 5; i++ {
		s, err := sbi.NewSBI("SBI "+string(rune('A'+i)), "Description", nil, sbi.SBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 30)

	// Concurrent reads
	for i := 0; i < 15; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := repo.List(ctx, repository.SBIFilter{})
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
	sbis, err := repo.List(ctx, repository.SBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 20 {
		t.Errorf("Expected 20 SBIs after concurrent operations, got %d", len(sbis))
	}
}

func TestSBIRepository_SequenceAssignment(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Track SBI IDs and their expected sequences
	sbiSequences := make(map[string]int)

	// Create multiple SBIs and assign sequences
	for i := 0; i < 3; i++ {
		s, err := sbi.NewSBI("SBI "+string(rune('A'+i)), "Description", nil, sbi.SBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		// Get next sequence and assign it
		seq, err := repo.GetNextSequence(ctx)
		if err != nil {
			t.Fatalf("Failed to get next sequence: %v", err)
		}

		s.SetSequence(seq)
		s.SetRegisteredAt(time.Now())

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}

		// Track expected sequence for this SBI ID
		sbiSequences[s.ID().String()] = seq
	}

	// Verify sequences
	sbis, err := repo.List(ctx, repository.SBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 3 {
		t.Fatalf("Expected 3 SBIs, got %d", len(sbis))
	}

	// Verify each SBI has the correct sequence
	// Note: List() returns map values in random order, so we check by ID
	for _, s := range sbis {
		expectedSeq, exists := sbiSequences[s.ID().String()]
		if !exists {
			t.Errorf("Unexpected SBI ID: %s", s.ID().String())
			continue
		}
		if s.Sequence() != expectedSeq {
			t.Errorf("SBI %s: expected sequence %d, got %d", s.ID().String(), expectedSeq, s.Sequence())
		}
	}

	// Verify all sequences are unique and in range [1, 3]
	seenSequences := make(map[int]bool)
	for _, s := range sbis {
		seq := s.Sequence()
		if seq < 1 || seq > 3 {
			t.Errorf("Sequence %d out of expected range [1, 3]", seq)
		}
		if seenSequences[seq] {
			t.Errorf("Duplicate sequence: %d", seq)
		}
		seenSequences[seq] = true
	}
}

func TestSBIRepository_CombinedFilters(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create a PBI
	pbiID := model.NewTaskID()

	// Create SBIs with mixed properties
	for i := 0; i < 6; i++ {
		var parentPBI *model.TaskID
		var labels []string
		var status model.Status

		if i < 3 {
			parentPBI = &pbiID
		}

		if i%2 == 0 {
			labels = []string{"backend"}
			status = model.StatusPicked
		} else {
			labels = []string{"frontend"}
			status = model.StatusPending
		}

		s, err := sbi.NewSBI("SBI "+string(rune('A'+i)), "Description", parentPBI, sbi.SBIMetadata{
			Labels: labels,
		})
		if err != nil {
			t.Fatalf("Failed to create SBI: %v", err)
		}

		if status != model.StatusPending {
			s.UpdateStatus(status)
		}

		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI: %v", err)
		}
	}

	// Test combined filter: PBI + labels + status
	pbiIDFilter := repository.PBIID(pbiID.String())
	filter := repository.SBIFilter{
		PBIID:    &pbiIDFilter,
		Labels:   []string{"backend"},
		Statuses: []model.Status{model.StatusPicked},
	}

	sbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	// Should have 2 SBIs (indices 0, 2 in first 3)
	if len(sbis) != 2 {
		t.Errorf("Expected 2 SBIs with combined filter, got %d", len(sbis))
	}

	for _, s := range sbis {
		if s.Status() != model.StatusPicked {
			t.Errorf("Expected status PICKED, got %s", s.Status().String())
		}
		if s.ParentTaskID().String() != pbiID.String() {
			t.Errorf("Expected parent PBI ID %s, got %s", pbiID.String(), s.ParentTaskID().String())
		}
	}
}

func TestSBIRepository_EmptyList(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// List from empty repository
	sbis, err := repo.List(ctx, repository.SBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list SBIs: %v", err)
	}

	if len(sbis) != 0 {
		t.Errorf("Expected 0 SBIs from empty repository, got %d", len(sbis))
	}
}

func TestSBIRepository_StatusTransitions(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Test complete status transition path
	s, err := sbi.NewSBI("Status Transition Test", "Description", nil, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	transitions := []model.Status{
		model.StatusPending,
		model.StatusPicked,
		model.StatusImplementing,
		model.StatusReviewing,
		model.StatusDone,
	}

	sbiID := repository.SBIID(s.ID().String())

	for i, expectedStatus := range transitions {
		// First transition is already set (PENDING)
		if i > 0 {
			err = s.UpdateStatus(expectedStatus)
			if err != nil {
				t.Fatalf("Failed to transition to %s: %v", expectedStatus.String(), err)
			}
		}

		// Save after each transition
		err = repo.Save(ctx, s)
		if err != nil {
			t.Fatalf("Failed to save SBI after transition to %s: %v", expectedStatus.String(), err)
		}

		// Verify status was persisted
		found, err := repo.Find(ctx, sbiID)
		if err != nil {
			t.Fatalf("Failed to find SBI: %v", err)
		}

		if found.Status() != expectedStatus {
			t.Errorf("Transition %d: Expected status %s, got %s", i, expectedStatus.String(), found.Status().String())
		}
	}
}

func TestSBIRepository_PBIMappingConsistency(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBI with PBI
	pbiID1 := model.NewTaskID()
	s, err := sbi.NewSBI("Test SBI", "Description", &pbiID1, sbi.SBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Verify SBI is found by PBI ID
	sbis, err := repo.FindByPBIID(ctx, repository.PBIID(pbiID1.String()))
	if err != nil {
		t.Fatalf("Failed to find SBIs by PBI ID: %v", err)
	}

	if len(sbis) != 1 {
		t.Errorf("Expected 1 SBI for PBI 1, got %d", len(sbis))
	}

	// Change parent PBI
	pbiID2 := model.NewTaskID()
	updatedSBI := sbi.ReconstructSBI(
		s.ID(),
		s.Title(),
		s.Description(),
		s.Status(),
		s.CurrentStep(),
		&pbiID2, // New parent PBI
		s.Metadata(),
		s.ExecutionState(),
		s.CreatedAt().Value(),
		s.UpdatedAt().Value(),
	)

	err = repo.Save(ctx, updatedSBI)
	if err != nil {
		t.Fatalf("Failed to save updated SBI: %v", err)
	}

	// Verify SBI no longer found under PBI 1
	sbis, err = repo.FindByPBIID(ctx, repository.PBIID(pbiID1.String()))
	if err != nil {
		t.Fatalf("Failed to find SBIs by PBI ID 1: %v", err)
	}

	if len(sbis) != 0 {
		t.Errorf("Expected 0 SBIs for PBI 1 after reassignment, got %d", len(sbis))
	}

	// Verify SBI is found under PBI 2
	sbis, err = repo.FindByPBIID(ctx, repository.PBIID(pbiID2.String()))
	if err != nil {
		t.Fatalf("Failed to find SBIs by PBI ID 2: %v", err)
	}

	if len(sbis) != 1 {
		t.Errorf("Expected 1 SBI for PBI 2, got %d", len(sbis))
	}

	// Verify correct SBI
	found, err := repo.Find(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to find SBI: %v", err)
	}

	if found.ParentTaskID().String() != pbiID2.String() {
		t.Errorf("Expected parent PBI ID %s, got %s", pbiID2.String(), found.ParentTaskID().String())
	}
}

func TestSBIRepository_MetadataUpdates(t *testing.T) {
	repo := NewMockSBIRepository()
	ctx := context.Background()

	// Create SBI with initial metadata
	s, err := sbi.NewSBI("Metadata Test", "Description", nil, sbi.SBIMetadata{
		EstimatedHours: 2.0,
		Priority:       1,
		Labels:         []string{"backend"},
		AssignedAgent:  "claude-code",
		FilePaths:      []string{"file1.go"},
	})
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	sbiID := repository.SBIID(s.ID().String())
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save SBI: %v", err)
	}

	// Update metadata
	newMetadata := sbi.SBIMetadata{
		EstimatedHours: 5.0,
		Priority:       2,
		Labels:         []string{"frontend", "ui", "critical"},
		AssignedAgent:  "gemini-cli",
		FilePaths:      []string{"file1.go", "file2.go", "file3.go"},
	}
	s.UpdateMetadata(newMetadata)

	// Save updated SBI
	err = repo.Save(ctx, s)
	if err != nil {
		t.Fatalf("Failed to save updated SBI: %v", err)
	}

	// Verify updates were persisted
	found, err := repo.Find(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to find SBI: %v", err)
	}

	metadata := found.Metadata()
	if metadata.EstimatedHours != 5.0 {
		t.Errorf("Expected estimated hours 5.0, got %f", metadata.EstimatedHours)
	}

	if metadata.Priority != 2 {
		t.Errorf("Expected priority 2, got %d", metadata.Priority)
	}

	if len(metadata.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(metadata.Labels))
	}

	if metadata.AssignedAgent != "gemini-cli" {
		t.Errorf("Expected agent 'gemini-cli', got '%s'", metadata.AssignedAgent)
	}

	if len(metadata.FilePaths) != 3 {
		t.Errorf("Expected 3 file paths, got %d", len(metadata.FilePaths))
	}
}
