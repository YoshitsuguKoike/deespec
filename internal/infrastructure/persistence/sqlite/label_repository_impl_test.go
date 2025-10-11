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
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
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

func TestLabelRepositoryImpl_AttachToTask(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create a label
	lbl := label.NewLabel("task-label", "Label for task", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Attach to task
	taskID := "test-task-123"
	err = repo.AttachToTask(ctx, taskID, lbl.ID(), 0)
	require.NoError(t, err)

	// Verify attachment by finding labels for task
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	require.NoError(t, err)
	assert.Len(t, labels, 1)
	assert.Equal(t, lbl.ID(), labels[0].ID())
}

func TestLabelRepositoryImpl_AttachToTaskMultiple(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create multiple labels
	label1 := label.NewLabel("label1", "Label 1", []string{}, 10)
	label2 := label.NewLabel("label2", "Label 2", []string{}, 20)
	err := repo.Save(ctx, label1)
	require.NoError(t, err)
	err = repo.Save(ctx, label2)
	require.NoError(t, err)

	// Attach to same task with different positions
	taskID := "task-456"
	err = repo.AttachToTask(ctx, taskID, label1.ID(), 1)
	require.NoError(t, err)
	err = repo.AttachToTask(ctx, taskID, label2.ID(), 0)
	require.NoError(t, err)

	// Verify order (position 0 should come first)
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	require.NoError(t, err)
	assert.Len(t, labels, 2)
	assert.Equal(t, label2.ID(), labels[0].ID()) // position 0
	assert.Equal(t, label1.ID(), labels[1].ID()) // position 1
}

func TestLabelRepositoryImpl_DetachFromTask(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create and attach a label
	lbl := label.NewLabel("detach-test", "Test detach", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	taskID := "task-to-detach"
	err = repo.AttachToTask(ctx, taskID, lbl.ID(), 0)
	require.NoError(t, err)

	// Detach from task
	err = repo.DetachFromTask(ctx, taskID, lbl.ID())
	require.NoError(t, err)

	// Verify detachment
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestLabelRepositoryImpl_FindLabelsByTaskID(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create labels
	labels := []*label.Label{
		label.NewLabel("label-a", "Label A", []string{}, 30),
		label.NewLabel("label-b", "Label B", []string{}, 20),
		label.NewLabel("label-c", "Label C", []string{}, 10),
	}

	taskID := "task-with-labels"
	for i, lbl := range labels {
		err := repo.Save(ctx, lbl)
		require.NoError(t, err)
		err = repo.AttachToTask(ctx, taskID, lbl.ID(), i)
		require.NoError(t, err)
	}

	// Find labels by task ID
	found, err := repo.FindLabelsByTaskID(ctx, taskID)
	require.NoError(t, err)
	assert.Len(t, found, 3)

	// Verify ordering by position
	assert.Equal(t, "label-a", found[0].Name())
	assert.Equal(t, "label-b", found[1].Name())
	assert.Equal(t, "label-c", found[2].Name())
}

func TestLabelRepositoryImpl_FindTaskIDsByLabelID(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create a label
	lbl := label.NewLabel("multi-task", "Multi-task label", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Attach to multiple tasks
	taskIDs := []string{"task-1", "task-2", "task-3"}
	for i, taskID := range taskIDs {
		err = repo.AttachToTask(ctx, taskID, lbl.ID(), i)
		require.NoError(t, err)
	}

	// Find task IDs by label ID
	found, err := repo.FindTaskIDsByLabelID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Len(t, found, 3)
	assert.Equal(t, taskIDs, found)
}

// Test Integrity Validation Operations

func TestLabelRepositoryImpl_ValidateIntegrity_OK(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	createTestFile(t, tmpDir, "template.md", "# Test Template\n\nContent here")

	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label with template
	lbl := label.NewLabel("validated", "Validated label", []string{"template.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Sync from file to calculate hash
	err = repo.SyncFromFile(ctx, lbl.ID())
	require.NoError(t, err)

	// Validate integrity
	result, err := repo.ValidateIntegrity(ctx, lbl.ID())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, repository.ValidationOK, result.Status)
	assert.Equal(t, lbl.ID(), result.LabelID)
	assert.Equal(t, "validated", result.LabelName)
}

func TestLabelRepositoryImpl_ValidateIntegrity_Modified(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	templatePath := filepath.Join(tmpDir, "template.md")

	// Create initial file
	err := os.WriteFile(templatePath, []byte("Original content"), 0644)
	require.NoError(t, err)

	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label and sync
	lbl := label.NewLabel("modified-test", "Modified test", []string{"template.md"}, 10)
	err = repo.Save(ctx, lbl)
	require.NoError(t, err)
	err = repo.SyncFromFile(ctx, lbl.ID())
	require.NoError(t, err)

	// Modify the file
	err = os.WriteFile(templatePath, []byte("Modified content"), 0644)
	require.NoError(t, err)

	// Validate integrity
	result, err := repo.ValidateIntegrity(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, repository.ValidationModified, result.Status)
	assert.NotEmpty(t, result.ExpectedHash)
	assert.NotEmpty(t, result.ActualHash)
	assert.NotEqual(t, result.ExpectedHash, result.ActualHash)
}

func TestLabelRepositoryImpl_ValidateIntegrity_Missing(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label with non-existent template
	lbl := label.NewLabel("missing-test", "Missing test", []string{"nonexistent.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Validate integrity
	result, err := repo.ValidateIntegrity(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, repository.ValidationMissing, result.Status)
}

func TestLabelRepositoryImpl_ValidateAllLabels(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	createTestFile(t, tmpDir, "template1.md", "Content 1")
	createTestFile(t, tmpDir, "template2.md", "Content 2")

	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create multiple labels
	label1 := label.NewLabel("label1", "Label 1", []string{"template1.md"}, 10)
	label2 := label.NewLabel("label2", "Label 2", []string{"template2.md"}, 10)
	label3 := label.NewLabel("label3", "Label 3", []string{"missing.md"}, 10)

	err := repo.Save(ctx, label1)
	require.NoError(t, err)
	err = repo.SyncFromFile(ctx, label1.ID())
	require.NoError(t, err)

	err = repo.Save(ctx, label2)
	require.NoError(t, err)
	err = repo.SyncFromFile(ctx, label2.ID())
	require.NoError(t, err)

	err = repo.Save(ctx, label3)
	require.NoError(t, err)

	// Validate all labels
	results, err := repo.ValidateAllLabels(ctx)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Check results
	okCount := 0
	missingCount := 0
	for _, result := range results {
		if result.Status == repository.ValidationOK {
			okCount++
		} else if result.Status == repository.ValidationMissing {
			missingCount++
		}
	}
	assert.Equal(t, 2, okCount)
	assert.Equal(t, 1, missingCount)
}

func TestLabelRepositoryImpl_SyncFromFile(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	content := "# Test Template\n\nLine 1\nLine 2\nLine 3"
	createTestFile(t, tmpDir, "sync-test.md", content)

	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label
	lbl := label.NewLabel("sync-label", "Sync test", []string{"sync-test.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Sync from file
	err = repo.SyncFromFile(ctx, lbl.ID())
	require.NoError(t, err)

	// Verify hash and line count are set
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.NotZero(t, found.LineCount())
	// Content "# Test Template\n\nLine 1\nLine 2\nLine 3" has 5 lines
	assert.Equal(t, 5, found.LineCount())

	hash, exists := found.GetContentHash("sync-test.md")
	assert.True(t, exists)
	assert.NotEmpty(t, hash)
}

func TestLabelRepositoryImpl_SyncFromFileMultipleTemplates(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	createTestFile(t, tmpDir, "template1.md", "Line 1\nLine 2")
	createTestFile(t, tmpDir, "template2.md", "Line 1\nLine 2\nLine 3")

	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label with multiple templates
	lbl := label.NewLabel("multi-sync", "Multi-template sync", []string{"template1.md", "template2.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Sync from files
	err = repo.SyncFromFile(ctx, lbl.ID())
	require.NoError(t, err)

	// Verify total line count
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, 5, found.LineCount()) // 2 + 3 lines

	// Verify both hashes are set
	hash1, exists1 := found.GetContentHash("template1.md")
	hash2, exists2 := found.GetContentHash("template2.md")
	assert.True(t, exists1)
	assert.True(t, exists2)
	assert.NotEmpty(t, hash1)
	assert.NotEmpty(t, hash2)
	assert.NotEqual(t, hash1, hash2)
}

// Test Edge Cases

func TestLabelRepositoryImpl_EmptyTemplatePaths(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label with empty template paths
	lbl := label.NewLabel("no-templates", "No templates", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Find and verify
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Empty(t, found.TemplatePaths())
}

func TestLabelRepositoryImpl_UpdatePositionOnReattach(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label
	lbl := label.NewLabel("reattach-test", "Reattach test", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	taskID := "task-reattach"

	// Attach with position 0
	err = repo.AttachToTask(ctx, taskID, lbl.ID(), 0)
	require.NoError(t, err)

	// Re-attach with position 5 (should update)
	err = repo.AttachToTask(ctx, taskID, lbl.ID(), 5)
	require.NoError(t, err)

	// Verify only one attachment exists
	// This is tested indirectly through FindLabelsByTaskID
	labels, err := repo.FindLabelsByTaskID(ctx, taskID)
	require.NoError(t, err)
	assert.Len(t, labels, 1)
}

func TestLabelRepositoryImpl_NilParentLabelID(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label with nil parent
	lbl := label.NewLabel("root-label", "Root label", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Verify parent is nil
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Nil(t, found.ParentLabelID())
}

func TestLabelRepositoryImpl_ContentHashesPersistence(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label and set content hashes manually
	lbl := label.NewLabel("hash-test", "Hash test", []string{"file1.md", "file2.md"}, 10)
	lbl.SetContentHash("file1.md", "abc123")
	lbl.SetContentHash("file2.md", "def456")
	lbl.SetLineCount(100)

	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Verify persistence
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)

	hash1, exists1 := found.GetContentHash("file1.md")
	hash2, exists2 := found.GetContentHash("file2.md")

	assert.True(t, exists1)
	assert.True(t, exists2)
	assert.Equal(t, "abc123", hash1)
	assert.Equal(t, "def456", hash2)
	assert.Equal(t, 100, found.LineCount())
}

func TestLabelRepositoryImpl_FindChildrenEmpty(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label without children
	parent := label.NewLabel("parent-no-children", "Parent", []string{}, 10)
	err := repo.Save(ctx, parent)
	require.NoError(t, err)

	// Find children
	children, err := repo.FindChildren(ctx, parent.ID())
	require.NoError(t, err)
	assert.Empty(t, children)
}

func TestLabelRepositoryImpl_MetadataField(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label (metadata is empty by default)
	lbl := label.NewLabel("metadata-test", "Metadata test", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Verify metadata is empty
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, "", found.Metadata())
}

// Test Transaction Context Support

func TestLabelRepositoryImpl_SaveWithTransaction(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	// Create transaction context using the same key structure
	type txKey struct{}
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Save label within transaction
	lbl := label.NewLabel("tx-label", "Transaction label", []string{}, 10)
	err = repo.Save(txCtx, lbl)
	require.NoError(t, err)
	assert.NotZero(t, lbl.ID())

	// Commit transaction
	err = tx.Commit()
	require.NoError(t, err)

	// Verify label was saved
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, "tx-label", found.Name())
}

func TestLabelRepositoryImpl_QueryWithTransaction(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create a label outside transaction
	lbl := label.NewLabel("query-tx", "Query transaction test", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	// Create transaction context
	type txKey struct{}
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Query operations should work within transaction
	found, err := repo.FindByID(txCtx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, "query-tx", found.Name())

	// FindAll should work within transaction
	all, err := repo.FindAll(txCtx)
	require.NoError(t, err)
	assert.NotEmpty(t, all)

	// Commit
	err = tx.Commit()
	require.NoError(t, err)
}

func TestLabelRepositoryImpl_UpdateWithTransaction(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label outside transaction
	lbl := label.NewLabel("update-tx", "Original description", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	type txKey struct{}
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Update within transaction
	lbl.SetDescription("Updated in transaction")
	err = repo.Update(txCtx, lbl)
	require.NoError(t, err)

	// Commit
	err = tx.Commit()
	require.NoError(t, err)

	// Verify update
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, "Updated in transaction", found.Description())
}

func TestLabelRepositoryImpl_DeleteWithTransaction(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create label
	lbl := label.NewLabel("delete-tx", "Delete in transaction", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)
	labelID := lbl.ID()

	// Begin transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	type txKey struct{}
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Delete within transaction
	err = repo.Delete(txCtx, labelID)
	require.NoError(t, err)

	// Commit
	err = tx.Commit()
	require.NoError(t, err)

	// Verify deletion
	_, err = repo.FindByID(ctx, labelID)
	assert.Error(t, err)
}

// Test Error Paths and Edge Cases

func TestLabelRepositoryImpl_FindAllQueryError(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Close database to simulate query error
	db.Close()

	// Attempt to find all labels
	_, err := repo.FindAll(ctx)
	assert.Error(t, err)
}

func TestLabelRepositoryImpl_FindActiveQueryError(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Close database to simulate query error
	db.Close()

	// Attempt to find active labels
	_, err := repo.FindActive(ctx)
	assert.Error(t, err)
}

func TestLabelRepositoryImpl_FindChildrenQueryError(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Close database to simulate query error
	db.Close()

	// Attempt to find children
	_, err := repo.FindChildren(ctx, 1)
	assert.Error(t, err)
}

func TestLabelRepositoryImpl_FindByParentIDQueryError(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Close database to simulate query error
	db.Close()

	// Attempt to find by parent ID
	_, err := repo.FindByParentID(ctx, nil)
	assert.Error(t, err)
}

func TestLabelRepositoryImpl_FindLabelsByTaskIDQueryError(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Close database to simulate query error
	db.Close()

	// Attempt to find labels by task ID
	_, err := repo.FindLabelsByTaskID(ctx, "task-123")
	assert.Error(t, err)
}

func TestLabelRepositoryImpl_FindTaskIDsByLabelIDQueryError(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Close database to simulate query error
	db.Close()

	// Attempt to find task IDs by label ID
	_, err := repo.FindTaskIDsByLabelID(ctx, 1)
	assert.Error(t, err)
}

func TestLabelRepositoryImpl_ValidateIntegrityLabelNotFound(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Attempt to validate non-existent label
	_, err := repo.ValidateIntegrity(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "find label failed")
}

func TestLabelRepositoryImpl_SyncFromFileLabelNotFound(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Attempt to sync non-existent label
	err := repo.SyncFromFile(ctx, 99999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "find label failed")
}

func TestLabelRepositoryImpl_SyncFromFileTemplateNotFound(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label with non-existent template
	lbl := label.NewLabel("sync-error", "Sync error test", []string{"nonexistent.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Attempt to sync from non-existent file
	err = repo.SyncFromFile(ctx, lbl.ID())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "resolve template path")
}

func TestLabelRepositoryImpl_ValidateAllLabelsQueryError(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Close database to simulate query error
	db.Close()

	// Attempt to validate all labels
	_, err := repo.ValidateAllLabels(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "find all labels failed")
}

// Test Special Cases

func TestLabelRepositoryImpl_EmptyTaskLabels(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Find labels for non-existent task
	labels, err := repo.FindLabelsByTaskID(ctx, "non-existent-task")
	require.NoError(t, err)
	assert.Empty(t, labels)
}

func TestLabelRepositoryImpl_EmptyLabelTasks(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config)
	ctx := context.Background()

	// Create a label but don't attach it to any tasks
	lbl := label.NewLabel("no-tasks", "Label with no tasks", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Find task IDs for this label
	taskIDs, err := repo.FindTaskIDsByLabelID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Empty(t, taskIDs)
}

func TestLabelRepositoryImpl_ValidateIntegrityNoTemplates(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	config := appconfig.LabelConfig{TemplateDirs: []string{}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label without templates
	lbl := label.NewLabel("no-templates", "No templates", []string{}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Validate integrity (should be OK with no templates)
	result, err := repo.ValidateIntegrity(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, repository.ValidationOK, result.Status)
}

func TestLabelRepositoryImpl_MultipleTemplateDirs(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	// Create two template directories
	tmpDir1 := setupTempDir(t)
	tmpDir2 := setupTempDir(t)

	// Create files in both directories
	createTestFile(t, tmpDir1, "template1.md", "Content 1")
	createTestFile(t, tmpDir2, "template2.md", "Content 2")

	// Configure with both directories (first has priority)
	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir1, tmpDir2}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label with templates from both directories
	lbl := label.NewLabel("multi-dir", "Multi-directory test", []string{"template1.md", "template2.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	// Sync from files
	err = repo.SyncFromFile(ctx, lbl.ID())
	require.NoError(t, err)

	// Verify both hashes are set
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)

	hash1, exists1 := found.GetContentHash("template1.md")
	hash2, exists2 := found.GetContentHash("template2.md")
	assert.True(t, exists1)
	assert.True(t, exists2)
	assert.NotEmpty(t, hash1)
	assert.NotEmpty(t, hash2)
}

func TestLabelRepositoryImpl_FileWithoutNewlineEnding(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	// Create file without newline at end
	content := "Line 1\nLine 2\nLine 3"
	createTestFile(t, tmpDir, "no-newline.md", content)

	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label and sync
	lbl := label.NewLabel("no-newline", "No newline test", []string{"no-newline.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	err = repo.SyncFromFile(ctx, lbl.ID())
	require.NoError(t, err)

	// Verify line count (should be 3 even without trailing newline)
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, 3, found.LineCount())
}

func TestLabelRepositoryImpl_EmptyFile(t *testing.T) {
	db := setupTestDBForLabel(t)
	defer db.Close()

	tmpDir := setupTempDir(t)
	// Create empty file
	createTestFile(t, tmpDir, "empty.md", "")

	config := appconfig.LabelConfig{TemplateDirs: []string{tmpDir}}
	repo := NewLabelRepository(db, config).(*LabelRepositoryImpl)
	ctx := context.Background()

	// Create label and sync
	lbl := label.NewLabel("empty-file", "Empty file test", []string{"empty.md"}, 10)
	err := repo.Save(ctx, lbl)
	require.NoError(t, err)

	err = repo.SyncFromFile(ctx, lbl.ID())
	require.NoError(t, err)

	// Verify line count (should be 0 for empty file)
	found, err := repo.FindByID(ctx, lbl.ID())
	require.NoError(t, err)
	assert.Equal(t, 0, found.LineCount())

	// Hash should still exist
	hash, exists := found.GetContentHash("empty.md")
	assert.True(t, exists)
	assert.NotEmpty(t, hash)
}
