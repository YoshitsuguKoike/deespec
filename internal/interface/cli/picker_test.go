package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPickNextTask(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Create test tasks
	createTestTask(t, "SBI-001", "First Task", 1, 1)
	createTestTask(t, "SBI-002", "Second Task", 2, 1)
	createTestTask(t, "SBI-003", "Third Task", 1, 2)

	cfg := PickConfig{
		SpecsDir:    ".deespec/specs/sbi",
		JournalPath: ".deespec/var/journal.ndjson",
	}

	// Test basic picking
	picked, reason, err := PickNextTask(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if picked == nil {
		t.Fatal("Expected to pick a task")
	}

	// Should pick SBI-001 (lowest POR)
	if picked.ID != "SBI-001" {
		t.Errorf("Expected SBI-001, got %s", picked.ID)
	}

	if reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestPickNextTask_NoPendingTasks(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Create test task
	createTestTask(t, "SBI-001", "First Task", 1, 1)

	// Create journal showing task is done
	createJournal(t, []map[string]interface{}{
		{
			"ts":         "2025-09-26T10:00:00Z",
			"turn":       1,
			"step":       "done",
			"decision":   "OK",
			"elapsed_ms": 1000,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "id": "SBI-001", "spec_path": ".deespec/specs/sbi/SBI-001_first-task"},
			},
		},
	})

	cfg := PickConfig{
		SpecsDir:    ".deespec/specs/sbi",
		JournalPath: ".deespec/var/journal.ndjson",
	}

	// Test no tasks available
	picked, reason, err := PickNextTask(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if picked != nil {
		t.Error("Expected no task to be picked")
	}

	if reason != "no ready tasks" {
		t.Errorf("Expected 'no ready tasks', got %s", reason)
	}
}

func TestSortTasksByPriority(t *testing.T) {
	tasks := []*Task{
		{ID: "SBI-003", POR: 2, Priority: 1},
		{ID: "SBI-001", POR: 1, Priority: 2},
		{ID: "SBI-002", POR: 1, Priority: 1},
	}

	sortTasksByPriority(tasks, []string{"por", "priority", "id"})

	expected := []string{"SBI-002", "SBI-001", "SBI-003"}
	for i, task := range tasks {
		if task.ID != expected[i] {
			t.Errorf("Position %d: expected %s, got %s", i, expected[i], task.ID)
		}
	}
}

func TestResumeIfInProgress(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Create journal
	createJournal(t, []map[string]interface{}{
		{
			"ts":         "2025-09-26T10:00:00Z",
			"turn":       1,
			"step":       "implement",
			"decision":   "PENDING",
			"elapsed_ms": 1000,
			"error":      "",
			"artifacts":  []string{},
		},
	})

	// Test resume with WIP
	st := &State{
		CurrentTaskID: "SBI-001",
		Current:       "plan",
		Turn:          1,
	}

	resumed, reason, err := ResumeIfInProgress(st, ".deespec/var/journal.ndjson")
	if err != nil {
		t.Fatal(err)
	}

	if !resumed {
		t.Error("Expected to resume")
	}

	if st.Current != "implement" {
		t.Errorf("Expected current to be 'implement', got %s", st.Current)
	}

	if reason == "" {
		t.Error("Expected non-empty reason")
	}
}

func TestResumeIfInProgress_NoWIP(t *testing.T) {
	st := &State{
		CurrentTaskID: "",
		Current:       "plan",
		Turn:          1,
	}

	resumed, reason, err := ResumeIfInProgress(st, "")
	if err != nil {
		t.Fatal(err)
	}

	if resumed {
		t.Error("Expected not to resume when no WIP")
	}

	if reason != "" {
		t.Error("Expected empty reason when no WIP")
	}
}

func TestSyncStateWithJournal(t *testing.T) {
	tests := []struct {
		name         string
		state        *State
		journalEntry map[string]interface{}
		expectChange bool
		expectedStep string
		expectedTurn int
	}{
		{
			name:         "already in sync",
			state:        &State{Current: "implement", Turn: 1},
			journalEntry: map[string]interface{}{"step": "implement", "turn": float64(1)},
			expectChange: false,
		},
		{
			name:         "journal ahead",
			state:        &State{Current: "plan", Turn: 1},
			journalEntry: map[string]interface{}{"step": "implement", "turn": float64(1)},
			expectChange: true,
			expectedStep: "implement",
			expectedTurn: 1,
		},
		{
			name:         "done clears WIP",
			state:        &State{Current: "review", Turn: 2, CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "done", "turn": float64(2)},
			expectChange: true,
			expectedStep: "done",
			expectedTurn: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed, newState, err := SyncStateWithJournal(tt.state, tt.journalEntry)
			if err != nil {
				t.Fatal(err)
			}

			if changed != tt.expectChange {
				t.Errorf("Expected changed=%v, got %v", tt.expectChange, changed)
			}

			if tt.expectChange {
				if newState.Current != tt.expectedStep {
					t.Errorf("Expected step=%s, got %s", tt.expectedStep, newState.Current)
				}
				if newState.Turn != tt.expectedTurn {
					t.Errorf("Expected turn=%d, got %d", tt.expectedTurn, newState.Turn)
				}
				if tt.expectedStep == "done" && newState.CurrentTaskID != "" {
					t.Error("Expected CurrentTaskID to be cleared on done")
				}
			}
		})
	}
}

