package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCompletedTasksFromJournal_WithDONEStatus(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Create journal entries with both old and new format
	entries := []map[string]interface{}{
		// Old format - step=done with pick artifact
		{
			"ts":         "2025-09-30T10:00:00Z",
			"turn":       1,
			"step":       "done",
			"decision":   "SUCCEEDED",
			"elapsed_ms": 1000,
			"error":      "",
			"artifacts": []map[string]interface{}{
				{"type": "pick", "id": "SBI-001"},
			},
		},
		// New format - status=DONE with file path artifact
		{
			"ts":         "2025-09-30T11:00:00Z",
			"turn":       21,
			"step":       "done",
			"status":     "DONE",
			"decision":   "SUCCEEDED",
			"elapsed_ms": 70257,
			"error":      "",
			"artifacts":  []string{".deespec/specs/sbi/SBI-002/done_21.md"},
		},
		// Another new format entry
		{
			"ts":         "2025-09-30T12:00:00Z",
			"turn":       24,
			"step":       "done",
			"status":     "DONE",
			"decision":   "SUCCEEDED",
			"elapsed_ms": 42396,
			"error":      "",
			"artifacts": []interface{}{
				".deespec/specs/sbi/SBI-003/done_24.md",
				map[string]interface{}{
					"type": "review_note_rollup",
					"path": ".deespec/specs/sbi/SBI-003/review_notes.md",
				},
			},
		},
		// Entry without DONE status (should not be marked as completed)
		{
			"ts":         "2025-09-30T13:00:00Z",
			"turn":       25,
			"step":       "review",
			"status":     "REVIEW",
			"decision":   "NEEDS_CHANGES",
			"elapsed_ms": 30000,
			"error":      "",
			"artifacts":  []string{".deespec/specs/sbi/SBI-004/review_25.md"},
		},
		// Mixed format - has both step=done and artifact path
		{
			"ts":         "2025-09-30T14:00:00Z",
			"turn":       30,
			"step":       "done",
			"status":     "DONE",
			"decision":   "SUCCEEDED",
			"elapsed_ms": 50000,
			"error":      "",
			"artifacts": []interface{}{
				".deespec/specs/sbi/SBI-005/done_30.md",
				map[string]interface{}{
					"type": "pick",
					"id":   "SBI-005",
				},
			},
		},
	}

	// Write journal file
	file, err := os.Create(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatal(err)
		}
	}
	file.Close()

	// Test the function
	completed := getCompletedTasksFromJournal(journalPath)

	// Verify results
	expectedCompleted := map[string]bool{
		"SBI-001": true, // Old format
		"SBI-002": true, // New format from path
		"SBI-003": true, // New format with mixed artifacts
		"SBI-005": true, // Mixed format
	}

	for taskID, expected := range expectedCompleted {
		if completed[taskID] != expected {
			t.Errorf("Task %s: expected completed=%v, got %v", taskID, expected, completed[taskID])
		}
	}

	// Verify SBI-004 is not marked as completed
	if completed["SBI-004"] {
		t.Error("Task SBI-004 should not be marked as completed (status=REVIEW)")
	}
}

func TestGetCompletedTasksFromJournal_EmptyJournal(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Create empty journal file
	file, err := os.Create(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()

	// Test with empty journal
	completed := getCompletedTasksFromJournal(journalPath)

	if len(completed) != 0 {
		t.Errorf("Expected empty completed map, got %v", completed)
	}
}

func TestGetCompletedTasksFromJournal_NonexistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "nonexistent.ndjson")

	// Test with non-existent file
	completed := getCompletedTasksFromJournal(journalPath)

	if len(completed) != 0 {
		t.Errorf("Expected empty completed map for non-existent file, got %v", completed)
	}
}

func TestFilterReadyTasks_WithDONEStatus(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "journal.ndjson")

	// Create journal with completed tasks
	entries := []map[string]interface{}{
		{
			"ts":        "2025-09-30T10:00:00Z",
			"turn":      21,
			"step":      "done",
			"status":    "DONE",
			"decision":  "SUCCEEDED",
			"artifacts": []string{".deespec/specs/sbi/SBI-001/done_21.md"},
		},
	}

	file, err := os.Create(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	for _, entry := range entries {
		encoder.Encode(entry)
	}
	file.Close()

	// Create test tasks
	tasks := []*Task{
		{ID: "SBI-001", Title: "Completed Task", Priority: 1, POR: 1},
		{ID: "SBI-002", Title: "Ready Task", Priority: 1, POR: 2},
		{ID: "SBI-003", Title: "Depends on 001", Priority: 1, POR: 3, DependsOn: []string{"SBI-001"}},
		{ID: "SBI-004", Title: "Depends on 002", Priority: 1, POR: 4, DependsOn: []string{"SBI-002"}},
	}

	// Filter ready tasks
	readyTasks := filterReadyTasks(tasks, journalPath)

	// Verify results
	// SBI-001 should be filtered out (completed)
	// SBI-002 should be ready
	// SBI-003 should be ready (dependency SBI-001 is completed)
	// SBI-004 should not be ready (dependency SBI-002 is not completed)

	readyIDs := make(map[string]bool)
	for _, task := range readyTasks {
		readyIDs[task.ID] = true
	}

	if readyIDs["SBI-001"] {
		t.Error("SBI-001 should not be in ready tasks (it's completed)")
	}
	if !readyIDs["SBI-002"] {
		t.Error("SBI-002 should be in ready tasks")
	}
	if !readyIDs["SBI-003"] {
		t.Error("SBI-003 should be in ready tasks (dependency completed)")
	}
	if readyIDs["SBI-004"] {
		t.Error("SBI-004 should not be in ready tasks (dependency not completed)")
	}
}
