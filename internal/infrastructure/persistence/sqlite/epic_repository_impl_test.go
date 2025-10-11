package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// setupTestDBForEPIC creates an in-memory SQLite database for testing
func setupTestDBForEPIC(t *testing.T) *sql.DB {
	// Use a shared cache in-memory database so all connections see the same database
	// The "?cache=shared" parameter ensures multiple connections access the same in-memory database
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	require.NoError(t, err)

	// Run migrations
	migrator := NewMigrator(db)
	err = migrator.Migrate()
	require.NoError(t, err)

	return db
}

// Test Basic CRUD Operations

func TestEPICRepositoryImpl_Find(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create and save an EPIC
	e, err := epic.NewEPIC("Test EPIC", "Test Description", epic.EPICMetadata{
		EstimatedStoryPoints: 10,
		Priority:             1,
		Labels:               []string{"backend", "api"},
		AssignedAgent:        "claude-code",
	})
	require.NoError(t, err)

	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Find the EPIC
	epicID := repository.EPICID(e.ID().String())
	found, err := repo.Find(ctx, epicID)
	require.NoError(t, err)
	assert.NotNil(t, found)

	// Verify fields
	assert.Equal(t, e.ID().String(), found.ID().String())
	assert.Equal(t, "Test EPIC", found.Title())
	assert.Equal(t, "Test Description", found.Description())
	assert.Equal(t, model.StatusPending, found.Status())

	// Verify metadata
	metadata := found.Metadata()
	assert.Equal(t, 10, metadata.EstimatedStoryPoints)
	assert.Equal(t, 1, metadata.Priority)
	assert.Equal(t, []string{"backend", "api"}, metadata.Labels)
	assert.Equal(t, "claude-code", metadata.AssignedAgent)
}

