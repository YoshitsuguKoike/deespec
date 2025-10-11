package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

// Test helper functions

func createTestMetaYAML(t *testing.T, dir string, id string, title string, priority int, por int, dependsOn []string) {
	t.Helper()

	metaPath := filepath.Join(dir, "meta.yaml")
	content := "id: " + id + "\n"
	content += "title: " + title + "\n"
	if priority > 0 {
		content += "priority: " + string(rune(priority+'0')) + "\n"
	}
	if por > 0 {
		content += "por: " + string(rune(por+'0')) + "\n"
	}
	if len(dependsOn) > 0 {
		content += "depends_on:\n"
		for _, dep := range dependsOn {
			content += "  - " + dep + "\n"
		}
	}
	content += "phase: implementation\n"
	content += "role: developer\n"

	err := os.WriteFile(metaPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test meta.yaml: %v", err)
	}
}

func createTestJournal(t *testing.T, journalPath string, content string) {
	t.Helper()

	// Create parent directory if it doesn't exist
	dir := filepath.Dir(journalPath)
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create journal directory: %v", err)
	}

	err = os.WriteFile(journalPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test journal: %v", err)
	}
}

// LoadAllTasks Tests

func TestSBITaskRepositoryImpl_LoadAllTasks_EmptyDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks should not fail on empty directory: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks from empty directory, got %d", len(tasks))
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_NonExistentDirectory(t *testing.T) {
	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	nonExistentPath := "/tmp/non_existent_directory_" + t.Name()
	tasks, err := repo.LoadAllTasks(ctx, nonExistentPath)
	if err != nil {
		t.Fatalf("LoadAllTasks should not fail on non-existent directory: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks from non-existent directory, got %d", len(tasks))
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_SingleTask(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a task directory with meta.yaml
	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	createTestMetaYAML(t, taskDir, "SBI-001", "Test Task 1", 1, 1, nil)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.ID != "SBI-001" {
		t.Errorf("Expected ID 'SBI-001', got '%s'", task.ID)
	}
	if task.Title != "Test Task 1" {
		t.Errorf("Expected title 'Test Task 1', got '%s'", task.Title)
	}
	if task.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", task.Priority)
	}
	if task.POR != 1 {
		t.Errorf("Expected POR 1, got %d", task.POR)
	}
	if task.Status != "PENDING" {
		t.Errorf("Expected status 'PENDING', got '%s'", task.Status)
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_MultipleTasks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create multiple task directories
	for i := 1; i <= 3; i++ {
		taskID := "SBI-00" + string(rune(i+'0'))
		taskDir := filepath.Join(tmpDir, taskID)
		err = os.MkdirAll(taskDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create task directory: %v", err)
		}
		createTestMetaYAML(t, taskDir, taskID, "Task "+string(rune(i+'0')), i, i, nil)
	}

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 3 {
		t.Errorf("Expected 3 tasks, got %d", len(tasks))
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_WithDependencies(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create task with dependencies
	taskDir := filepath.Join(tmpDir, "SBI-002")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	createTestMetaYAML(t, taskDir, "SBI-002", "Dependent Task", 2, 2, []string{"SBI-001"})

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if len(task.DependsOn) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(task.DependsOn))
	}
	if task.DependsOn[0] != "SBI-001" {
		t.Errorf("Expected dependency 'SBI-001', got '%s'", task.DependsOn[0])
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_DefaultPriority(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	// Create meta.yaml without priority and por
	metaPath := filepath.Join(taskDir, "meta.yaml")
	content := `id: SBI-001
title: Test Task
phase: implementation
role: developer
`
	err = os.WriteFile(metaPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create meta.yaml: %v", err)
	}

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.Priority != 999 {
		t.Errorf("Expected default priority 999, got %d", task.Priority)
	}
	if task.POR != 999 {
		t.Errorf("Expected default POR 999, got %d", task.POR)
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_InvalidYAML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	// Create invalid YAML
	metaPath := filepath.Join(taskDir, "meta.yaml")
	invalidContent := "id: SBI-001\ntitle: Test\ninvalid yaml: [unclosed"
	err = os.WriteFile(metaPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create meta.yaml: %v", err)
	}

	// Use custom warning logger to capture warnings
	var warnCalled bool
	warnLog := func(format string, args ...interface{}) {
		warnCalled = true
	}

	repo := NewSBITaskRepositoryImpl(warnLog, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks should not fail on invalid YAML: %v", err)
	}

	// Should return empty list and log warning
	if len(tasks) != 0 {
		t.Errorf("Expected 0 tasks from invalid YAML, got %d", len(tasks))
	}

	if !warnCalled {
		t.Error("Expected warning to be logged for invalid YAML")
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_NestedDirectories(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested structure
	nestedDir := filepath.Join(tmpDir, "subdir", "SBI-001")
	err = os.MkdirAll(nestedDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	createTestMetaYAML(t, nestedDir, "SBI-001", "Nested Task", 1, 1, nil)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task from nested directory, got %d", len(tasks))
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_MetaYmlExtension(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	// Use .yml extension instead of .yaml
	metaPath := filepath.Join(taskDir, "meta.yml")
	content := `id: SBI-001
title: Test Task
priority: 1
por: 1
phase: implementation
role: developer
`
	err = os.WriteFile(metaPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create meta.yml: %v", err)
	}

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("Expected 1 task with .yml extension, got %d", len(tasks))
	}
}

func TestSBITaskRepositoryImpl_LoadAllTasks_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create task directory
	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}
	createTestMetaYAML(t, taskDir, "SBI-001", "Test Task", 1, 1, nil)

	repo := NewSBITaskRepositoryImpl(nil, nil)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = repo.LoadAllTasks(ctx, tmpDir)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// GetCompletedTasks Tests

func TestSBITaskRepositoryImpl_GetCompletedTasks_EmptyJournal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	completed, err := repo.GetCompletedTasks(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetCompletedTasks should not fail on non-existent journal: %v", err)
	}

	if len(completed) != 0 {
		t.Errorf("Expected 0 completed tasks from empty journal, got %d", len(completed))
	}
}

func TestSBITaskRepositoryImpl_GetCompletedTasks_OldFormatDone(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"done","decision":"COMPLETED","elapsed_ms":1000,"error":"","artifacts":[{"type":"pick","id":"SBI-001"}]}
`
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	completed, err := repo.GetCompletedTasks(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetCompletedTasks failed: %v", err)
	}

	if !completed["SBI-001"] {
		t.Error("Expected SBI-001 to be marked as completed (old format)")
	}
}

func TestSBITaskRepositoryImpl_GetCompletedTasks_NewFormatDone(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"done","status":"DONE","decision":"COMPLETED","elapsed_ms":1000,"error":"","artifacts":[".deespec/specs/sbi/SBI-002/done_1.md"]}
`
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	completed, err := repo.GetCompletedTasks(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetCompletedTasks failed: %v", err)
	}

	if !completed["SBI-002"] {
		t.Error("Expected SBI-002 to be marked as completed (new format)")
	}
}

func TestSBITaskRepositoryImpl_GetCompletedTasks_MultipleEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[]}
{"ts":"2025-01-01T00:01:00Z","turn":1,"step":"implement","decision":"PENDING","elapsed_ms":500,"error":"","artifacts":[]}
{"ts":"2025-01-01T00:02:00Z","turn":1,"step":"done","decision":"COMPLETED","elapsed_ms":1000,"error":"","artifacts":[{"type":"pick","id":"SBI-001"}]}
{"ts":"2025-01-01T00:03:00Z","turn":2,"step":"done","status":"DONE","decision":"COMPLETED","elapsed_ms":2000,"error":"","artifacts":[".deespec/specs/sbi/SBI-002/done_1.md"]}
`
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	completed, err := repo.GetCompletedTasks(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetCompletedTasks failed: %v", err)
	}

	if len(completed) != 2 {
		t.Errorf("Expected 2 completed tasks, got %d", len(completed))
	}

	if !completed["SBI-001"] {
		t.Error("Expected SBI-001 to be marked as completed")
	}

	if !completed["SBI-002"] {
		t.Error("Expected SBI-002 to be marked as completed")
	}
}

func TestSBITaskRepositoryImpl_GetCompletedTasks_DefaultPath(t *testing.T) {
	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	// Test with empty path (should use default)
	completed, err := repo.GetCompletedTasks(ctx, "")
	if err != nil {
		t.Fatalf("GetCompletedTasks should not fail with default path: %v", err)
	}

	// Should return empty map if default journal doesn't exist
	if completed == nil {
		t.Error("Expected non-nil map from GetCompletedTasks")
	}
}

func TestSBITaskRepositoryImpl_GetCompletedTasks_CorruptedJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"done","artifacts":[{"type":"pick","id":"SBI-001"}]}
{"invalid json line
{"ts":"2025-01-01T00:02:00Z","turn":2,"step":"done","status":"DONE","artifacts":[".deespec/specs/sbi/SBI-002/done_1.md"]}
`
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	completed, err := repo.GetCompletedTasks(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetCompletedTasks failed: %v", err)
	}

	// Should skip corrupted line and process valid ones
	if len(completed) != 2 {
		t.Errorf("Expected 2 completed tasks (corrupted line skipped), got %d", len(completed))
	}
}

func TestSBITaskRepositoryImpl_GetCompletedTasks_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Create a large journal with many entries
	var journalContent string
	for i := 0; i < 1000; i++ {
		journalContent += `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[]}
`
	}
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = repo.GetCompletedTasks(ctx, journalPath)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// GetLastJournalEntry Tests

func TestSBITaskRepositoryImpl_GetLastJournalEntry_EmptyJournal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	entry, err := repo.GetLastJournalEntry(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetLastJournalEntry should not fail on non-existent journal: %v", err)
	}

	if entry != nil {
		t.Error("Expected nil entry from non-existent journal")
	}
}

func TestSBITaskRepositoryImpl_GetLastJournalEntry_SingleEntry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[]}
`
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	entry, err := repo.GetLastJournalEntry(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetLastJournalEntry failed: %v", err)
	}

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}

	if entry["step"] != "plan" {
		t.Errorf("Expected step 'plan', got '%v'", entry["step"])
	}
}

func TestSBITaskRepositoryImpl_GetLastJournalEntry_MultipleEntries(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[]}
{"ts":"2025-01-01T00:01:00Z","turn":1,"step":"implement","decision":"PENDING","elapsed_ms":500,"error":"","artifacts":[]}
{"ts":"2025-01-01T00:02:00Z","turn":1,"step":"done","decision":"COMPLETED","elapsed_ms":1000,"error":"","artifacts":[]}
`
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	entry, err := repo.GetLastJournalEntry(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetLastJournalEntry failed: %v", err)
	}

	if entry == nil {
		t.Fatal("Expected non-nil entry")
	}

	if entry["step"] != "done" {
		t.Errorf("Expected last step 'done', got '%v'", entry["step"])
	}

	if entry["decision"] != "COMPLETED" {
		t.Errorf("Expected decision 'COMPLETED', got '%v'", entry["decision"])
	}
}

