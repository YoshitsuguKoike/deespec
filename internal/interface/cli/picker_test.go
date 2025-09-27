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
			journalEntry: map[string]interface{}{"step": "implement", "decision": "PENDING", "turn": float64(1)},
			expectChange: true,
			expectedStep: "implement",
			expectedTurn: 1,
		},
		{
			name:         "done clears WIP",
			state:        &State{Current: "review", Turn: 2, CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "done", "decision": "OK", "turn": float64(2)},
			expectChange: true,
			expectedStep: "plan", // done/OK transitions to plan, not done
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

// SBI-PICK-002 Tests

func TestRecordPickInJournal_SBI_PICK_002_Format(t *testing.T) {
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
		POR:      2,
		Priority: 5,
	}

	// Create directory for journal
	if err := os.MkdirAll(".deespec/var", 0755); err != nil {
		t.Fatal(err)
	}

	err := RecordPickInJournal(task, 1, "")
	if err != nil {
		t.Fatal(err)
	}

	// Verify SBI-PICK-002 artifact format
	data, err := os.ReadFile(".deespec/var/journal.ndjson")
	if err != nil {
		t.Fatal(err)
	}

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatal(err)
	}

	artifacts, ok := entry["artifacts"].([]interface{})
	if !ok || len(artifacts) == 0 {
		t.Fatal("Expected artifacts array")
	}

	artifact, ok := artifacts[0].(map[string]interface{})
	if !ok {
		t.Fatal("Expected artifact object")
	}

	// Check required fields according to SBI-PICK-002
	if artifact["task_id"] != "SBI-001" {
		t.Errorf("Expected task_id=SBI-001, got %v", artifact["task_id"])
	}

	if artifact["spec_path"] != ".deespec/specs/sbi/SBI-001_test" {
		t.Errorf("Expected spec_path, got %v", artifact["spec_path"])
	}

	if artifact["por"] != float64(2) {
		t.Errorf("Expected por=2, got %v", artifact["por"])
	}

	if artifact["priority"] != float64(5) {
		t.Errorf("Expected priority=5, got %v", artifact["priority"])
	}

	// Backward compatibility - id field should still exist
	if artifact["id"] != "SBI-001" {
		t.Errorf("Expected id=SBI-001 for backward compatibility, got %v", artifact["id"])
	}
}

func TestSyncStateWithJournal_SBI_PICK_002_Matrix(t *testing.T) {
	tests := []struct {
		name         string
		state        *State
		journalEntry map[string]interface{}
		expectChange bool
		expectedStep string
		expectedWIP  string
	}{
		{
			name:         "plan/PENDING -> implement",
			state:        &State{Current: "plan", CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "plan", "decision": "PENDING"},
			expectChange: true,
			expectedStep: "implement",
			expectedWIP:  "SBI-001",
		},
		{
			name:         "implement/PENDING -> implement (idempotent)",
			state:        &State{Current: "implement", CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "implement", "decision": "PENDING"},
			expectChange: true,
			expectedStep: "implement",
			expectedWIP:  "SBI-001",
		},
		{
			name:         "test/PENDING -> test (idempotent)",
			state:        &State{Current: "test", CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "test", "decision": "PENDING"},
			expectChange: true,
			expectedStep: "test",
			expectedWIP:  "SBI-001",
		},
		{
			name:         "review/NEEDS_CHANGES -> implement (boomerang)",
			state:        &State{Current: "review", CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "review", "decision": "NEEDS_CHANGES"},
			expectChange: true,
			expectedStep: "implement",
			expectedWIP:  "SBI-001",
		},
		{
			name:         "review/OK -> done",
			state:        &State{Current: "review", CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "review", "decision": "OK"},
			expectChange: true,
			expectedStep: "done",
			expectedWIP:  "SBI-001",
		},
		{
			name:         "done/OK -> plan (WIP cleared)",
			state:        &State{Current: "done", CurrentTaskID: "SBI-001"},
			journalEntry: map[string]interface{}{"step": "done", "decision": "OK"},
			expectChange: true,
			expectedStep: "plan",
			expectedWIP:  "",
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
				if newState.CurrentTaskID != tt.expectedWIP {
					t.Errorf("Expected WIP=%s, got %s", tt.expectedWIP, newState.CurrentTaskID)
				}
			}
		})
	}
}