func TestEPICRepositoryImpl_FindNotFound(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Try to find non-existent EPIC
	_, err := repo.Find(ctx, repository.EPICID("non-existent-id"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEPICRepositoryImpl_Save(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
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
		{
			name:        "EPIC with empty labels",
			title:       "Feature D",
			description: "Implement feature D",
			metadata: epic.EPICMetadata{
				EstimatedStoryPoints: 3,
				Priority:             3,
				Labels:               []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := epic.NewEPIC(tt.title, tt.description, tt.metadata)
			require.NoError(t, err)

			err = repo.Save(ctx, e)
			require.NoError(t, err)

			// Verify saved EPIC
			epicID := repository.EPICID(e.ID().String())
			found, err := repo.Find(ctx, epicID)
			require.NoError(t, err)

			assert.Equal(t, tt.title, found.Title())
			assert.Equal(t, tt.description, found.Description())
			assert.Equal(t, tt.metadata.EstimatedStoryPoints, found.Metadata().EstimatedStoryPoints)
			assert.Equal(t, tt.metadata.Priority, found.Metadata().Priority)
			assert.Equal(t, tt.metadata.Labels, found.Metadata().Labels)
			assert.Equal(t, tt.metadata.AssignedAgent, found.Metadata().AssignedAgent)
		})
	}
}

func TestEPICRepositoryImpl_SaveAndUpdate(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create and save an EPIC
	e, err := epic.NewEPIC("Original Title", "Original Description", epic.EPICMetadata{
		Priority: 1,
		Labels:   []string{"backend"},
	})
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Update the EPIC
	err = e.UpdateTitle("Updated Title")
	require.NoError(t, err)
	e.UpdateDescription("Updated Description")

	// Save again (update)
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Verify update
	found, err := repo.Find(ctx, epicID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", found.Title())
	assert.Equal(t, "Updated Description", found.Description())

	// Verify no duplicate was created
	allEPICs, err := repo.List(ctx, repository.EPICFilter{})
	require.NoError(t, err)
	assert.Len(t, allEPICs, 1)
}

func TestEPICRepositoryImpl_Delete(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create and save an EPIC without PBIs
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Delete the EPIC
	err = repo.Delete(ctx, epicID)
	require.NoError(t, err)

	// Verify EPIC is deleted
	_, err = repo.Find(ctx, epicID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEPICRepositoryImpl_DeleteNotFound(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Try to delete non-existent EPIC
	err := repo.Delete(ctx, repository.EPICID("non-existent-id"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Test List and Filtering

func TestEPICRepositoryImpl_List(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create multiple EPICs
	for i := 0; i < 3; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{
			Priority: i + 1,
		})
		require.NoError(t, err)
		err = repo.Save(ctx, e)
		require.NoError(t, err)
	}

	// List all EPICs
	allEPICs, err := repo.List(ctx, repository.EPICFilter{})
	require.NoError(t, err)
	assert.Len(t, allEPICs, 3)
}

func TestEPICRepositoryImpl_ListWithStatusFilter(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPICs with different statuses
	statusTransitions := [][]model.Status{
		{model.StatusPending},
		{model.StatusPending, model.StatusPicked},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing, model.StatusReviewing, model.StatusDone},
	}

	for i, transitions := range statusTransitions {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		require.NoError(t, err)

		// Apply status transitions in order
		for j := 1; j < len(transitions); j++ {
			err = e.UpdateStatus(transitions[j])
			require.NoError(t, err)
		}

		err = repo.Save(ctx, e)
		require.NoError(t, err)
	}

	// Filter by DONE status
	filter := repository.EPICFilter{
		Statuses: []repository.Status{repository.Status(model.StatusDone.String())},
	}

	doneEPICs, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, doneEPICs, 2)

	for _, e := range doneEPICs {
		assert.Equal(t, model.StatusDone, e.Status())
	}
}

func TestEPICRepositoryImpl_ListWithMultipleStatusFilters(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPICs with different statuses
	statusTransitions := [][]model.Status{
		{model.StatusPending},
		{model.StatusPending, model.StatusPicked},
		{model.StatusPending, model.StatusPicked, model.StatusImplementing},
	}

	for i, transitions := range statusTransitions {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		require.NoError(t, err)

		for j := 1; j < len(transitions); j++ {
			err = e.UpdateStatus(transitions[j])
			require.NoError(t, err)
		}

		err = repo.Save(ctx, e)
		require.NoError(t, err)
	}

	// Filter by multiple statuses
	filter := repository.EPICFilter{
		Statuses: []repository.Status{
			repository.Status(model.StatusPending.String()),
			repository.Status(model.StatusPicked.String()),
		},
	}

	epics, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, epics, 2)
}

func TestEPICRepositoryImpl_ListWithPagination(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create 5 EPICs
	for i := 0; i < 5; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		require.NoError(t, err)
		err = repo.Save(ctx, e)
		require.NoError(t, err)
	}

	// Test with limit
	filter := repository.EPICFilter{Limit: 2}
	epics, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, epics, 2)

	// Test with offset and large limit (to get remaining items)
	filter = repository.EPICFilter{Offset: 3, Limit: 100}
	epics, err = repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, epics, 2)

	// Test with both limit and offset
	filter = repository.EPICFilter{Limit: 2, Offset: 1}
	epics, err = repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, epics, 2)
}

func TestEPICRepositoryImpl_ListEmpty(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// List from empty database
	epics, err := repo.List(ctx, repository.EPICFilter{})
	require.NoError(t, err)
	assert.Empty(t, epics)
}

// Test PBI Relationships

func TestEPICRepositoryImpl_SaveWithPBIs(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC with PBIs
	e, err := epic.NewEPIC("EPIC with PBIs", "Description", epic.EPICMetadata{})
	require.NoError(t, err)

	// Add PBIs
	pbiID1 := model.NewTaskID()
	pbiID2 := model.NewTaskID()
	pbiID3 := model.NewTaskID()

	err = e.AddPBI(pbiID1)
	require.NoError(t, err)
	err = e.AddPBI(pbiID2)
	require.NoError(t, err)
	err = e.AddPBI(pbiID3)
	require.NoError(t, err)

	// Save EPIC
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Find and verify PBIs
	epicID := repository.EPICID(e.ID().String())
	found, err := repo.Find(ctx, epicID)
	require.NoError(t, err)

	pbiIDs := found.PBIIDs()
	assert.Len(t, pbiIDs, 3)

	// Verify PBI IDs match
	pbiIDStrings := make([]string, len(pbiIDs))
	for i, id := range pbiIDs {
		pbiIDStrings[i] = id.String()
	}
	assert.Contains(t, pbiIDStrings, pbiID1.String())
	assert.Contains(t, pbiIDStrings, pbiID2.String())
	assert.Contains(t, pbiIDStrings, pbiID3.String())
}

