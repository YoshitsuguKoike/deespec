package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockPBIRepository is a mock implementation of PBIRepository for testing
type MockPBIRepository struct {
	mu       sync.RWMutex
	pbis     map[repository.PBIID]*pbi.PBI
	epicMap  map[repository.PBIID]repository.EPICID  // Maps PBI IDs to EPIC IDs
	sbiMap   map[repository.SBIID]repository.PBIID   // Maps SBI IDs to PBI IDs
}

// NewMockPBIRepository creates a new mock PBI repository
func NewMockPBIRepository() *MockPBIRepository {
	return &MockPBIRepository{
		pbis:    make(map[repository.PBIID]*pbi.PBI),
		epicMap: make(map[repository.PBIID]repository.EPICID),
		sbiMap:  make(map[repository.SBIID]repository.PBIID),
	}
}

// Find retrieves a PBI by its ID
func (m *MockPBIRepository) Find(ctx context.Context, id repository.PBIID) (*pbi.PBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, exists := m.pbis[id]
	if !exists {
		return nil, ErrPBINotFound
	}
	return p, nil
}

// Save persists a PBI entity
func (m *MockPBIRepository) Save(ctx context.Context, p *pbi.PBI) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := repository.PBIID(p.ID().String())

	// Clear old SBI mappings that point to this PBI
	for sbiID, pbiID := range m.sbiMap {
		if pbiID == id {
			delete(m.sbiMap, sbiID)
		}
	}

	m.pbis[id] = p

	// Update EPIC mapping if PBI has a parent EPIC
	if p.HasParentEPIC() {
		m.epicMap[id] = repository.EPICID(p.ParentTaskID().String())
	} else {
		delete(m.epicMap, id)
	}

	// Add new SBI mappings for current SBIs
	for _, sbiID := range p.SBIIDs() {
		m.sbiMap[repository.SBIID(sbiID.String())] = id
	}

	return nil
}

// Delete removes a PBI
func (m *MockPBIRepository) Delete(ctx context.Context, id repository.PBIID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.pbis[id]
	if !exists {
		return ErrPBINotFound
	}

	// Check if PBI can be deleted
	if !p.CanDelete() {
		return errors.New("cannot delete PBI with child SBIs")
	}

	// Remove SBI mappings
	for _, sbiID := range p.SBIIDs() {
		delete(m.sbiMap, repository.SBIID(sbiID.String()))
	}

	// Remove EPIC mapping
	delete(m.epicMap, id)

	delete(m.pbis, id)
	return nil
}

