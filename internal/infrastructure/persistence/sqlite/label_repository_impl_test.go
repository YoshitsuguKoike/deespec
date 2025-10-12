package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appconfig "github.com/YoshitsuguKoike/deespec/internal/app/config"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
)

// setupTestDBForLabel creates an in-memory SQLite database for testing
func setupTestDBForLabel(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	require.NoError(t, err)

	// Run migrations
	migrator := NewMigrator(db)
	err = migrator.Migrate()
	require.NoError(t, err)

	return db
}

// setupTempDir creates a temporary directory for template files
func setupTempDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "label_test_*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})
	return tmpDir
}

// createTestFile creates a test file with content
func createTestFile(t *testing.T, dir, filename, content string) string {
	filePath := filepath.Join(dir, filename)
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)
	return filePath
}

// Test Basic CRUD Operations

func TestLabelRepositoryImpl_Save(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	tests := []struct {
		name       string
		label      *label.Label
		wantErr    bool
		checkIDSet bool
	}{
		{
			name:       "Basic label",
			label:      label.NewLabel("feature", "Feature label", []string{}, 10),
			wantErr:    false,
			checkIDSet: true,
		},
		{
			name:       "Label with template paths",
			label:      label.NewLabel("bug", "Bug label", []string{"templates/bug.md"}, 5),
			wantErr:    false,
			checkIDSet: true,
		},
		{
			name:       "Label with high priority",
			label:      label.NewLabel("critical", "Critical issues", []string{}, 100),
			wantErr:    false,
			checkIDSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Save(ctx, tt.label)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.checkIDSet {
					assert.NotZero(t, tt.label.ID())
				}
			}
		})
	}
}

func TestLabelRepositoryImpl_FindByID(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create and save a label
	lbl := label.NewLabel("test-label", "Test description", []string{"path/to/template.md"}, 10)
	lbl.SetColor("#FF0000")
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Find the label
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.NotNil(t, found)

	// Verify fields
	assert.Equal(t, lbl.ID(), found.ID())
	assert.Equal(t, "test-label", found.Name())
	assert.Equal(t, "Test description", found.Description())
	assert.Equal(t, []string{"path/to/template.md"}, found.TemplatePaths())
	assert.Equal(t, 10, found.Priority())
	assert.Equal(t, "#FF0000", found.Color())
	assert.True(t, found.IsActive())
}

func TestLabelRepositoryImpl_FindByIDNotFound(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Try to find non-existent label
	_, err := repo.FindByID(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLabelRepositoryImpl_FindByName(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create and save a label
	lbl := label.NewLabel("unique-name", "Description", []string{}, 5)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Find by name
	found, err := repo.FindByName(ctx, "unique-name")
	require.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, lbl.ID(), found.ID())
	assert.Equal(t, "unique-name", found.Name())
}

func TestLabelRepositoryImpl_FindByNameNotFound(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Try to find non-existent label
	_, err := repo.FindByName(ctx, "non-existent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLabelRepositoryImpl_Update(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create and save a label
	lbl := label.NewLabel("original", "Original description", []string{}, 5)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Update the label
	lbl.SetDescription("Updated description")
	lbl.SetPriority(20)
	lbl.SetColor("#00FF00")
	lbl.Deactivate()

	err = repo.Update(ctx, lbl)
	require.NoError(t, err)

	// Verify update
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, "Updated description", found.Description())
	assert.Equal(t, 20, found.Priority())
	assert.Equal(t, "#00FF00", found.Color())
	assert.False(t, found.IsActive())
}

func TestLabelRepositoryImpl_Delete(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create and save a label
	lbl := label.NewLabel("to-delete", "Description", []string{}, 5)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)
	labelID := lbl.ID()

	// Delete the label
	err = repo.Delete(ctx, labelID)
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.FindByID(ctx, labelID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// Test Query Operations

func TestLabelRepositoryImpl_FindAll(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create multiple labels
	labels := []*label.Label{
		label.NewLabel("label-a", "Description A", []string{}, 10),
		label.NewLabel("label-b", "Description B", []string{}, 20),
		label.NewLabel("label-c", "Description C", []string{}, 5),
	}

	for _, lbl := range labels {
		err := repo.Save(ctx, lbl)
		require.NoError(t, err)
	}

	// Find all labels
	found, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, found, 3)

	// Verify ordering (by priority DESC, name ASC)
	// label-b (20) should be first, then label-a (10), then label-c (5)
	assert.Equal(t, "label-b", found[0].Name())
	assert.Equal(t, "label-a", found[1].Name())
	assert.Equal(t, "label-c", found[2].Name())
}

func TestLabelRepositoryImpl_FindActive(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create labels with different active states
	activeLabel := label.NewLabel("active", "Active label", []string{}, 10)
	inactiveLabel := label.NewLabel("inactive", "Inactive label", []string{}, 5)
	inactiveLabel.Deactivate()

	err := repo.Save(ctx, activeLabel)
	require.NoError(t, err)
	err = repo.Save(ctx, inactiveLabel)
	require.NoError(t, err)

	// Find only active labels
	found, err := repo.FindActive(ctx)
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, "active", found[0].Name())
	assert.True(t, found[0].IsActive())
}

func TestLabelRepositoryImpl_FindAllEmpty(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Find all from empty database
	found, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, found)
}

// Test Hierarchical Operations

func TestLabelRepositoryImpl_FindChildren(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create parent label
	parent := label.NewLabel("parent", "Parent label", []string{}, 10)
	err := repo.Save(ctx, parent)
	require.NoError(t, err)

	// Create child labels
	parentID := parent.ID()
	child1 := label.NewLabel("child1", "Child 1", []string{}, 5)
	child1.SetParentLabelID(&parentID)
	err = repo.Save(ctx, child1)
	require.NoError(t, err)

	child2 := label.NewLabel("child2", "Child 2", []string{}, 5)
	child2.SetParentLabelID(&parentID)
	err = repo.Save(ctx, child2)
	require.NoError(t, err)

	// Find children
	children, err := repo.FindChildren(ctx, parent.ID())
	require.NoError(t, err)
	assert.Len(t, children, 2)

	// Verify children names (ordered by name ASC)
	names := []string{children[0].Name(), children[1].Name()}
	assert.Contains(t, names, "child1")
	assert.Contains(t, names, "child2")
}

func TestLabelRepositoryImpl_FindByParentID(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create root label (no parent)
	root := label.NewLabel("root", "Root label", []string{}, 10)
	err := repo.Save(ctx, root)
	require.NoError(t, err)

	// Create child label
	child := label.NewLabel("child", "Child label", []string{}, 5)
	parentID := root.ID()
	child.SetParentLabelID(&parentID)
	err = repo.Save(ctx, child)
	require.NoError(t, err)

	// Find root labels (nil parent)
	rootLabels, err := repo.FindByParentID(ctx, nil)
	require.NoError(t, err)
	assert.Len(t, rootLabels, 1)
	assert.Equal(t, "root", rootLabels[0].Name())

	// Find labels by specific parent ID
	childLabels, err := repo.FindByParentID(ctx, &parentID)
	require.NoError(t, err)
	assert.Len(t, childLabels, 1)
	assert.Equal(t, "child", childLabels[0].Name())
}

// Test Task Association Operations