func TestEPICRepositoryImpl_FindByPBIID(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC with a PBI
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	require.NoError(t, err)

	pbiID := model.NewTaskID()
	err = e.AddPBI(pbiID)
	require.NoError(t, err)

	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Find EPIC by PBI ID
	found, err := repo.FindByPBIID(ctx, repository.PBIID(pbiID.String()))
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, e.ID().String(), found.ID().String())
}

func TestEPICRepositoryImpl_FindByPBIIDNotFound(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Try to find EPIC by non-existent PBI ID
	_, err := repo.FindByPBIID(ctx, repository.PBIID("non-existent-pbi"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestEPICRepositoryImpl_UpdatePBIRelationships(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC with initial PBIs
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	require.NoError(t, err)

	pbiID1 := model.NewTaskID()
	pbiID2 := model.NewTaskID()
	err = e.AddPBI(pbiID1)
	require.NoError(t, err)
	err = e.AddPBI(pbiID2)
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Remove one PBI and add a new one
	err = e.RemovePBI(pbiID1)
	require.NoError(t, err)

	pbiID3 := model.NewTaskID()
	err = e.AddPBI(pbiID3)
	require.NoError(t, err)

	// Save updated EPIC
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Verify updated relationships
	found, err := repo.Find(ctx, epicID)
	require.NoError(t, err)

	pbiIDs := found.PBIIDs()
	assert.Len(t, pbiIDs, 2)

	pbiIDStrings := make([]string, len(pbiIDs))
	for i, id := range pbiIDs {
		pbiIDStrings[i] = id.String()
	}
	assert.NotContains(t, pbiIDStrings, pbiID1.String())
	assert.Contains(t, pbiIDStrings, pbiID2.String())
	assert.Contains(t, pbiIDStrings, pbiID3.String())

	// Verify old PBI mapping is gone
	_, err = repo.FindByPBIID(ctx, repository.PBIID(pbiID1.String()))
	assert.Error(t, err)
}

// Test Status Transitions

func TestEPICRepositoryImpl_StatusTransitions(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC and test all valid status transitions
	e, err := epic.NewEPIC("Status Test", "Description", epic.EPICMetadata{})
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())

	transitions := []model.Status{
		model.StatusPending,
		model.StatusPicked,
		model.StatusImplementing,
		model.StatusReviewing,
		model.StatusDone,
	}

	for i, expectedStatus := range transitions {
		if i > 0 {
			err = e.UpdateStatus(expectedStatus)
			require.NoError(t, err)
		}

		err = repo.Save(ctx, e)
		require.NoError(t, err)

		found, err := repo.Find(ctx, epicID)
		require.NoError(t, err)
		assert.Equal(t, expectedStatus, found.Status())
	}
}

// Test Metadata Updates

func TestEPICRepositoryImpl_MetadataUpdates(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC with initial metadata
	e, err := epic.NewEPIC("Metadata Test", "Description", epic.EPICMetadata{
		EstimatedStoryPoints: 5,
		Priority:             3,
		Labels:               []string{"backend"},
		AssignedAgent:        "claude-code",
	})
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Update metadata
	newMetadata := epic.EPICMetadata{
		EstimatedStoryPoints: 8,
		Priority:             1,
		Labels:               []string{"frontend", "ui", "critical"},
		AssignedAgent:        "gemini-cli",
	}
	e.UpdateMetadata(newMetadata)

	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Verify updates
	found, err := repo.Find(ctx, epicID)
	require.NoError(t, err)

	metadata := found.Metadata()
	assert.Equal(t, 8, metadata.EstimatedStoryPoints)
	assert.Equal(t, 1, metadata.Priority)
	assert.Equal(t, []string{"frontend", "ui", "critical"}, metadata.Labels)
	assert.Equal(t, "gemini-cli", metadata.AssignedAgent)
}

// Test Edge Cases

func TestEPICRepositoryImpl_EmptyDescription(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC with empty description
	e, err := epic.NewEPIC("Test EPIC", "", epic.EPICMetadata{})
	require.NoError(t, err)

	err = repo.Save(ctx, e)
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())
	found, err := repo.Find(ctx, epicID)
	require.NoError(t, err)
	assert.Equal(t, "", found.Description())
}

func TestEPICRepositoryImpl_NilLabels(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC with nil labels
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{
		Labels: nil,
	})
	require.NoError(t, err)

	err = repo.Save(ctx, e)
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())
	found, err := repo.Find(ctx, epicID)
	require.NoError(t, err)
	assert.Empty(t, found.Metadata().Labels)
}

