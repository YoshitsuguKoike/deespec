package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeJournalEntry_FillsDefaults(t *testing.T) {
	// Test with nil entry
	out := NormalizeJournalEntry(nil)

	// Check required fields are filled
	if out.TS == "" {
		t.Fatalf("ts should be filled with timestamp")
	}
	if out.Step != "unknown" {
		t.Errorf("step should default to 'unknown', got %q", out.Step)
	}
	if out.Artifacts == nil {
		t.Fatalf("artifacts must not be nil")
	}
	if len(out.Artifacts) != 0 {
		t.Errorf("artifacts should be empty array, got %v", out.Artifacts)
	}

	// Test with empty entry
	in := &JournalEntry{}
	out2 := NormalizeJournalEntry(in)

	if out2.TS == "" {
		t.Fatalf("ts should be filled")
	}
	if out2.Step != "unknown" {
		t.Errorf("step should be 'unknown' for empty step")
	}
	if out2.Artifacts == nil {
		t.Fatalf("artifacts must not be nil")
	}
}

func TestNormalizeJournalEntry_PreservesExistingValues(t *testing.T) {
	entry := &JournalEntry{
		TS:        "2025-01-01T00:00:00Z",
		Turn:      5,
		Step:      "review",
		Decision:  "OK",
		ElapsedMs: 1500,
		Error:     "some error",
		Artifacts: []string{"file1.txt", "file2.txt"},
	}

	normalized := NormalizeJournalEntry(entry)

	// Check values are preserved
	if normalized.TS != "2025-01-01T00:00:00Z" {
		t.Error("ts should be preserved")
	}
	if normalized.Turn != 5 {
		t.Error("turn should be preserved")
	}
	if normalized.Step != "review" {
		t.Error("step should be preserved")
	}
	if normalized.Decision != "OK" {
		t.Error("decision should be preserved")
	}
	if normalized.ElapsedMs != 1500 {
		t.Error("elapsed_ms should be preserved")
	}
	if normalized.Error != "some error" {
		t.Error("error should be preserved")
	}
	if len(normalized.Artifacts) != 2 || normalized.Artifacts[0] != "file1.txt" {
		t.Error("artifacts should be preserved")
	}
}

func TestNormalizeJournalEntryMap_FillsMissingKeys(t *testing.T) {
	// Test with empty entry
	entry := map[string]interface{}{}
	normalized := NormalizeJournalEntryMap(entry)

	// Check all required keys are present
	requiredKeys := []string{"ts", "turn", "step", "decision", "elapsed_ms", "error", "artifacts"}
	for _, key := range requiredKeys {
		if _, exists := normalized[key]; !exists {
			t.Errorf("Required key %q is missing", key)
		}
	}

	// Check default values
	if normalized["ts"] == "" {
		t.Error("ts should be filled with timestamp")
	}
	if normalized["turn"] != 0 {
		t.Error("turn should default to 0")
	}
	if normalized["step"] != "unknown" {
		t.Error("step should default to 'unknown'")
	}
	if normalized["decision"] != "PENDING" {
		t.Error("decision should default to 'PENDING'")
	}
	if normalized["elapsed_ms"] != 0 {
		t.Error("elapsed_ms should default to 0")
	}
	if normalized["error"] != "" {
		t.Error("error should default to empty string")
	}

	// Check artifacts is array type
	artifacts, ok := normalized["artifacts"].([]string)
	if !ok {
		t.Error("artifacts should be []string type")
	}
	if len(artifacts) != 0 {
		t.Error("artifacts should default to empty array")
	}
}

func TestNormalizeJournalEntryMap_PreservesExistingValues(t *testing.T) {
	entry := map[string]interface{}{
		"ts":         "2025-01-01T00:00:00Z",
		"turn":       5,
		"step":       "review",
		"decision":   "OK",
		"elapsed_ms": 1500,
		"error":      "some error",
		"artifacts":  []string{"file1.txt", "file2.txt"},
	}

	normalized := NormalizeJournalEntryMap(entry)

	// Check values are preserved
	if normalized["ts"] != "2025-01-01T00:00:00Z" {
		t.Error("ts should be preserved")
	}
	if normalized["turn"] != 5 {
		t.Error("turn should be preserved")
	}
	if normalized["step"] != "review" {
		t.Error("step should be preserved")
	}
	if normalized["decision"] != "OK" {
		t.Error("decision should be preserved")
	}
	if normalized["elapsed_ms"] != 1500 {
		t.Error("elapsed_ms should be preserved")
	}
	if normalized["error"] != "some error" {
		t.Error("error should be preserved")
	}

	// Check artifacts
	artifacts, ok := normalized["artifacts"].([]string)
	if !ok || len(artifacts) != 2 {
		t.Error("artifacts should be preserved as []string")
	}
}

