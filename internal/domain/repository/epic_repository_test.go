package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockEPICRepository is a mock implementation of EPICRepository for testing
type MockEPICRepository struct {
	mu     sync.RWMutex
	epics  map[repository.EPICID]*epic.EPIC
	pbiMap map[repository.PBIID]repository.EPICID // Maps PBI IDs to EPIC IDs
}

// NewMockEPICRepository creates a new mock EPIC repository
func NewMockEPICRepository() *MockEPICRepository {
	return &MockEPICRepository{
		epics:  make(map[repository.EPICID]*epic.EPIC),
		pbiMap: make(map[repository.PBIID]repository.EPICID),
	}
}

// Find retrieves an EPIC by its ID
func (m *MockEPICRepository) Find(ctx context.Context, id repository.EPICID) (*epic.EPIC, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	e, exists := m.epics[id]
	if !exists {
		return nil, ErrEPICNotFound
	}
	return e, nil
}

// Save persists an EPIC entity
func (m *MockEPICRepository) Save(ctx context.Context, e *epic.EPIC) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := repository.EPICID(e.ID().String())

	// If EPIC already exists, clear old PBI mappings first
	if oldEPIC, exists := m.epics[id]; exists {
		for _, oldPBIID := range oldEPIC.PBIIDs() {
			delete(m.pbiMap, repository.PBIID(oldPBIID.String()))
		}
	}

	m.epics[id] = e

	// Update PBI mappings with current PBIs
	for _, pbiID := range e.PBIIDs() {
		m.pbiMap[repository.PBIID(pbiID.String())] = id
	}

	return nil
}

// Delete removes an EPIC
func (m *MockEPICRepository) Delete(ctx context.Context, id repository.EPICID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	e, exists := m.epics[id]
	if !exists {
		return ErrEPICNotFound
	}

	// Check if EPIC can be deleted
	if !e.CanDelete() {
		return errors.New("cannot delete EPIC with child PBIs")
	}

	// Remove PBI mappings
	for _, pbiID := range e.PBIIDs() {
		delete(m.pbiMap, repository.PBIID(pbiID.String()))
	}

	delete(m.epics, id)
	return nil
}