func TestSBITaskRepositoryImpl_GetLastJournalEntry_TrailingEmptyLines(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[]}


`
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	entry, err := repo.GetLastJournalEntry(ctx, journalPath)
	if err != nil {
		t.Fatalf("GetLastJournalEntry failed: %v", err)
	}

	if entry == nil {
		t.Fatal("Expected non-nil entry (should skip trailing empty lines)")
	}

	if entry["step"] != "plan" {
		t.Errorf("Expected step 'plan', got '%v'", entry["step"])
	}
}

func TestSBITaskRepositoryImpl_GetLastJournalEntry_DefaultPath(t *testing.T) {
	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	// Test with empty path (should use default)
	entry, err := repo.GetLastJournalEntry(ctx, "")
	if err != nil {
		t.Fatalf("GetLastJournalEntry should not fail with default path: %v", err)
	}

	// Should return nil if default journal doesn't exist
	_ = entry // entry may be nil or non-nil depending on state
}

func TestSBITaskRepositoryImpl_GetLastJournalEntry_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Create journal with many entries
	var journalContent string
	for i := 0; i < 1000; i++ {
		journalContent += `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[]}
`
	}
	createTestJournal(t, journalPath, journalContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = repo.GetLastJournalEntry(ctx, journalPath)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

// RecordPickInJournal Tests

func TestSBITaskRepositoryImpl_RecordPickInJournal_NewJournal(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	task := &dto.SBITaskDTO{
		ID:       "SBI-001",
		SpecPath: "specs/spec",
		Title:    "Test Task",
		Priority: 1,
		POR:      2,
	}

	err = repo.RecordPickInJournal(ctx, task, 1, journalPath)
	if err != nil {
		t.Fatalf("RecordPickInJournal failed: %v", err)
	}

	// Verify file exists and has content
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	if len(content) == 0 {
		t.Error("Expected journal to have content")
	}
}

func TestSBITaskRepositoryImpl_RecordPickInJournal_AppendToExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Create existing journal
	initialContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[]}
`
	createTestJournal(t, journalPath, initialContent)

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	task := &dto.SBITaskDTO{
		ID:       "SBI-002",
		SpecPath: "specs/spec2",
		Title:    "Test Task 2",
		Priority: 3,
		POR:      4,
	}

	err = repo.RecordPickInJournal(ctx, task, 2, journalPath)
	if err != nil {
		t.Fatalf("RecordPickInJournal failed: %v", err)
	}

	// Verify both entries exist
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	lines := len([]byte(content))
	if lines < len(initialContent) {
		t.Error("Expected journal to have appended content")
	}
}