func TestDependencyChecking(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Create test tasks with dependencies
	// Task B has no dependencies
	createTestTaskWithDeps(t, "SBI-B", "Task B", 1, 1, []string{})
	// Task A depends on B
	createTestTaskWithDeps(t, "SBI-A", "Task A", 1, 1, []string{"SBI-B"})
	// Task C depends on unknown task
	createTestTaskWithDeps(t, "SBI-C", "Task C", 1, 1, []string{"SBI-UNKNOWN"})

	cfg := PickConfig{
		SpecsDir:    ".deespec/specs/sbi",
		JournalPath: ".deespec/var/journal.ndjson",
	}

	// First pick should get B (no dependencies)
	picked, _, err := PickNextTask(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if picked == nil || picked.ID != "SBI-B" {
		t.Errorf("Expected to pick SBI-B first, got %v", picked)
	}

	// Mark B as done
	createJournal(t, []map[string]interface{}{
		{
			"ts":         "2025-09-26T10:00:00Z",
			"turn":       1,
			"step":       "done",
			"decision":   "OK",
			"elapsed_ms": 1000,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "task_id": "SBI-B", "id": "SBI-B"},
			},
		},
	})

	// Now A should be pickable
	picked, _, err = PickNextTask(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if picked == nil || picked.ID != "SBI-A" {
		t.Errorf("Expected to pick SBI-A after B done, got %v", picked)
	}

	// C should never be pickable (unknown dependency)
	// Create fresh journal with A also done
	createJournal(t, []map[string]interface{}{
		{
			"ts":         "2025-09-26T10:00:00Z",
			"turn":       1,
			"step":       "done",
			"decision":   "OK",
			"elapsed_ms": 1000,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "task_id": "SBI-B", "id": "SBI-B"},
			},
		},
		{
			"ts":         "2025-09-26T10:01:00Z",
			"turn":       2,
			"step":       "done",
			"decision":   "OK",
			"elapsed_ms": 1000,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "task_id": "SBI-A", "id": "SBI-A"},
			},
		},
	})

	picked, reason, err := PickNextTask(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if picked != nil {
		t.Errorf("Should not pick any task (C has unknown dep), but picked %v", picked)
	}
	if reason != "no ready tasks" {
		t.Errorf("Expected 'no ready tasks', got %s", reason)
	}
}

func TestCyclicDependencies(t *testing.T) {
	// Test circular dependency detection
	tasks := []*Task{
		{ID: "A", DependsOn: []string{"B"}},
		{ID: "B", DependsOn: []string{"C"}},
		{ID: "C", DependsOn: []string{"A"}}, // Creates cycle A->B->C->A
		{ID: "D", DependsOn: []string{}},    // Independent task
	}

	inCycle := detectCycles(tasks)

	// A, B, and C should be in cycle
	if !inCycle["A"] {
		t.Error("Expected A to be in cycle")
	}
	if !inCycle["B"] {
		t.Error("Expected B to be in cycle")
	}
	if !inCycle["C"] {
		t.Error("Expected C to be in cycle")
	}
	// D should not be in cycle
	if inCycle["D"] {
		t.Error("Expected D not to be in cycle")
	}
}

// Helper function to create task with dependencies
func createTestTaskWithDeps(t *testing.T, id, title string, priority, por int, deps []string) {
	t.Helper()

	taskDir := filepath.Join(".deespec", "specs", "sbi", fmt.Sprintf("%s_%s", id, strings.ToLower(strings.ReplaceAll(title, " ", "-"))))
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatal(err)
	}

	meta := TaskMeta{
		ID:        id,
		Title:     title,
		Priority:  priority,
		POR:       por,
		DependsOn: deps,
		Phase:     "Implementation",
		Role:      "developer",
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

func TestTurnConsistency_SBI_PICK_002(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	originalDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(originalDir)

	// Create multiple journal entries for same turn
	journalEntries := []map[string]interface{}{
		{
			"ts":         "2025-09-26T10:00:00Z",
			"turn":       5,
			"step":       "plan",
			"decision":   "PENDING",
			"elapsed_ms": 0,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "task_id": "SBI-001", "spec_path": ".deespec/specs/sbi/SBI-001_test"},
			},
		},
		{
			"ts":         "2025-09-26T10:01:00Z",
			"turn":       5, // Same turn
			"step":       "implement",
			"decision":   "PENDING",
			"elapsed_ms": 1000,
			"error":      "",
			"artifacts":  []string{".deespec/var/artifacts/turn5/implement.md"},
		},
		{
			"ts":         "2025-09-26T10:02:00Z",
			"turn":       5, // Same turn
			"step":       "test",
			"decision":   "PENDING",
			"elapsed_ms": 500,
			"error":      "",
			"artifacts":  []string{".deespec/var/artifacts/turn5/test.md"},
		},
	}

	createJournal(t, journalEntries)

	// Read journal and verify all entries have same turn
	data, err := os.ReadFile(".deespec/var/journal.ndjson")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var turns []float64
	for _, line := range lines {
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		if turn, ok := entry["turn"].(float64); ok {
			turns = append(turns, turn)
		}
	}

	// All turns should be the same (5)
	expectedTurn := float64(5)
	for i, turn := range turns {
		if turn != expectedTurn {
			t.Errorf("Entry %d: expected turn=%v, got %v", i, expectedTurn, turn)
		}
	}

	if len(turns) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(turns))
	}
}