// List retrieves EPICs by filter
func (m *MockEPICRepository) List(ctx context.Context, filter repository.EPICFilter) ([]*epic.EPIC, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*epic.EPIC
	for _, e := range m.epics {
		if m.matchesFilter(e, filter) {
			result = append(result, e)
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

// FindByPBIID retrieves the parent EPIC of a PBI
func (m *MockEPICRepository) FindByPBIID(ctx context.Context, pbiID repository.PBIID) (*epic.EPIC, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	epicID, exists := m.pbiMap[pbiID]
	if !exists {
		return nil, ErrEPICNotFound
	}

	e, exists := m.epics[epicID]
	if !exists {
		return nil, ErrEPICNotFound
	}

	return e, nil
}

func (m *MockEPICRepository) matchesFilter(e *epic.EPIC, filter repository.EPICFilter) bool {
	// Status filter
	if len(filter.Statuses) > 0 {
		matched := false
		for _, s := range filter.Statuses {
			if repository.Status(e.Status().String()) == s {
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

// ErrEPICNotFound is returned when an EPIC is not found
var ErrEPICNotFound = errors.New("epic not found")

// Test Suite for EPICRepository

func TestEPICRepository_Find(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create and save an EPIC
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{
		EstimatedStoryPoints: 10,
		Priority:             1,
		Labels:               []string{"backend"},
		AssignedAgent:        "claude-code",
	})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Test finding the EPIC
	found, err := repo.Find(ctx, epicID)
	if err != nil {
		t.Fatalf("Failed to find EPIC: %v", err)
	}

	if found.ID().String() != e.ID().String() {
		t.Errorf("Expected EPIC ID %s, got %s", e.ID().String(), found.ID().String())
	}

	if found.Title() != "Test EPIC" {
		t.Errorf("Expected title 'Test EPIC', got '%s'", found.Title())
	}
}

func TestEPICRepository_FindNotFound(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Try to find non-existent EPIC
	_, err := repo.Find(ctx, repository.EPICID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when finding non-existent EPIC")
	}

	if !errors.Is(err, ErrEPICNotFound) {
		t.Errorf("Expected ErrEPICNotFound, got %v", err)
	}
}

func TestEPICRepository_Save(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	tests := []struct {
		name        string
		title       string
		description string
		metadata    epic.EPICMetadata
	}{
		{
			name:        "Basic EPIC",
			title:       "Feature A",
			description: "Implement feature A",
			metadata: epic.EPICMetadata{
				EstimatedStoryPoints: 5,
				Priority:             1,
			},
		},
		{
			name:        "EPIC with labels",
			title:       "Feature B",
			description: "Implement feature B",
			metadata: epic.EPICMetadata{
				EstimatedStoryPoints: 10,
				Priority:             2,
				Labels:               []string{"frontend", "ui"},
			},
		},
		{
			name:        "EPIC with agent",
			title:       "Feature C",
			description: "Implement feature C",
			metadata: epic.EPICMetadata{
				EstimatedStoryPoints: 8,
				Priority:             1,
				AssignedAgent:        "gemini-cli",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := epic.NewEPIC(tt.title, tt.description, tt.metadata)
			if err != nil {
				t.Fatalf("Failed to create EPIC: %v", err)
			}

			err = repo.Save(ctx, e)
			if err != nil {
				t.Fatalf("Failed to save EPIC: %v", err)
			}

			// Verify saved EPIC
			epicID := repository.EPICID(e.ID().String())
			found, err := repo.Find(ctx, epicID)
			if err != nil {
				t.Fatalf("Failed to find saved EPIC: %v", err)
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

func TestEPICRepository_Delete(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create and save an EPIC without PBIs
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Delete the EPIC
	err = repo.Delete(ctx, epicID)
	if err != nil {
		t.Fatalf("Failed to delete EPIC: %v", err)
	}

	// Verify EPIC is deleted
	_, err = repo.Find(ctx, epicID)
	if err == nil {
		t.Error("Expected error when finding deleted EPIC")
	}
}

func TestEPICRepository_DeleteWithPBIs(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create EPIC with a PBI
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	// Add a PBI to the EPIC
	pbiID := model.NewTaskID()
	err = e.AddPBI(pbiID)
	if err != nil {
		t.Fatalf("Failed to add PBI: %v", err)
	}

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Try to delete EPIC with PBIs
	err = repo.Delete(ctx, epicID)
	if err == nil {
		t.Error("Expected error when deleting EPIC with PBIs")
	}
}

func TestEPICRepository_DeleteNotFound(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Try to delete non-existent EPIC
	err := repo.Delete(ctx, repository.EPICID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when deleting non-existent EPIC")
	}
}

func TestEPICRepository_List(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create multiple EPICs with different statuses
	// Note: Status transitions must follow: PENDING -> PICKED -> IMPLEMENTING -> REVIEWING -> DONE
	statuses := []struct {
		transitions []model.Status
	}{
		{transitions: []model.Status{model.StatusPending}},
		{transitions: []model.Status{model.StatusPending, model.StatusPicked}},
		{transitions: []model.Status{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone}},
	}

	for i, status := range statuses {
		e, err := epic.NewEPIC(
			"EPIC "+string(rune('A'+i)),
			"Description",
			epic.EPICMetadata{Priority: i + 1},
		)
		if err != nil {
			t.Fatalf("Failed to create EPIC: %v", err)
		}

		// Apply status transitions in order
		for j := 1; j < len(status.transitions); j++ {
			err = e.UpdateStatus(status.transitions[j])
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC: %v", err)
		}
	}

	// Test listing all EPICs
	allEPICs, err := repo.List(ctx, repository.EPICFilter{})
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(allEPICs) != 3 {
		t.Errorf("Expected 3 EPICs, got %d", len(allEPICs))
	}
}

func TestEPICRepository_ListWithStatusFilter(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create EPICs with different statuses
	// Note: Status transitions must follow: PENDING -> PICKED -> IMPLEMENTING -> REVIEWING -> DONE
	statusTransitions := [][]model.Status{
		{model.StatusPending},
		{model.StatusPending, model.StatusPicked},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
	}

	for i, transitions := range statusTransitions {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		if err != nil {
			t.Fatalf("Failed to create EPIC: %v", err)
		}

		// Apply status transitions in order
		for j := 1; j < len(transitions); j++ {
			err = e.UpdateStatus(transitions[j])
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC: %v", err)
		}
	}

	// Filter by DONE status
	filter := repository.EPICFilter{
		Statuses: []repository.Status{repository.Status(model.StatusDone.String())},
	}

	doneEPICs, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(doneEPICs) != 2 {
		t.Errorf("Expected 2 DONE EPICs, got %d", len(doneEPICs))
	}

	for _, e := range doneEPICs {
		if e.Status() != model.StatusDone {
			t.Errorf("Expected status DONE, got %s", e.Status().String())
		}
	}
}

func TestEPICRepository_ListWithPagination(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create 5 EPICs
	for i := 0; i < 5; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		if err != nil {
			t.Fatalf("Failed to create EPIC: %v", err)
		}

		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC: %v", err)
		}
	}

	// Test with limit
	filter := repository.EPICFilter{
		Limit: 2,
	}

	epics, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 2 {
		t.Errorf("Expected 2 EPICs with limit=2, got %d", len(epics))
	}

	// Test with offset
	filter = repository.EPICFilter{
		Offset: 3,
	}

	epics, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 2 {
		t.Errorf("Expected 2 EPICs with offset=3, got %d", len(epics))
	}

	// Test with both limit and offset
	filter = repository.EPICFilter{
		Limit:  2,
		Offset: 1,
	}

	epics, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 2 {
		t.Errorf("Expected 2 EPICs with limit=2 and offset=1, got %d", len(epics))
	}
}

func TestEPICRepository_FindByPBIID(t *testing.T) {
	repo := NewMockEPICRepository()
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

	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Find EPIC by PBI ID
	found, err := repo.FindByPBIID(ctx, repository.PBIID(pbiID.String()))
	if err != nil {
		t.Fatalf("Failed to find EPIC by PBI ID: %v", err)
	}

	if found.ID().String() != e.ID().String() {
		t.Errorf("Expected EPIC ID %s, got %s", e.ID().String(), found.ID().String())
	}

	// Verify the PBI is in the EPIC
	pbiIDs := found.PBIIDs()
	foundPBI := false
	for _, id := range pbiIDs {
		if id.String() == pbiID.String() {
			foundPBI = true
			break
		}
	}

	if !foundPBI {
		t.Error("PBI not found in EPIC")
	}
}

func TestEPICRepository_FindByPBIIDNotFound(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Try to find EPIC by non-existent PBI ID
	_, err := repo.FindByPBIID(ctx, repository.PBIID("non-existent-pbi"))
	if err == nil {
		t.Error("Expected error when finding EPIC by non-existent PBI ID")
	}

	if !errors.Is(err, ErrEPICNotFound) {
		t.Errorf("Expected ErrEPICNotFound, got %v", err)
	}
}

func TestEPICRepository_UpdateEPIC(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create and save an EPIC
	e, err := epic.NewEPIC("Original Title", "Original Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Update the EPIC
	err = e.UpdateTitle("Updated Title")
	if err != nil {
		t.Fatalf("Failed to update title: %v", err)
	}

	e.UpdateDescription("Updated Description")

	// Save the updated EPIC
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save updated EPIC: %v", err)
	}

	// Verify the update
	found, err := repo.Find(ctx, epicID)
	if err != nil {
		t.Fatalf("Failed to find EPIC: %v", err)
	}

	if found.Title() != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", found.Title())
	}

	if found.Description() != "Updated Description" {
		t.Errorf("Expected description 'Updated Description', got '%s'", found.Description())
	}
}

func TestEPICRepository_Concurrency(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Test concurrent saves
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			e, err := epic.NewEPIC("EPIC "+string(rune('A'+index)), "Description", epic.EPICMetadata{})
			if err != nil {
				t.Errorf("Failed to create EPIC: %v", err)
				return
			}

			err = repo.Save(ctx, e)
			if err != nil {
				t.Errorf("Failed to save EPIC: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all EPICs were saved
	epics, err := repo.List(ctx, repository.EPICFilter{})
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 10 {
		t.Errorf("Expected 10 EPICs after concurrent saves, got %d", len(epics))
	}
}

func TestEPICRepository_EmptyList(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// List from empty repository
	epics, err := repo.List(ctx, repository.EPICFilter{})
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 0 {
		t.Errorf("Expected 0 EPICs from empty repository, got %d", len(epics))
	}
}

// Turn 2 Additional Tests - Edge Cases and Complex Scenarios

func TestEPICRepository_SaveAndOverwrite(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create and save an EPIC
	e, err := epic.NewEPIC("Original Title", "Original Description", epic.EPICMetadata{
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Modify and save again (overwrite)
	err = e.UpdateTitle("Modified Title")
	if err != nil {
		t.Fatalf("Failed to update title: %v", err)
	}

	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to overwrite EPIC: %v", err)
	}

	// Verify only one EPIC exists with updated data
	allEPICs, err := repo.List(ctx, repository.EPICFilter{})
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(allEPICs) != 1 {
		t.Errorf("Expected 1 EPIC after overwrite, got %d", len(allEPICs))
	}

	found, err := repo.Find(ctx, epicID)
	if err != nil {
		t.Fatalf("Failed to find EPIC: %v", err)
	}

	if found.Title() != "Modified Title" {
		t.Errorf("Expected title 'Modified Title', got '%s'", found.Title())
	}
}

func TestEPICRepository_MultiplePBIManagement(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create EPIC and add multiple PBIs
	e, err := epic.NewEPIC("Multi-PBI EPIC", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	pbiIDs := make([]model.TaskID, 3)
	for i := 0; i < 3; i++ {
		pbiIDs[i] = model.NewTaskID()
		err = e.AddPBI(pbiIDs[i])
		if err != nil {
			t.Fatalf("Failed to add PBI %d: %v", i, err)
		}
	}

	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Verify all PBIs can be found
	for i, pbiID := range pbiIDs {
		found, err := repo.FindByPBIID(ctx, repository.PBIID(pbiID.String()))
		if err != nil {
			t.Errorf("Failed to find EPIC by PBI %d: %v", i, err)
		}

		if found.ID().String() != e.ID().String() {
			t.Errorf("PBI %d: Expected EPIC ID %s, got %s", i, e.ID().String(), found.ID().String())
		}
	}

	// Verify EPIC has all PBIs
	savedEPIC, err := repo.Find(ctx, repository.EPICID(e.ID().String()))
	if err != nil {
		t.Fatalf("Failed to find EPIC: %v", err)
	}

	if len(savedEPIC.PBIIDs()) != 3 {
		t.Errorf("Expected 3 PBIs, got %d", len(savedEPIC.PBIIDs()))
	}
}

func TestEPICRepository_PBIMappingConsistency(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create first EPIC with a PBI
	e1, err := epic.NewEPIC("EPIC 1", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC 1: %v", err)
	}

	pbiID := model.NewTaskID()
	err = e1.AddPBI(pbiID)
	if err != nil {
		t.Fatalf("Failed to add PBI to EPIC 1: %v", err)
	}

	err = repo.Save(ctx, e1)
	if err != nil {
		t.Fatalf("Failed to save EPIC 1: %v", err)
	}

	// Verify PBI maps to EPIC 1
	found, err := repo.FindByPBIID(ctx, repository.PBIID(pbiID.String()))
	if err != nil {
		t.Fatalf("Failed to find EPIC by PBI ID: %v", err)
	}

	if found.ID().String() != e1.ID().String() {
		t.Errorf("Expected EPIC 1, got EPIC %s", found.ID().String())
	}

	// Remove PBI from EPIC 1
	err = e1.RemovePBI(pbiID)
	if err != nil {
		t.Fatalf("Failed to remove PBI from EPIC 1: %v", err)
	}

	err = repo.Save(ctx, e1)
	if err != nil {
		t.Fatalf("Failed to save updated EPIC 1: %v", err)
	}

	// Verify PBI no longer maps to any EPIC
	_, err = repo.FindByPBIID(ctx, repository.PBIID(pbiID.String()))
	if err == nil {
		t.Error("Expected error when finding EPIC for removed PBI")
	}
}

func TestEPICRepository_ListWithMultipleStatusFilters(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create EPICs with different statuses
	statusTransitions := [][]model.Status{
		{model.StatusPending},
		{model.StatusPending, model.StatusPicked},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
	}

	for i, transitions := range statusTransitions {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		if err != nil {
			t.Fatalf("Failed to create EPIC: %v", err)
		}

		for j := 1; j < len(transitions); j++ {
			err = e.UpdateStatus(transitions[j])
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC: %v", err)
		}
	}

	// Filter by multiple statuses
	filter := repository.EPICFilter{
		Statuses: []repository.Status{
			repository.Status(model.StatusPending.String()),
			repository.Status(model.StatusPicked.String()),
		},
	}

	epics, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 2 {
		t.Errorf("Expected 2 EPICs (PENDING + PICKED), got %d", len(epics))
	}

	// Verify statuses
	for _, e := range epics {
		status := e.Status()
		if status != model.StatusPending && status != model.StatusPicked {
			t.Errorf("Unexpected status %s in filtered results", status.String())
		}
	}
}

func TestEPICRepository_PaginationEdgeCases(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create 3 EPICs
	for i := 0; i < 3; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		if err != nil {
			t.Fatalf("Failed to create EPIC: %v", err)
		}

		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC: %v", err)
		}
	}

	tests := []struct {
		name           string
		filter         repository.EPICFilter
		expectedCount  int
		description    string
	}{
		{
			name:          "Offset exceeds total",
			filter:        repository.EPICFilter{Offset: 10},
			expectedCount: 0,
			description:   "Offset larger than total should return empty",
		},
		{
			name:          "Limit exceeds remaining",
			filter:        repository.EPICFilter{Limit: 10},
			expectedCount: 3,
			description:   "Limit larger than total should return all",
		},
		{
			name:          "Offset equals total",
			filter:        repository.EPICFilter{Offset: 3},
			expectedCount: 0,
			description:   "Offset equal to total should return empty",
		},
		{
			name:          "Limit is zero",
			filter:        repository.EPICFilter{Limit: 0},
			expectedCount: 3,
			description:   "Limit of 0 should return all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epics, err := repo.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Failed to list EPICs: %v", err)
			}

			if len(epics) != tt.expectedCount {
				t.Errorf("%s: expected %d EPICs, got %d", tt.description, tt.expectedCount, len(epics))
			}
		})
	}
}

func TestEPICRepository_AllStatusTransitions(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Test the complete valid status transition path
	e, err := epic.NewEPIC("Status Transition Test", "Description", epic.EPICMetadata{})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	// Expected transitions: PENDING -> PICKED -> IMPLEMENTING -> REVIEWING -> DONE
	transitions := []model.Status{
		model.StatusPending,
		model.StatusPicked,
		model.StatusImplementing,
		model.StatusReviewing,
		model.StatusDone,
	}

	epicID := repository.EPICID(e.ID().String())

	for i, expectedStatus := range transitions {
		// First transition is already set (PENDING)
		if i > 0 {
			err = e.UpdateStatus(expectedStatus)
			if err != nil {
				t.Fatalf("Failed to transition to %s: %v", expectedStatus.String(), err)
			}
		}

		// Save after each transition
		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC after transition to %s: %v", expectedStatus.String(), err)
		}

		// Verify status was persisted
		found, err := repo.Find(ctx, epicID)
		if err != nil {
			t.Fatalf("Failed to find EPIC: %v", err)
		}

		if found.Status() != expectedStatus {
			t.Errorf("Transition %d: Expected status %s, got %s", i, expectedStatus.String(), found.Status().String())
		}
	}
}

func TestEPICRepository_MetadataUpdates(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create EPIC with initial metadata
	e, err := epic.NewEPIC("Metadata Test", "Description", epic.EPICMetadata{
		EstimatedStoryPoints: 5,
		Priority:             3,
		Labels:               []string{"backend"},
		AssignedAgent:        "claude-code",
	})
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save EPIC: %v", err)
	}

	// Update priority
	err = e.UpdatePriority(1)
	if err != nil {
		t.Fatalf("Failed to update priority: %v", err)
	}

	// Update labels
	e.UpdateLabels([]string{"frontend", "ui", "critical"})

	// Update agent
	e.UpdateAssignedAgent("gemini-cli")

	// Save updated EPIC
	err = repo.Save(ctx, e)
	if err != nil {
		t.Fatalf("Failed to save updated EPIC: %v", err)
	}

	// Verify updates were persisted
	found, err := repo.Find(ctx, epicID)
	if err != nil {
		t.Fatalf("Failed to find EPIC: %v", err)
	}

	if found.Priority() != 1 {
		t.Errorf("Expected priority 1, got %d", found.Priority())
	}

	labels := found.Labels()
	if len(labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(labels))
	}

	if found.AssignedAgent() != "gemini-cli" {
		t.Errorf("Expected agent 'gemini-cli', got '%s'", found.AssignedAgent())
	}
}

func TestEPICRepository_ConcurrentReadsAndWrites(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Pre-populate with some EPICs
	for i := 0; i < 5; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		if err != nil {
			t.Fatalf("Failed to create EPIC: %v", err)
		}

		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC: %v", err)
		}
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 20)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := repo.List(ctx, repository.EPICFilter{})
			if err != nil {
				errorChan <- err
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			e, err := epic.NewEPIC("New EPIC "+string(rune('A'+index)), "Description", epic.EPICMetadata{})
			if err != nil {
				errorChan <- err
				return
			}

			err = repo.Save(ctx, e)
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
	epics, err := repo.List(ctx, repository.EPICFilter{})
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	if len(epics) != 15 {
		t.Errorf("Expected 15 EPICs after concurrent operations, got %d", len(epics))
	}
}

func TestEPICRepository_FindEmptyID(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Try to find EPIC with empty ID
	_, err := repo.Find(ctx, repository.EPICID(""))
	if err == nil {
		t.Error("Expected error when finding EPIC with empty ID")
	}
}

func TestEPICRepository_ListCombinedFilters(t *testing.T) {
	repo := NewMockEPICRepository()
	ctx := context.Background()

	// Create 10 EPICs with mixed statuses
	for i := 0; i < 10; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		if err != nil {
			t.Fatalf("Failed to create EPIC: %v", err)
		}

		// Every other EPIC gets moved to PICKED status
		if i%2 == 0 {
			err = e.UpdateStatus(model.StatusPicked)
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, e)
		if err != nil {
			t.Fatalf("Failed to save EPIC: %v", err)
		}
	}

	// Test combined filter: status + pagination
	filter := repository.EPICFilter{
		Statuses: []repository.Status{repository.Status(model.StatusPicked.String())},
		Limit:    2,
		Offset:   1,
	}

	epics, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list EPICs: %v", err)
	}

	// Should have 5 PICKED EPICs total, with offset 1 and limit 2, we get 2
	if len(epics) != 2 {
		t.Errorf("Expected 2 EPICs with combined filter, got %d", len(epics))
	}

	// All returned EPICs should be PICKED
	for _, e := range epics {
		if e.Status() != model.StatusPicked {
			t.Errorf("Expected PICKED status, got %s", e.Status().String())
		}
	}
}