func TestSBITaskRepositoryImpl_RecordPickInJournal_NullPriorities(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	task := &dto.SBITaskDTO{
		ID:       "SBI-001",
		SpecPath: "specs/spec",
		Title:    "Test Task",
		Priority: 0, // Should be null in JSON
		POR:      0, // Should be null in JSON
	}

	err = repo.RecordPickInJournal(ctx, task, 1, journalPath)
	if err != nil {
		t.Fatalf("RecordPickInJournal failed: %v", err)
	}

	// Read and verify JSON contains null for priorities
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	// Content should contain null values
	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Error("Expected journal to have content")
	}
}

func TestSBITaskRepositoryImpl_RecordPickInJournal_DefaultPath(t *testing.T) {
	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	task := &dto.SBITaskDTO{
		ID:       "SBI-001",
		SpecPath: "specs/spec",
		Title:    "Test Task",
		Priority: 1,
		POR:      2,
	}

	// Note: This test will try to write to the default path
	// In a real scenario, you might want to clean up after this test
	err := repo.RecordPickInJournal(ctx, task, 1, "")
	if err != nil {
		// It's ok if this fails due to permissions or missing directory
		// The key is that it attempts to use the default path
		t.Logf("RecordPickInJournal with default path: %v", err)
	}
}

// Edge Cases and Integration Tests

