package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestJournalWriterAppend(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "test-journal-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "test.ndjson")
	writer := NewJournalWriter(journalPath)

	// Test entry
	entry := &JournalEntry{
		TS:        time.Now().UTC().Format(time.RFC3339Nano),
		Turn:      1,
		Step:      "test",
		Decision:  "OK",
		ElapsedMs: 100,
		Error:     "",
		Artifacts: []string{},
	}

	// Write entry
	if err := writer.Append(entry); err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("Journal file not created: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	// Parse JSON
	var readEntry JournalEntry
	if err := json.Unmarshal(bytes.TrimSuffix(content, []byte("\n")), &readEntry); err != nil {
		t.Fatalf("Failed to parse journal entry: %v", err)
	}

	// Verify fields
	if readEntry.Turn != entry.Turn {
		t.Errorf("Turn mismatch: got %d, want %d", readEntry.Turn, entry.Turn)
	}
	if readEntry.Step != entry.Step {
		t.Errorf("Step mismatch: got %s, want %s", readEntry.Step, entry.Step)
	}
	if readEntry.Decision != entry.Decision {
		t.Errorf("Decision mismatch: got %s, want %s", readEntry.Decision, entry.Decision)
	}
}

func TestJournalWriterMultipleAppends(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-journal-multi-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "multi.ndjson")
	writer := NewJournalWriter(journalPath)

	// Write multiple entries
	for i := 0; i < 10; i++ {
		entry := &JournalEntry{
			TS:        time.Now().UTC().Format(time.RFC3339Nano),
			Turn:      i,
			Step:      "test",
			Decision:  "PENDING",
			ElapsedMs: int64(i * 10),
			Error:     "",
			Artifacts: []string{},
		}
		if err := writer.Append(entry); err != nil {
			t.Fatalf("Failed to append entry %d: %v", i, err)
		}
	}

	// Read file
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	// Count lines
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 10 {
		t.Errorf("Expected 10 entries, got %d", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var entry JournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Line %d is not valid JSON: %v", i, err)
		}
		if entry.Turn != i {
			t.Errorf("Line %d has wrong turn: got %d, want %d", i, entry.Turn, i)
		}
	}
}

func TestJournalWriterConcurrentAppends(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-journal-concurrent-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "concurrent.ndjson")
	writer := NewJournalWriter(journalPath)

	// Use channel to coordinate goroutines
	done := make(chan bool, 5)

	// Launch concurrent writers
	for i := 0; i < 5; i++ {
		go func(id int) {
			entry := &JournalEntry{
				TS:        time.Now().UTC().Format(time.RFC3339Nano),
				Turn:      id,
				Step:      "concurrent",
				Decision:  "OK",
				ElapsedMs: 50,
				Error:     "",
				Artifacts: []string{},
			}
			if err := writer.Append(entry); err != nil {
				t.Errorf("Goroutine %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 5; i++ {
		<-done
	}

	// Read and verify all entries were written
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 5 {
		t.Errorf("Expected 5 entries, got %d", len(lines))
	}

	// Verify all entries are valid
	turnsSeen := make(map[int]bool)
	for _, line := range lines {
		var entry JournalEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("Invalid JSON: %v", err)
		}
		turnsSeen[entry.Turn] = true
	}

	// Check all turns were written
	for i := 0; i < 5; i++ {
		if !turnsSeen[i] {
			t.Errorf("Turn %d was not written", i)
		}
	}
}

func TestJournalWriterNormalization(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-journal-norm-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	journalPath := filepath.Join(tmpDir, "norm.ndjson")
	writer := NewJournalWriter(journalPath)

	// Entry with missing fields
	entry := &JournalEntry{
		Turn: 1,
		Step: "test",
		// Missing TS, Decision, ElapsedMs, Error, Artifacts
	}

	// Should normalize and not fail
	if err := writer.Append(entry); err != nil {
		t.Fatalf("Failed to append normalized entry: %v", err)
	}

	// Read and verify normalization
	content, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	var readEntry JournalEntry
	if err := json.Unmarshal(bytes.TrimSuffix(content, []byte("\n")), &readEntry); err != nil {
		t.Fatalf("Failed to parse entry: %v", err)
	}

	// Check normalized fields
	if readEntry.TS == "" {
		t.Error("TS should be normalized to non-empty")
	}
	if readEntry.Decision != "PENDING" {
		t.Errorf("Decision should be normalized to PENDING, got %s", readEntry.Decision)
	}
	if readEntry.ElapsedMs != 0 {
		t.Errorf("ElapsedMs should be normalized to 0, got %d", readEntry.ElapsedMs)
	}
	if readEntry.Error != "" {
		t.Errorf("Error should be normalized to empty, got %s", readEntry.Error)
	}
	if readEntry.Artifacts == nil {
		t.Error("Artifacts should be normalized to non-nil")
	}
}