func TestEPICRepositoryImpl_DeleteWithPBIs(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create EPIC with PBIs
	e, err := epic.NewEPIC("Test EPIC", "Description", epic.EPICMetadata{})
	require.NoError(t, err)

	pbiID := model.NewTaskID()
	err = e.AddPBI(pbiID)
	require.NoError(t, err)

	epicID := repository.EPICID(e.ID().String())
	err = repo.Save(ctx, e)
	require.NoError(t, err)

	// Delete should succeed at DB level (constraint checking is done in domain)
	// But let's verify the PBI relationships are cleaned up
	err = repo.Delete(ctx, epicID)
	require.NoError(t, err)

	// Verify EPIC is gone
	_, err = repo.Find(ctx, epicID)
	assert.Error(t, err)

	// Verify PBI mapping is also gone
	_, err = repo.FindByPBIID(ctx, repository.PBIID(pbiID.String()))
	assert.Error(t, err)
}

func TestEPICRepositoryImpl_CombinedFilters(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create 10 EPICs with mixed statuses
	for i := 0; i < 10; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		require.NoError(t, err)

		// Every other EPIC gets moved to PICKED status
		if i%2 == 0 {
			err = e.UpdateStatus(model.StatusPicked)
			require.NoError(t, err)
		}

		err = repo.Save(ctx, e)
		require.NoError(t, err)
	}

	// Test combined filter: status + pagination
	filter := repository.EPICFilter{
		Statuses: []repository.Status{repository.Status(model.StatusPicked.String())},
		Limit:    2,
		Offset:   1,
	}

	epics, err := repo.List(ctx, filter)
	require.NoError(t, err)
	assert.Len(t, epics, 2)

	// All returned EPICs should be PICKED
	for _, e := range epics {
		assert.Equal(t, model.StatusPicked, e.Status())
	}
}

func TestEPICRepositoryImpl_PaginationBoundaries(t *testing.T) {
	db := setupTestDBForEPIC(t)
	defer db.Close()

	repo := NewEPICRepository(db)
	ctx := context.Background()

	// Create 3 EPICs
	for i := 0; i < 3; i++ {
		e, err := epic.NewEPIC("EPIC "+string(rune('A'+i)), "Description", epic.EPICMetadata{})
		require.NoError(t, err)
		err = repo.Save(ctx, e)
		require.NoError(t, err)
	}

	tests := []struct {
		name          string
		filter        repository.EPICFilter
		expectedCount int
	}{
		{
			name:          "Offset exceeds total with limit",
			filter:        repository.EPICFilter{Offset: 10, Limit: 5},
			expectedCount: 0,
		},
		{
			name:          "Limit exceeds remaining",
			filter:        repository.EPICFilter{Limit: 10},
			expectedCount: 3,
		},
		{
			name:          "Offset equals total with limit",
			filter:        repository.EPICFilter{Offset: 3, Limit: 5},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			epics, err := repo.List(ctx, tt.filter)
			require.NoError(t, err)
			assert.Len(t, epics, tt.expectedCount)
		})
	}
}