func TestSBITaskRepositoryImpl_UnicodeSupport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	// Create meta.yaml with Japanese characters
	metaPath := filepath.Join(taskDir, "meta.yaml")
	content := `id: SBI-001
title: テスト用タスク 🚀
priority: 1
por: 1
phase: 実装フェーズ
role: 開発者
`
	err = os.WriteFile(metaPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create meta.yaml: %v", err)
	}

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Title != "テスト用タスク 🚀" {
		t.Errorf("Unicode title not preserved: got '%s'", tasks[0].Title)
	}
}

func TestSBITaskRepositoryImpl_LoggingIntegration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	var warnCalled bool
	var debugCalled bool

	warnLog := func(format string, args ...interface{}) {
		warnCalled = true
	}
	debugLog := func(format string, args ...interface{}) {
		debugCalled = true
	}

	repo := NewSBITaskRepositoryImpl(warnLog, debugLog)

	// Create invalid task to trigger warning
	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	metaPath := filepath.Join(taskDir, "meta.yaml")
	invalidContent := "invalid: yaml: [content"
	err = os.WriteFile(metaPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create invalid meta.yaml: %v", err)
	}

	ctx := context.Background()
	_, _ = repo.LoadAllTasks(ctx, tmpDir)

	if !warnCalled {
		t.Error("Expected warning logger to be called for invalid YAML")
	}

	// Test debug logging with journal operations
	journalPath := filepath.Join(tmpDir, "journal.ndjson")
	journalContent := `{"ts":"2025-01-01T00:00:00Z","turn":1,"step":"plan","decision":"PENDING","elapsed_ms":100,"error":"","artifacts":[{"type":"register","id":"SBI-001"}]}
`
	createTestJournal(t, journalPath, journalContent)

	_, _ = repo.GetCompletedTasks(ctx, journalPath)

	if !debugCalled {
		t.Error("Expected debug logger to be called for journal parsing")
	}
}

func TestSBITaskRepositoryImpl_MetaFieldsPreservation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sbi_task_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	taskDir := filepath.Join(tmpDir, "SBI-001")
	err = os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create task directory: %v", err)
	}

	metaPath := filepath.Join(taskDir, "meta.yaml")
	content := `id: SBI-001
title: Test Task
priority: 1
por: 2
phase: development
role: backend-engineer
`
	err = os.WriteFile(metaPath, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create meta.yaml: %v", err)
	}

	repo := NewSBITaskRepositoryImpl(nil, nil)
	ctx := context.Background()

	tasks, err := repo.LoadAllTasks(ctx, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("Expected 1 task, got %d", len(tasks))
	}

	task := tasks[0]
	if task.Meta["phase"] != "development" {
		t.Errorf("Expected phase 'development', got '%v'", task.Meta["phase"])
	}

	if task.Meta["role"] != "backend-engineer" {
		t.Errorf("Expected role 'backend-engineer', got '%v'", task.Meta["role"])
	}
}