func TestNormalizeJournalEntryMap_HandlesStringArtifacts(t *testing.T) {
	// Test single string artifact
	entry := map[string]interface{}{
		"artifacts": "single_file.txt",
	}
	normalized := NormalizeJournalEntryMap(entry)

	artifacts, ok := normalized["artifacts"].([]string)
	if !ok {
		t.Fatal("artifacts should be []string type")
	}
	if len(artifacts) != 1 || artifacts[0] != "single_file.txt" {
		t.Error("single string artifact should be converted to array")
	}

	// Test empty string artifact
	entry2 := map[string]interface{}{
		"artifacts": "",
	}
	normalized2 := NormalizeJournalEntryMap(entry2)

	artifacts2, ok := normalized2["artifacts"].([]string)
	if !ok {
		t.Fatal("artifacts should be []string type")
	}
	if len(artifacts2) != 0 {
		t.Error("empty string artifact should be converted to empty array")
	}
}

func TestJournalWriter_Append(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "test_journal.ndjson")

	// Create writer
	writer := NewJournalWriter(journalPath)

	// Write an entry with missing fields
	entry := &JournalEntry{
		Turn: 1,
		Step: "test",
	}

	err := writer.Append(entry)
	if err != nil {
		t.Fatalf("Failed to append entry: %v", err)
	}

	// Read and verify the written entry
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal file: %v", err)
	}

	var written map[string]interface{}
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("Failed to unmarshal journal entry: %v", err)
	}

	// Verify all required keys are present
	requiredKeys := []string{"ts", "turn", "step", "decision", "elapsed_ms", "error", "artifacts"}
	for _, key := range requiredKeys {
		if _, exists := written[key]; !exists {
			t.Errorf("Required key %q is missing in written entry", key)
		}
	}

	// Verify specific values
	if written["turn"].(float64) != 1 {
		t.Error("turn value should be preserved")
	}
	if written["step"] != "test" {
		t.Error("step value should be preserved")
	}

	// Verify artifacts is array type
	if artifacts, ok := written["artifacts"].([]interface{}); !ok {
		t.Error("artifacts should be an array")
	} else if len(artifacts) != 0 {
		t.Error("artifacts should be empty array")
	}
}

func TestJournalWriter_QuickAppend(t *testing.T) {
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "test_journal.ndjson")

	writer := NewJournalWriter(journalPath)

	// Use QuickAppend
	err := writer.QuickAppend(2, "review", "OK", 1500, "", []string{"output.md"})
	if err != nil {
		t.Fatalf("Failed to quick append: %v", err)
	}

	// Read and verify
	data, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("Failed to read journal file: %v", err)
	}

	var written map[string]interface{}
	if err := json.Unmarshal(data, &written); err != nil {
		t.Fatalf("Failed to unmarshal journal entry: %v", err)
	}

	// Verify values
	if written["turn"].(float64) != 2 {
		t.Error("turn should be 2")
	}
	if written["step"] != "review" {
		t.Error("step should be 'review'")
	}
	if written["decision"] != "OK" {
		t.Error("decision should be 'OK'")
	}
	if written["elapsed_ms"].(float64) != 1500 {
		t.Error("elapsed_ms should be 1500")
	}

	// Check artifacts
	if artifacts, ok := written["artifacts"].([]interface{}); !ok {
		t.Error("artifacts should be array")
	} else if len(artifacts) != 1 || artifacts[0] != "output.md" {
		t.Error("artifacts should contain 'output.md'")
	}
}

func TestJournalWriter_ValidationWarning(t *testing.T) {
	// Test validation with DEE_VALIDATE=1
	tmpDir := t.TempDir()
	journalPath := filepath.Join(tmpDir, "test_journal.ndjson")

	// Enable validation
	os.Setenv("DEE_VALIDATE", "1")
	defer os.Unsetenv("DEE_VALIDATE")

	writer := NewJournalWriter(journalPath)

	// Write valid entry - should not fail
	entry := &JournalEntry{
		Turn:      1,
		Step:      "test",
		Artifacts: []string{},
	}

	err := writer.Append(entry)
	if err != nil {
		t.Fatalf("Valid entry should not fail even with validation: %v", err)
	}

	// Even with nil artifacts, normalization should fix it
	entry2 := &JournalEntry{
		Turn:      2,
		Step:      "implement",
		Artifacts: nil, // This will be normalized to []
	}

	err = writer.Append(entry2)
	if err != nil {
		t.Fatalf("Entry should not fail after normalization: %v", err)
	}
}
