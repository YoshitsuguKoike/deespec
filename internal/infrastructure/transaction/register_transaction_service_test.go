package transaction

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"gopkg.in/yaml.v3"
)

func TestExecuteRegisterTransaction(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "register_tx_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	// Create test spec
	spec := &dto.RegisterSpec{
		ID:     "test-001",
		Title:  "Test Specification",
		Labels: []string{"test", "transaction"},
	}

	specPath := ".deespec/specs/test-001_test_specification"

	// Create journal entry
	journalEntry := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       0,
		"step":       "register",
		"decision":   "OK",
		"elapsed_ms": 0,
		"error":      "",
		"artifacts": []map[string]interface{}{
			{
				"type":      "register",
				"id":        spec.ID,
				"spec_path": specPath,
				"source":    "test",
			},
		},
	}

	// Create service
	noopWarn := func(format string, args ...interface{}) {}
	service := NewRegisterTransactionService("", "", nil, noopWarn)

	// Execute transaction
	ctx := context.Background()
	err = service.ExecuteRegisterTransaction(ctx, spec, specPath, journalEntry)
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify meta.yaml was created
	metaPath := filepath.Join(specPath, "meta.yaml")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("meta.yaml not created")
	}

	// Verify meta.yaml content
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("Failed to read meta.yaml: %v", err)
	}

	var meta map[string]interface{}
	if err := yaml.Unmarshal(metaData, &meta); err != nil {
		t.Fatalf("Failed to parse meta.yaml: %v", err)
	}

	if meta["id"] != spec.ID {
		t.Errorf("meta.yaml ID mismatch: got %v, want %s", meta["id"], spec.ID)
	}
	if meta["title"] != spec.Title {
		t.Errorf("meta.yaml Title mismatch: got %v, want %s", meta["title"], spec.Title)
	}

	// Verify spec.md was created
	specMdPath := filepath.Join(specPath, "spec.md")
	if _, err := os.Stat(specMdPath); os.IsNotExist(err) {
		t.Error("spec.md not created")
	}

	// Verify journal entry was appended
	journalPath := filepath.Join(".deespec/var", "journal.ndjson")
	journalFile, err := os.Open(journalPath)
	if err != nil {
		t.Fatalf("Failed to open journal: %v", err)
	}
	defer journalFile.Close()

	journalData, err := io.ReadAll(journalFile)
	if err != nil {
		t.Fatalf("Failed to read journal: %v", err)
	}

	var journalEntries []map[string]interface{}
	for _, line := range splitLines(string(journalData)) {
		if line == "" {
			continue
		}
		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			journalEntries = append(journalEntries, entry)
		}
	}

	if len(journalEntries) != 1 {
		t.Errorf("Expected 1 journal entry, got %d", len(journalEntries))
	}

	if len(journalEntries) > 0 {
		entry := journalEntries[0]
		if entry["turn"].(float64) != 0 {
			t.Errorf("Journal turn mismatch: got %v, want 0", entry["turn"])
		}

		artifacts := entry["artifacts"].([]interface{})
		if len(artifacts) != 1 {
			t.Errorf("Expected 1 artifact, got %d", len(artifacts))
		}

		if len(artifacts) > 0 {
			artifact := artifacts[0].(map[string]interface{})
			if artifact["id"] != spec.ID {
				t.Errorf("Artifact ID mismatch: got %v, want %s", artifact["id"], spec.ID)
			}
			if artifact["spec_path"] != specPath {
				t.Errorf("Artifact spec_path mismatch: got %v, want %s", artifact["spec_path"], specPath)
			}
		}
	}

	// Verify transaction directory was cleaned up
	txnDir := filepath.Join(".deespec/var/txn")
	entries, err := os.ReadDir(txnDir)
	if err == nil && len(entries) > 0 {
		t.Errorf("Transaction directory not cleaned up: %d entries remaining", len(entries))
	}
}

func TestExecuteRegisterTransactionFailure(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "register_tx_fail_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	// Create test spec with empty ID (should fail validation)
	spec := &dto.RegisterSpec{
		ID:     "", // Empty ID should cause validation failure
		Title:  "Test",
		Labels: []string{},
	}

	specPath := ".deespec/specs/test"

	journalEntry := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       0,
		"step":       "register",
		"decision":   "OK",
		"elapsed_ms": 0,
		"error":      "",
		"artifacts":  []map[string]interface{}{},
	}

	// Create service
	noopWarn := func(format string, args ...interface{}) {}
	service := NewRegisterTransactionService("", "", nil, noopWarn)

	// This should fail due to empty ID
	ctx := context.Background()
	err = service.ExecuteRegisterTransaction(ctx, spec, specPath, journalEntry)
	if err == nil {
		t.Error("Expected transaction to fail with empty ID")
	}

	// Verify no files were created
	metaPath := filepath.Join(specPath, "meta.yaml")
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("meta.yaml should not exist after failed transaction")
	}

	// Verify journal was not modified
	journalPath := filepath.Join(".deespec/var", "journal.ndjson")
	if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
		t.Error("journal should not exist after failed transaction")
	}
}

func TestExecuteRegisterTransactionEmptySpecPath(t *testing.T) {
	// Test that empty specPath causes early validation failure
	spec := &dto.RegisterSpec{
		ID:     "test-001",
		Title:  "Test",
		Labels: []string{},
	}

	specPath := "" // Empty spec path

	journalEntry := map[string]interface{}{}

	noopWarn := func(format string, args ...interface{}) {}
	service := NewRegisterTransactionService("", "", nil, noopWarn)

	ctx := context.Background()
	err := service.ExecuteRegisterTransaction(ctx, spec, specPath, journalEntry)
	if err == nil {
		t.Error("Expected transaction to fail with empty spec path")
	}
}

// Helper function to split lines
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