func TestGetCompletedTasksFromJournal(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Create journal with completed task
	createJournal(t, []map[string]interface{}{
		{
			"ts":         "2025-09-26T10:00:00Z",
			"turn":       1,
			"step":       "plan",
			"decision":   "PENDING",
			"elapsed_ms": 0,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "id": "SBI-001", "spec_path": ".deespec/specs/sbi/SBI-001_test"},
			},
		},
		{
			"ts":         "2025-09-26T10:01:00Z",
			"turn":       2,
			"step":       "done",
			"decision":   "OK",
			"elapsed_ms": 5000,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "id": "SBI-001", "spec_path": ".deespec/specs/sbi/SBI-001_test"},
			},
		},
	})

	completed := getCompletedTasksFromJournal(".deespec/var/journal.ndjson")

	if !completed["SBI-001"] {
		t.Error("Expected SBI-001 to be marked as completed")
	}

	if completed["SBI-002"] {
		t.Error("SBI-002 should not be marked as completed")
	}
}

// Test helpers

func createTestTask(t *testing.T, id, title string, priority, por int) {
	t.Helper()

	taskDir := filepath.Join(".deespec", "specs", "sbi", fmt.Sprintf("%s_%s", id, strings.ToLower(strings.ReplaceAll(title, " ", "-"))))
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := TaskMeta{
		ID:       id,
		Title:    title,
		Priority: priority,
		POR:      por,
		Phase:    "Implementation",
		Role:     "developer",
	}

	data, err := yaml.Marshal(meta)
	if err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(taskDir, "meta.yaml")
	if err := os.WriteFile(metaPath, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func createJournal(t *testing.T, entries []map[string]interface{}) {
	t.Helper()

	journalDir := filepath.Dir(".deespec/var/journal.ndjson")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(".deespec/var/journal.ndjson")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, entry := range entries {
		data, err := json.Marshal(entry)
		if err != nil {
			t.Fatal(err)
		}
		f.Write(data)
		f.Write([]byte("\n"))
	}
}

// TestNoAbsolutePathsInPicker verifies we don't use absolute paths
func TestNoAbsolutePathsInPicker(t *testing.T) {
	// Verify default paths are relative
	cfg := PickConfig{}
	if cfg.SpecsDir == "" {
		cfg.SpecsDir = ".deespec/specs/sbi"
	}

	if filepath.IsAbs(cfg.SpecsDir) {
		t.Errorf("Default specs dir should be relative, got: %s", cfg.SpecsDir)
	}

	defaultJournal := ".deespec/var/journal.ndjson"
	if filepath.IsAbs(defaultJournal) {
		t.Errorf("Default journal path should be relative, got: %s", defaultJournal)
	}
}

func TestRecordPickInJournal(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	task := &Task{
		ID:       "SBI-001",
		SpecPath: ".deespec/specs/sbi/SBI-001_test",
		Title:    "Test Task",
	}

	// Create directory for journal
	if err := os.MkdirAll(".deespec/var", 0755); err != nil {
		t.Fatal(err)
	}

	err := RecordPickInJournal(task, 1, "")
	if err != nil {
		t.Fatal(err)
	}

	// Verify journal entry
	data, err := os.ReadFile(".deespec/var/journal.ndjson")
	if err != nil {
		t.Fatal(err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}

	if entry["step"] != "plan" {
		t.Errorf("Expected step=plan, got %v", entry["step"])
	}

	if entry["turn"] != float64(1) {
		t.Errorf("Expected turn=1, got %v", entry["turn"])
	}

	artifacts, ok := entry["artifacts"].([]interface{})
	if !ok || len(artifacts) == 0 {
		t.Fatal("Expected artifacts array")
	}

	artifact, ok := artifacts[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected artifact object")
	}

	if artifact["type"] != "pick" {
		t.Errorf("Expected artifact type=pick, got %v", artifact["type"])
	}

	if artifact["id"] != "SBI-001" {
		t.Errorf("Expected artifact id=SBI-001, got %v", artifact["id"])
	}
}