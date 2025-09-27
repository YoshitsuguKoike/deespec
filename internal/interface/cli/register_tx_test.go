package cli

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRegisterWithTransaction(t *testing.T) {
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
	spec := &RegisterSpec{
		ID:     "test-001",
		Title:  "Test Specification",
		Labels: []string{"test", "transaction"},
	}

	// Create test result
	result := &RegisterResult{
		OK:       true,
		ID:       spec.ID,
		SpecPath: "test-001_test_specification",
		Warnings: []string{},
	}

	// Create test config
	config := &ResolvedConfig{
		JournalRecordSource:     true,
		JournalRecordInputBytes: false,
		InputSource:             "test",
		InputBytes:              100,
		PathMaxBytes:            240,
		SlugMaxRunes:            60,
		PathBaseDir:             ".deespec/specs",
	}

	// Create journal entry
	turn := 0
	journalEntry := buildJournalEntry(spec, result, config, turn)

	// Execute transaction
	err = registerWithTransaction(spec, result, config, journalEntry)
	if err != nil {
		t.Fatalf("Transaction failed: %v", err)
	}

	// Verify meta.yaml was created
	metaPath := filepath.Join(".deespec/specs", result.SpecPath, "meta.yaml")
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
	specPath := filepath.Join(".deespec/specs", result.SpecPath, "spec.md")
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
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
			if artifact["spec_path"] != result.SpecPath {
				t.Errorf("Artifact spec_path mismatch: got %v, want %s", artifact["spec_path"], result.SpecPath)
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

func TestRegisterWithTransactionFailure(t *testing.T) {
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

	// Create test spec with invalid characters that will fail
	spec := &RegisterSpec{
		ID:     "", // Empty ID should cause validation failure
		Title:  "Test",
		Labels: []string{},
	}

	result := &RegisterResult{
		OK:       true,
		ID:       spec.ID,
		SpecPath: "test",
		Warnings: []string{},
	}

	config := &ResolvedConfig{
		PathMaxBytes: 240,
		SlugMaxRunes: 60,
		PathBaseDir:  ".deespec/specs",
	}

	journalEntry := buildJournalEntry(spec, result, config, 0)

	// This should fail due to empty ID
	err = registerWithTransaction(spec, result, config, journalEntry)
	if err == nil {
		t.Error("Expected transaction to fail with empty ID")
	}

	// Verify no files were created
	metaPath := filepath.Join(".deespec/specs", result.SpecPath, "meta.yaml")
	if _, err := os.Stat(metaPath); !os.IsNotExist(err) {
		t.Error("meta.yaml should not exist after failed transaction")
	}

	// Verify journal was not modified
	journalPath := filepath.Join(".deespec/var", "journal.ndjson")
	if _, err := os.Stat(journalPath); !os.IsNotExist(err) {
		t.Error("journal should not exist after failed transaction")
	}
}

func TestTransactionWithCrashRecovery(t *testing.T) {
	// This test simulates a crash during commit and verifies recovery
	// For now, we just verify the intent marker creation

	tempDir, err := os.MkdirTemp("", "register_tx_crash_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	oldWd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldWd)

	// Manually create a transaction with intent but no commit
	txnID := "txn_test_recovery"
	txnDir := filepath.Join(".deespec/var/txn", txnID)
	os.MkdirAll(txnDir, 0755)

	// Create intent marker
	intentPath := filepath.Join(txnDir, "status.intent")
	intentData := `{"txn_id":"txn_test_recovery","marked_at":"2025-09-27T05:00:00Z","checksums":{},"ready":true}`
	os.WriteFile(intentPath, []byte(intentData), 0644)

	// Create manifest
	manifestPath := filepath.Join(txnDir, "manifest.json")
	manifestData := `{"id":"txn_test_recovery","description":"Test","files":[],"created_at":"2025-09-27T05:00:00Z"}`
	os.WriteFile(manifestPath, []byte(manifestData), 0644)

	// Verify scanner detects it
	scanner := NewScanner(".deespec/var/txn")
	result, err := scanner.Scan()
	if err != nil {
		t.Fatalf("Scanner failed: %v", err)
	}

	if len(result.IntentOnly) != 1 {
		t.Errorf("Expected 1 intent-only transaction, got %d", len(result.IntentOnly))
	}

	// In Step 8, this would trigger forward recovery
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

// NewScanner creates a test scanner (import from txn package needed in real code)
func NewScanner(baseDir string) interface{ Scan() (*ScanResult, error) } {
	// This would import from txn package in real implementation
	return nil
}

type ScanResult struct {
	TotalFound int
	IntentOnly []string
}
