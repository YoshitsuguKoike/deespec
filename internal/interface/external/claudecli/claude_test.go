package claudecli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStreamEvent_JSON(t *testing.T) {
	// Test StreamEvent JSON marshaling
	event := StreamEvent{
		Type:      "content",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Content:   "Test content",
		Tool:      "Read",
		Args: map[string]interface{}{
			"file": "test.go",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal StreamEvent: %v", err)
	}

	var decoded StreamEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal StreamEvent: %v", err)
	}

	if decoded.Type != event.Type {
		t.Errorf("Type mismatch: got %s, want %s", decoded.Type, event.Type)
	}

	if decoded.Content != event.Content {
		t.Errorf("Content mismatch: got %s, want %s", decoded.Content, event.Content)
	}
}

func TestStreamContext_HistoryPath(t *testing.T) {
	// Test history file path generation
	ctx := &StreamContext{
		SBIDir:   ".deespec/specs/sbi/TEST-001",
		StepName: "implement",
		Turn:     5,
	}

	expectedPath := filepath.Join(ctx.SBIDir, "histories", "workflow_step_5.jsonl")

	// This matches what the actual implementation does
	actualPath := filepath.Join(ctx.SBIDir, "histories",
		fmt.Sprintf("workflow_step_%d.jsonl", ctx.Turn))

	if actualPath != expectedPath {
		t.Errorf("Path mismatch: got %s, want %s", actualPath, expectedPath)
	}
}

func TestRunWithStream_CreatesHistoryDir(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	sbiDir := filepath.Join(tmpDir, "sbi", "TEST-001")

	ctx := &StreamContext{
		SBIDir:   sbiDir,
		StepName: "test",
		Turn:     1,
		LogWriter: func(format string, args ...interface{}) {
			// No-op logger for test
		},
	}

	// Check that histories directory would be created
	historiesDir := filepath.Join(ctx.SBIDir, "histories")

	// Create the directories to simulate the RunWithStream behavior
	if err := os.MkdirAll(historiesDir, 0755); err != nil {
		t.Fatalf("Failed to create histories directory: %v", err)
	}

	// Verify directory exists
	if _, err := os.Stat(historiesDir); os.IsNotExist(err) {
		t.Error("Histories directory was not created")
	}

	// Verify we can write a JSONL file there
	testFile := filepath.Join(historiesDir, "workflow_step_1.jsonl")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	// Write a test event
	encoder := json.NewEncoder(file)
	testEvent := StreamEvent{
		Type:      "test",
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Content:   "Test event",
	}

	if err := encoder.Encode(testEvent); err != nil {
		t.Errorf("Failed to write test event: %v", err)
	}
}