// List retrieves PBIs by filter
func (m *MockPBIRepository) List(ctx context.Context, filter repository.PBIFilter) ([]*pbi.PBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*pbi.PBI
	for _, p := range m.pbis {
		if m.matchesFilter(p, filter) {
			result = append(result, p)
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

// FindByEPICID retrieves all PBIs belonging to an EPIC
func (m *MockPBIRepository) FindByEPICID(ctx context.Context, epicID repository.EPICID) ([]*pbi.PBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*pbi.PBI
	for pbiID, eid := range m.epicMap {
		if eid == epicID {
			if p, exists := m.pbis[pbiID]; exists {
				result = append(result, p)
			}
		}
	}

	return result, nil
}

// FindBySBIID retrieves the parent PBI of an SBI
func (m *MockPBIRepository) FindBySBIID(ctx context.Context, sbiID repository.SBIID) (*pbi.PBI, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pbiID, exists := m.sbiMap[sbiID]
	if !exists {
		return nil, ErrPBINotFound
	}

	p, exists := m.pbis[pbiID]
	if !exists {
		return nil, ErrPBINotFound
	}

	return p, nil
}

func (m *MockPBIRepository) matchesFilter(p *pbi.PBI, filter repository.PBIFilter) bool {
	// EPIC ID filter
	if filter.EPICID != nil {
		pbiID := repository.PBIID(p.ID().String())
		epicID, exists := m.epicMap[pbiID]
		if !exists || epicID != *filter.EPICID {
			return false
		}
	}

	// Status filter
	if len(filter.Statuses) > 0 {
		matched := false
		for _, s := range filter.Statuses {
			if repository.Status(p.Status().String()) == s {
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

// ErrPBINotFound is returned when a PBI is not found
var ErrPBINotFound = errors.New("pbi not found")

// Test Suite for PBIRepository

func TestPBIRepository_Find(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create and save a PBI
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{
		StoryPoints: 5,
		Priority:    1,
		Labels:      []string{"backend"},
		AssignedAgent: "claude-code",
	})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Test finding the PBI
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if found.ID().String() != p.ID().String() {
		t.Errorf("Expected PBI ID %s, got %s", p.ID().String(), found.ID().String())
	}

	if found.Title() != "Test PBI" {
		t.Errorf("Expected title 'Test PBI', got '%s'", found.Title())
	}
}

func TestPBIRepository_FindNotFound(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Try to find non-existent PBI
	_, err := repo.Find(ctx, repository.PBIID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when finding non-existent PBI")
	}

	if !errors.Is(err, ErrPBINotFound) {
		t.Errorf("Expected ErrPBINotFound, got %v", err)
	}
}

func TestPBIRepository_Save(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	tests := []struct {
		name        string
		title       string
		description string
		metadata    pbi.PBIMetadata
	}{
		{
			name:        "Basic PBI",
			title:       "Feature A",
			description: "Implement feature A",
			metadata: pbi.PBIMetadata{
				StoryPoints: 3,
				Priority:    1,
			},
		},
		{
			name:        "PBI with labels",
			title:       "Feature B",
			description: "Implement feature B",
			metadata: pbi.PBIMetadata{
				StoryPoints: 5,
				Priority:    2,
				Labels:      []string{"frontend", "ui"},
			},
		},
		{
			name:        "PBI with agent and acceptance criteria",
			title:       "Feature C",
			description: "Implement feature C",
			metadata: pbi.PBIMetadata{
				StoryPoints:        8,
				Priority:           1,
				AssignedAgent:      "gemini-cli",
				AcceptanceCriteria: []string{"Criterion 1", "Criterion 2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := pbi.NewPBI(tt.title, tt.description, nil, tt.metadata)
			if err != nil {
				t.Fatalf("Failed to create PBI: %v", err)
			}

			err = repo.Save(ctx, p)
			if err != nil {
				t.Fatalf("Failed to save PBI: %v", err)
			}

			// Verify saved PBI
			pbiID := repository.PBIID(p.ID().String())
			found, err := repo.Find(ctx, pbiID)
			if err != nil {
				t.Fatalf("Failed to find saved PBI: %v", err)
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

func TestPBIRepository_SaveWithParentEPIC(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with parent EPIC
	epicID := model.NewTaskID()
	p, err := pbi.NewPBI("Test PBI", "Description", &epicID, pbi.PBIMetadata{
		StoryPoints: 5,
		Priority:    1,
	})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Verify PBI has parent EPIC
	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if !found.HasParentEPIC() {
		t.Error("Expected PBI to have parent EPIC")
	}

	if found.ParentTaskID().String() != epicID.String() {
		t.Errorf("Expected parent EPIC ID %s, got %s", epicID.String(), found.ParentTaskID().String())
	}
}

func TestPBIRepository_Delete(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create and save a PBI without SBIs
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Delete the PBI
	err = repo.Delete(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to delete PBI: %v", err)
	}

	// Verify PBI is deleted
	_, err = repo.Find(ctx, pbiID)
	if err == nil {
		t.Error("Expected error when finding deleted PBI")
	}
}

func TestPBIRepository_DeleteWithSBIs(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with an SBI
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	// Add an SBI to the PBI
	sbiID := model.NewTaskID()
	err = p.AddSBI(sbiID)
	if err != nil {
		t.Fatalf("Failed to add SBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Try to delete PBI with SBIs
	err = repo.Delete(ctx, pbiID)
	if err == nil {
		t.Error("Expected error when deleting PBI with SBIs")
	}
}

func TestPBIRepository_DeleteNotFound(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Try to delete non-existent PBI
	err := repo.Delete(ctx, repository.PBIID("non-existent-id"))
	if err == nil {
		t.Error("Expected error when deleting non-existent PBI")
	}
}

func TestPBIRepository_List(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create multiple PBIs with different statuses
	// Note: Status transitions must follow: PENDING -> PICKED -> IMPLEMENTING -> REVIEWING -> DONE
	statuses := []struct {
		transitions []model.Status
	}{
		{transitions: []model.Status{model.StatusPending}},
		{transitions: []model.Status{model.StatusPending, model.StatusPicked}},
		{transitions: []model.Status{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone}},
	}

	for i, status := range statuses {
		p, err := pbi.NewPBI(
			"PBI "+string(rune('A'+i)),
			"Description",
			nil,
			pbi.PBIMetadata{Priority: i + 1},
		)
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		// Apply status transitions in order
		for j := 1; j < len(status.transitions); j++ {
			err = p.UpdateStatus(status.transitions[j])
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	// Test listing all PBIs
	allPBIs, err := repo.List(ctx, repository.PBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(allPBIs) != 3 {
		t.Errorf("Expected 3 PBIs, got %d", len(allPBIs))
	}
}

func TestPBIRepository_ListWithStatusFilter(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBIs with different statuses
	statusTransitions := [][]model.Status{
		{model.StatusPending},
		{model.StatusPending, model.StatusPicked},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
	}

	for i, transitions := range statusTransitions {
		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		// Apply status transitions in order
		for j := 1; j < len(transitions); j++ {
			err = p.UpdateStatus(transitions[j])
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	// Filter by DONE status
	filter := repository.PBIFilter{
		Statuses: []repository.Status{repository.Status(model.StatusDone.String())},
	}

	donePBIs, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(donePBIs) != 2 {
		t.Errorf("Expected 2 DONE PBIs, got %d", len(donePBIs))
	}

	for _, p := range donePBIs {
		if p.Status() != model.StatusDone {
			t.Errorf("Expected status DONE, got %s", p.Status().String())
		}
	}
}

func TestPBIRepository_ListWithEPICFilter(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create two EPICs
	epicID1 := model.NewTaskID()
	epicID2 := model.NewTaskID()

	// Create PBIs belonging to different EPICs
	for i := 0; i < 3; i++ {
		var parentEPIC *model.TaskID
		if i < 2 {
			parentEPIC = &epicID1
		} else {
			parentEPIC = &epicID2
		}

		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", parentEPIC, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	// Filter by EPIC ID
	epicIDFilter := repository.EPICID(epicID1.String())
	filter := repository.PBIFilter{
		EPICID: &epicIDFilter,
	}

	pbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 2 {
		t.Errorf("Expected 2 PBIs for EPIC 1, got %d", len(pbis))
	}

	for _, p := range pbis {
		if p.ParentTaskID().String() != epicID1.String() {
			t.Errorf("Expected parent EPIC ID %s, got %s", epicID1.String(), p.ParentTaskID().String())
		}
	}
}

func TestPBIRepository_ListWithPagination(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create 5 PBIs
	for i := 0; i < 5; i++ {
		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	// Test with limit
	filter := repository.PBIFilter{
		Limit: 2,
	}

	pbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 2 {
		t.Errorf("Expected 2 PBIs with limit=2, got %d", len(pbis))
	}

	// Test with offset
	filter = repository.PBIFilter{
		Offset: 3,
	}

	pbis, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 2 {
		t.Errorf("Expected 2 PBIs with offset=3, got %d", len(pbis))
	}

	// Test with both limit and offset
	filter = repository.PBIFilter{
		Limit:  2,
		Offset: 1,
	}

	pbis, err = repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 2 {
		t.Errorf("Expected 2 PBIs with limit=2 and offset=1, got %d", len(pbis))
	}
}

func TestPBIRepository_FindByEPICID(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBIs with same parent EPIC
	epicID := model.NewTaskID()

	for i := 0; i < 3; i++ {
		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", &epicID, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	// Find PBIs by EPIC ID
	pbis, err := repo.FindByEPICID(ctx, repository.EPICID(epicID.String()))
	if err != nil {
		t.Fatalf("Failed to find PBIs by EPIC ID: %v", err)
	}

	if len(pbis) != 3 {
		t.Errorf("Expected 3 PBIs for EPIC, got %d", len(pbis))
	}

	for _, p := range pbis {
		if p.ParentTaskID().String() != epicID.String() {
			t.Errorf("Expected parent EPIC ID %s, got %s", epicID.String(), p.ParentTaskID().String())
		}
	}
}

func TestPBIRepository_FindByEPICIDNotFound(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Try to find PBIs by non-existent EPIC ID
	pbis, err := repo.FindByEPICID(ctx, repository.EPICID("non-existent-epic"))
	if err != nil {
		t.Fatalf("Expected no error for non-existent EPIC, got %v", err)
	}

	if len(pbis) != 0 {
		t.Errorf("Expected 0 PBIs for non-existent EPIC, got %d", len(pbis))
	}
}

func TestPBIRepository_FindBySBIID(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with an SBI
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	sbiID := model.NewTaskID()
	err = p.AddSBI(sbiID)
	if err != nil {
		t.Fatalf("Failed to add SBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Find PBI by SBI ID
	found, err := repo.FindBySBIID(ctx, repository.SBIID(sbiID.String()))
	if err != nil {
		t.Fatalf("Failed to find PBI by SBI ID: %v", err)
	}

	if found.ID().String() != p.ID().String() {
		t.Errorf("Expected PBI ID %s, got %s", p.ID().String(), found.ID().String())
	}

	// Verify the SBI is in the PBI
	sbiIDs := found.SBIIDs()
	foundSBI := false
	for _, id := range sbiIDs {
		if id.String() == sbiID.String() {
			foundSBI = true
			break
		}
	}

	if !foundSBI {
		t.Error("SBI not found in PBI")
	}
}

func TestPBIRepository_FindBySBIIDNotFound(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Try to find PBI by non-existent SBI ID
	_, err := repo.FindBySBIID(ctx, repository.SBIID("non-existent-sbi"))
	if err == nil {
		t.Error("Expected error when finding PBI by non-existent SBI ID")
	}

	if !errors.Is(err, ErrPBINotFound) {
		t.Errorf("Expected ErrPBINotFound, got %v", err)
	}
}

func TestPBIRepository_UpdatePBI(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create and save a PBI
	p, err := pbi.NewPBI("Original Title", "Original Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Update the PBI
	err = p.UpdateTitle("Updated Title")
	if err != nil {
		t.Fatalf("Failed to update title: %v", err)
	}

	p.UpdateDescription("Updated Description")

	// Save the updated PBI
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save updated PBI: %v", err)
	}

	// Verify the update
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if found.Title() != "Updated Title" {
		t.Errorf("Expected title 'Updated Title', got '%s'", found.Title())
	}

	if found.Description() != "Updated Description" {
		t.Errorf("Expected description 'Updated Description', got '%s'", found.Description())
	}
}

func TestPBIRepository_Concurrency(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Test concurrent saves
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			p, err := pbi.NewPBI("PBI "+string(rune('A'+index)), "Description", nil, pbi.PBIMetadata{})
			if err != nil {
				t.Errorf("Failed to create PBI: %v", err)
				return
			}

			err = repo.Save(ctx, p)
			if err != nil {
				t.Errorf("Failed to save PBI: %v", err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all PBIs were saved
	pbis, err := repo.List(ctx, repository.PBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 10 {
		t.Errorf("Expected 10 PBIs after concurrent saves, got %d", len(pbis))
	}
}

func TestPBIRepository_EmptyList(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// List from empty repository
	pbis, err := repo.List(ctx, repository.PBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 0 {
		t.Errorf("Expected 0 PBIs from empty repository, got %d", len(pbis))
	}
}

// Additional Tests - Edge Cases and Complex Scenarios

func TestPBIRepository_SaveAndOverwrite(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create and save a PBI
	p, err := pbi.NewPBI("Original Title", "Original Description", nil, pbi.PBIMetadata{
		Priority: 1,
	})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Modify and save again (overwrite)
	err = p.UpdateTitle("Modified Title")
	if err != nil {
		t.Fatalf("Failed to update title: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to overwrite PBI: %v", err)
	}

	// Verify only one PBI exists with updated data
	allPBIs, err := repo.List(ctx, repository.PBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(allPBIs) != 1 {
		t.Errorf("Expected 1 PBI after overwrite, got %d", len(allPBIs))
	}

	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if found.Title() != "Modified Title" {
		t.Errorf("Expected title 'Modified Title', got '%s'", found.Title())
	}
}

func TestPBIRepository_MultipleSBIManagement(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI and add multiple SBIs
	p, err := pbi.NewPBI("Multi-SBI PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	sbiIDs := make([]model.TaskID, 3)
	for i := 0; i < 3; i++ {
		sbiIDs[i] = model.NewTaskID()
		err = p.AddSBI(sbiIDs[i])
		if err != nil {
			t.Fatalf("Failed to add SBI %d: %v", i, err)
		}
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Verify all SBIs can be found
	for i, sbiID := range sbiIDs {
		found, err := repo.FindBySBIID(ctx, repository.SBIID(sbiID.String()))
		if err != nil {
			t.Errorf("Failed to find PBI by SBI %d: %v", i, err)
		}

		if found.ID().String() != p.ID().String() {
			t.Errorf("SBI %d: Expected PBI ID %s, got %s", i, p.ID().String(), found.ID().String())
		}
	}

	// Verify PBI has all SBIs
	savedPBI, err := repo.Find(ctx, repository.PBIID(p.ID().String()))
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if len(savedPBI.SBIIDs()) != 3 {
		t.Errorf("Expected 3 SBIs, got %d", len(savedPBI.SBIIDs()))
	}
}

func TestPBIRepository_SBIMappingConsistency(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with an SBI
	p, err := pbi.NewPBI("PBI 1", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	sbiID := model.NewTaskID()
	err = p.AddSBI(sbiID)
	if err != nil {
		t.Fatalf("Failed to add SBI to PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Verify SBI maps to PBI
	found, err := repo.FindBySBIID(ctx, repository.SBIID(sbiID.String()))
	if err != nil {
		t.Fatalf("Failed to find PBI by SBI ID: %v", err)
	}

	if found.ID().String() != p.ID().String() {
		t.Errorf("Expected PBI 1, got PBI %s", found.ID().String())
	}

	// Remove SBI from PBI
	err = p.RemoveSBI(sbiID)
	if err != nil {
		t.Fatalf("Failed to remove SBI from PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save updated PBI: %v", err)
	}

	// Verify PBI no longer has the SBI
	updatedPBI, err := repo.Find(ctx, repository.PBIID(p.ID().String()))
	if err != nil {
		t.Fatalf("Failed to find updated PBI: %v", err)
	}

	if len(updatedPBI.SBIIDs()) != 0 {
		t.Errorf("Expected 0 SBIs after removal, got %d", len(updatedPBI.SBIIDs()))
	}

	// Verify SBI no longer maps to any PBI
	_, err = repo.FindBySBIID(ctx, repository.SBIID(sbiID.String()))
	if err == nil {
		t.Error("Expected error when finding PBI for removed SBI")
	}

	if !errors.Is(err, ErrPBINotFound) {
		t.Errorf("Expected ErrPBINotFound, got %v", err)
	}
}

func TestPBIRepository_ListWithMultipleStatusFilters(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBIs with different statuses
	statusTransitions := [][]model.Status{
		{model.StatusPending},
		{model.StatusPending, model.StatusPicked},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
	}

	for i, transitions := range statusTransitions {
		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		for j := 1; j < len(transitions); j++ {
			err = p.UpdateStatus(transitions[j])
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	// Filter by multiple statuses
	filter := repository.PBIFilter{
		Statuses: []repository.Status{
			repository.Status(model.StatusPending.String()),
			repository.Status(model.StatusPicked.String()),
		},
	}

	pbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 2 {
		t.Errorf("Expected 2 PBIs (PENDING + PICKED), got %d", len(pbis))
	}

	// Verify statuses
	for _, p := range pbis {
		status := p.Status()
		if status != model.StatusPending && status != model.StatusPicked {
			t.Errorf("Unexpected status %s in filtered results", status.String())
		}
	}
}

func TestPBIRepository_PaginationEdgeCases(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create 3 PBIs
	for i := 0; i < 3; i++ {
		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	tests := []struct {
		name          string
		filter        repository.PBIFilter
		expectedCount int
		description   string
	}{
		{
			name:          "Offset exceeds total",
			filter:        repository.PBIFilter{Offset: 10},
			expectedCount: 0,
			description:   "Offset larger than total should return empty",
		},
		{
			name:          "Limit exceeds remaining",
			filter:        repository.PBIFilter{Limit: 10},
			expectedCount: 3,
			description:   "Limit larger than total should return all",
		},
		{
			name:          "Offset equals total",
			filter:        repository.PBIFilter{Offset: 3},
			expectedCount: 0,
			description:   "Offset equal to total should return empty",
		},
		{
			name:          "Limit is zero",
			filter:        repository.PBIFilter{Limit: 0},
			expectedCount: 3,
			description:   "Limit of 0 should return all",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pbis, err := repo.List(ctx, tt.filter)
			if err != nil {
				t.Fatalf("Failed to list PBIs: %v", err)
			}

			if len(pbis) != tt.expectedCount {
				t.Errorf("%s: expected %d PBIs, got %d", tt.description, tt.expectedCount, len(pbis))
			}
		})
	}
}

func TestPBIRepository_AllStatusTransitions(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Test the complete valid status transition path
	p, err := pbi.NewPBI("Status Transition Test", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	// Expected transitions: PENDING -> PICKED -> IMPLEMENTING -> REVIEWING -> DONE
	transitions := []model.Status{
		model.StatusPending,
		model.StatusPicked,
		model.StatusImplementing,
		model.StatusReviewing,
		model.StatusDone,
	}

	pbiID := repository.PBIID(p.ID().String())

	for i, expectedStatus := range transitions {
		// First transition is already set (PENDING)
		if i > 0 {
			err = p.UpdateStatus(expectedStatus)
			if err != nil {
				t.Fatalf("Failed to transition to %s: %v", expectedStatus.String(), err)
			}
		}

		// Save after each transition
		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI after transition to %s: %v", expectedStatus.String(), err)
		}

		// Verify status was persisted
		found, err := repo.Find(ctx, pbiID)
		if err != nil {
			t.Fatalf("Failed to find PBI: %v", err)
		}

		if found.Status() != expectedStatus {
			t.Errorf("Transition %d: Expected status %s, got %s", i, expectedStatus.String(), found.Status().String())
		}
	}
}

func TestPBIRepository_MetadataUpdates(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with initial metadata
	p, err := pbi.NewPBI("Metadata Test", "Description", nil, pbi.PBIMetadata{
		StoryPoints:        5,
		Priority:           3,
		Labels:             []string{"backend"},
		AssignedAgent:      "claude-code",
		AcceptanceCriteria: []string{"Criterion 1"},
	})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Update metadata
	newMetadata := pbi.PBIMetadata{
		StoryPoints:        8,
		Priority:           1,
		Labels:             []string{"frontend", "ui", "critical"},
		AssignedAgent:      "gemini-cli",
		AcceptanceCriteria: []string{"Criterion 1", "Criterion 2", "Criterion 3"},
	}
	p.UpdateMetadata(newMetadata)

	// Save updated PBI
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save updated PBI: %v", err)
	}

	// Verify updates were persisted
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	metadata := found.Metadata()
	if metadata.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", metadata.Priority)
	}

	if len(metadata.Labels) != 3 {
		t.Errorf("Expected 3 labels, got %d", len(metadata.Labels))
	}

	if metadata.AssignedAgent != "gemini-cli" {
		t.Errorf("Expected agent 'gemini-cli', got '%s'", metadata.AssignedAgent)
	}

	if metadata.StoryPoints != 8 {
		t.Errorf("Expected story points 8, got %d", metadata.StoryPoints)
	}

	if len(metadata.AcceptanceCriteria) != 3 {
		t.Errorf("Expected 3 acceptance criteria, got %d", len(metadata.AcceptanceCriteria))
	}
}

func TestPBIRepository_ConcurrentReadsAndWrites(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Pre-populate with some PBIs
	for i := 0; i < 5; i++ {
		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", nil, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 20)

	// Concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			_, err := repo.List(ctx, repository.PBIFilter{})
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

			p, err := pbi.NewPBI("New PBI "+string(rune('A'+index)), "Description", nil, pbi.PBIMetadata{})
			if err != nil {
				errorChan <- err
				return
			}

			err = repo.Save(ctx, p)
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
	pbis, err := repo.List(ctx, repository.PBIFilter{})
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	if len(pbis) != 15 {
		t.Errorf("Expected 15 PBIs after concurrent operations, got %d", len(pbis))
	}
}

func TestPBIRepository_FindEmptyID(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Try to find PBI with empty ID
	_, err := repo.Find(ctx, repository.PBIID(""))
	if err == nil {
		t.Error("Expected error when finding PBI with empty ID")
	}
}

func TestPBIRepository_ListCombinedFilters(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create an EPIC
	epicID := model.NewTaskID()

	// Create 10 PBIs with mixed statuses, some belonging to EPIC
	for i := 0; i < 10; i++ {
		var parentEPIC *model.TaskID
		if i < 5 {
			parentEPIC = &epicID
		}

		p, err := pbi.NewPBI("PBI "+string(rune('A'+i)), "Description", parentEPIC, pbi.PBIMetadata{})
		if err != nil {
			t.Fatalf("Failed to create PBI: %v", err)
		}

		// Every other PBI gets moved to PICKED status
		if i%2 == 0 {
			err = p.UpdateStatus(model.StatusPicked)
			if err != nil {
				t.Fatalf("Failed to update status: %v", err)
			}
		}

		err = repo.Save(ctx, p)
		if err != nil {
			t.Fatalf("Failed to save PBI: %v", err)
		}
	}

	// Test combined filter: EPIC + status + pagination
	epicIDFilter := repository.EPICID(epicID.String())
	filter := repository.PBIFilter{
		EPICID:   &epicIDFilter,
		Statuses: []repository.Status{repository.Status(model.StatusPicked.String())},
		Limit:    2,
		Offset:   0,
	}

	pbis, err := repo.List(ctx, filter)
	if err != nil {
		t.Fatalf("Failed to list PBIs: %v", err)
	}

	// Should have 3 PICKED PBIs in EPIC (indices 0, 2, 4), with limit 2, we get 2
	if len(pbis) != 2 {
		t.Errorf("Expected 2 PBIs with combined filter, got %d", len(pbis))
	}

	// All returned PBIs should be PICKED and belong to the EPIC
	for _, p := range pbis {
		if p.Status() != model.StatusPicked {
			t.Errorf("Expected PICKED status, got %s", p.Status().String())
		}
		if p.ParentTaskID().String() != epicID.String() {
			t.Errorf("Expected parent EPIC ID %s, got %s", epicID.String(), p.ParentTaskID().String())
		}
	}
}

func TestPBIRepository_EPICMappingConsistency(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with EPIC
	epicID1 := model.NewTaskID()
	p, err := pbi.NewPBI("PBI 1", "Description", &epicID1, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Verify PBI is found by EPIC ID
	pbis, err := repo.FindByEPICID(ctx, repository.EPICID(epicID1.String()))
	if err != nil {
		t.Fatalf("Failed to find PBIs by EPIC ID: %v", err)
	}

	if len(pbis) != 1 {
		t.Errorf("Expected 1 PBI for EPIC 1, got %d", len(pbis))
	}

	// Change parent EPIC (simulate reassignment)
	epicID2 := model.NewTaskID()
	// Note: PBI doesn't have a method to change parent EPIC directly in domain model
	// We would need to use reconstruction in a real implementation
	// For this test, we'll create a new PBI with same ID but different parent
	updatedPBI := pbi.ReconstructPBI(
		p.ID(),
		p.Title(),
		p.Description(),
		p.Status(),
		p.CurrentStep(),
		&epicID2, // New parent EPIC
		p.SBIIDs(),
		p.Metadata(),
		p.CreatedAt().Value(),
		p.UpdatedAt().Value(),
	)

	err = repo.Save(ctx, updatedPBI)
	if err != nil {
		t.Fatalf("Failed to save updated PBI: %v", err)
	}

	// Verify PBI no longer found under EPIC 1
	pbis, err = repo.FindByEPICID(ctx, repository.EPICID(epicID1.String()))
	if err != nil {
		t.Fatalf("Failed to find PBIs by EPIC ID 1: %v", err)
	}

	if len(pbis) != 0 {
		t.Errorf("Expected 0 PBIs for EPIC 1 after reassignment, got %d", len(pbis))
	}

	// Verify PBI is found under EPIC 2
	pbis, err = repo.FindByEPICID(ctx, repository.EPICID(epicID2.String()))
	if err != nil {
		t.Fatalf("Failed to find PBIs by EPIC ID 2: %v", err)
	}

	if len(pbis) != 1 {
		t.Errorf("Expected 1 PBI for EPIC 2, got %d", len(pbis))
	}

	// Verify correct PBI
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if found.ParentTaskID().String() != epicID2.String() {
		t.Errorf("Expected parent EPIC ID %s, got %s", epicID2.String(), found.ParentTaskID().String())
	}
}

// Additional tests for domain-level constraints and helper methods

func TestPBIRepository_AddDuplicateSBI(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI and add an SBI
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	sbiID := model.NewTaskID()
	err = p.AddSBI(sbiID)
	if err != nil {
		t.Fatalf("Failed to add SBI: %v", err)
	}

	// Try to add the same SBI again
	err = p.AddSBI(sbiID)
	if err == nil {
		t.Error("Expected error when adding duplicate SBI")
	}

	// Save and verify only one SBI exists
	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if len(found.SBIIDs()) != 1 {
		t.Errorf("Expected 1 SBI, got %d", len(found.SBIIDs()))
	}
}

func TestPBIRepository_RemoveNonExistentSBI(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI without any SBIs
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Try to remove a non-existent SBI
	nonExistentSBI := model.NewTaskID()
	err = p.RemoveSBI(nonExistentSBI)
	if err == nil {
		t.Error("Expected error when removing non-existent SBI")
	}
}

func TestPBIRepository_HasSBIsHelper(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI without SBIs
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	// Verify HasSBIs returns false
	if found.HasSBIs() {
		t.Error("Expected HasSBIs() to return false for PBI without SBIs")
	}

	// Add an SBI
	sbiID := model.NewTaskID()
	err = p.AddSBI(sbiID)
	if err != nil {
		t.Fatalf("Failed to add SBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Verify HasSBIs returns true
	found, err = repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if !found.HasSBIs() {
		t.Error("Expected HasSBIs() to return true for PBI with SBIs")
	}
}

func TestPBIRepository_SBICountHelper(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI and add multiple SBIs
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	// Initially count should be 0
	if p.SBICount() != 0 {
		t.Errorf("Expected SBICount() to return 0, got %d", p.SBICount())
	}

	// Add SBIs and verify count
	for i := 0; i < 5; i++ {
		sbiID := model.NewTaskID()
		err = p.AddSBI(sbiID)
		if err != nil {
			t.Fatalf("Failed to add SBI %d: %v", i, err)
		}

		expectedCount := i + 1
		if p.SBICount() != expectedCount {
			t.Errorf("After adding SBI %d: expected SBICount() = %d, got %d", i, expectedCount, p.SBICount())
		}
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Verify count after retrieval
	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if found.SBICount() != 5 {
		t.Errorf("Expected SBICount() to return 5, got %d", found.SBICount())
	}
}

func TestPBIRepository_IsCompletedHelper(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI in PENDING status
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	// Verify IsCompleted returns false for PENDING status
	if found.IsCompleted() {
		t.Error("Expected IsCompleted() to return false for PENDING status")
	}

	// Transition to DONE status
	err = p.UpdateStatus(model.StatusPicked)
	if err != nil {
		t.Fatalf("Failed to update to PICKED: %v", err)
	}
	err = p.UpdateStatus(model.StatusImplementing)
	if err != nil {
		t.Fatalf("Failed to update to IMPLEMENTING: %v", err)
	}
	err = p.UpdateStatus(model.StatusReviewing)
	if err != nil {
		t.Fatalf("Failed to update to REVIEWING: %v", err)
	}
	err = p.UpdateStatus(model.StatusDone)
	if err != nil {
		t.Fatalf("Failed to update to DONE: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	found, err = repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	// Verify IsCompleted returns true for DONE status
	if !found.IsCompleted() {
		t.Error("Expected IsCompleted() to return true for DONE status")
	}
}

func TestPBIRepository_IsFailedHelper(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI in PENDING status
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	// Verify IsFailed returns false for PENDING status
	if found.IsFailed() {
		t.Error("Expected IsFailed() to return false for PENDING status")
	}

	// Transition to FAILED status (PENDING -> PICKED -> IMPLEMENTING -> FAILED)
	err = p.UpdateStatus(model.StatusPicked)
	if err != nil {
		t.Fatalf("Failed to update to PICKED: %v", err)
	}
	err = p.UpdateStatus(model.StatusImplementing)
	if err != nil {
		t.Fatalf("Failed to update to IMPLEMENTING: %v", err)
	}
	err = p.UpdateStatus(model.StatusFailed)
	if err != nil {
		t.Fatalf("Failed to update to FAILED: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	found, err = repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	// Verify IsFailed returns true for FAILED status
	if !found.IsFailed() {
		t.Error("Expected IsFailed() to return true for FAILED status")
	}
}

func TestPBIRepository_UpdateTitleWithEmptyString(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with valid title
	p, err := pbi.NewPBI("Original Title", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	// Try to update title to empty string
	err = p.UpdateTitle("")
	if err == nil {
		t.Error("Expected error when updating title to empty string")
	}

	// Verify title remains unchanged
	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	if found.Title() != "Original Title" {
		t.Errorf("Expected title to remain 'Original Title', got '%s'", found.Title())
	}
}

func TestPBIRepository_CanDeleteWithoutSBIs(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI without SBIs
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	// Verify CanDelete returns true
	if !found.CanDelete() {
		t.Error("Expected CanDelete() to return true for PBI without SBIs")
	}

	// Verify deletion succeeds
	err = repo.Delete(ctx, pbiID)
	if err != nil {
		t.Errorf("Expected deletion to succeed, got error: %v", err)
	}
}

func TestPBIRepository_CannotDeleteWithSBIs(t *testing.T) {
	repo := NewMockPBIRepository()
	ctx := context.Background()

	// Create PBI with SBIs
	p, err := pbi.NewPBI("Test PBI", "Description", nil, pbi.PBIMetadata{})
	if err != nil {
		t.Fatalf("Failed to create PBI: %v", err)
	}

	sbiID := model.NewTaskID()
	err = p.AddSBI(sbiID)
	if err != nil {
		t.Fatalf("Failed to add SBI: %v", err)
	}

	err = repo.Save(ctx, p)
	if err != nil {
		t.Fatalf("Failed to save PBI: %v", err)
	}

	pbiID := repository.PBIID(p.ID().String())
	found, err := repo.Find(ctx, pbiID)
	if err != nil {
		t.Fatalf("Failed to find PBI: %v", err)
	}

	// Verify CanDelete returns false
	if found.CanDelete() {
		t.Error("Expected CanDelete() to return false for PBI with SBIs")
	}

	// Verify deletion fails
	err = repo.Delete(ctx, pbiID)
	if err == nil {
		t.Error("Expected deletion to fail for PBI with SBIs")
	}
}
