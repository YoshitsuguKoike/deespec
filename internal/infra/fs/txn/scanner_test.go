package txn

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewScanner(t *testing.T) {
	// Test with default base directory
	s1 := NewScanner("")
	if s1.BaseDir != ".deespec/var/txn" {
		t.Errorf("Default BaseDir mismatch: got %s, want .deespec/var/txn", s1.BaseDir)
	}
	if s1.Logger == nil {
		t.Error("Logger should not be nil")
	}

	// Test with custom base directory
	s2 := NewScanner("testdata/custom/path")
	if s2.BaseDir != "testdata/custom/path" {
		t.Errorf("Custom BaseDir mismatch: got %s, want testdata/custom/path", s2.BaseDir)
	}
}

func TestScanNoDirectory(t *testing.T) {
	// Create scanner with non-existent directory
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "non-existent")

	scanner := NewScanner(nonExistentDir)
	result, err := scanner.Scan()

	if err != nil {
		t.Fatalf("Scan should not fail for non-existent directory: %v", err)
	}

	if result.TotalFound != 0 {
		t.Errorf("Expected 0 transactions, got %d", result.TotalFound)
	}

	if len(result.IntentOnly) != 0 || len(result.Committed) != 0 ||
		len(result.Incomplete) != 0 || len(result.Abandoned) != 0 {
		t.Error("All transaction lists should be empty")
	}
}

func TestScanWithTransactions(t *testing.T) {
	// Create temporary test directory structure
	tmpDir := t.TempDir()
	txnDir := filepath.Join(tmpDir, "txn")

	// Create base transaction directory
	if err := os.MkdirAll(txnDir, 0755); err != nil {
		t.Fatalf("Failed to create txn directory: %v", err)
	}

	// Create various transaction states
	testCases := []struct {
		txnID         string
		files         map[string]string // filename -> content
		expectedState string
	}{
		// Transaction with intent but no commit (needs forward recovery)
		{
			txnID: "txn_intent_only",
			files: map[string]string{
				"status.intent": "ready",
				"manifest.json": "{}",
			},
			expectedState: "intent_only",
		},
		// Committed transaction (ready for cleanup)
		{
			txnID: "txn_committed",
			files: map[string]string{
				"status.intent": "ready",
				"status.commit": "done",
				"manifest.json": "{}",
			},
			expectedState: "committed",
		},
		// Incomplete transaction (partial staging)
		{
			txnID: "txn_incomplete",
			files: map[string]string{
				"manifest.json": "{}",
			},
			expectedState: "incomplete",
		},
		// Abandoned transaction (no markers)
		{
			txnID:         "txn_abandoned",
			files:         map[string]string{},
			expectedState: "abandoned",
		},
	}

	for _, tc := range testCases {
		// Create transaction directory
		tcDir := filepath.Join(txnDir, tc.txnID)
		if err := os.MkdirAll(tcDir, 0755); err != nil {
			t.Fatalf("Failed to create txn dir %s: %v", tc.txnID, err)
		}

		// Create files
		for filename, content := range tc.files {
			filePath := filepath.Join(tcDir, filename)
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				t.Fatalf("Failed to create file %s: %v", filename, err)
			}
		}
	}

	// Also create stage directory for incomplete transaction
	stageDir := filepath.Join(txnDir, "txn_incomplete", "stage")
	if err := os.MkdirAll(stageDir, 0755); err != nil {
		t.Fatalf("Failed to create stage dir: %v", err)
	}

	// Run scan
	scanner := NewScanner(txnDir)
	result, err := scanner.Scan()

	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Verify results
	if result.TotalFound != 4 {
		t.Errorf("Expected 4 transactions, got %d", result.TotalFound)
	}

	if len(result.IntentOnly) != 1 {
		t.Errorf("Expected 1 intent_only transaction, got %d", len(result.IntentOnly))
	} else if result.IntentOnly[0] != "txn_intent_only" {
		t.Errorf("Expected txn_intent_only, got %s", result.IntentOnly[0])
	}

	if len(result.Committed) != 1 {
		t.Errorf("Expected 1 committed transaction, got %d", len(result.Committed))
	} else if result.Committed[0] != "txn_committed" {
		t.Errorf("Expected txn_committed, got %s", result.Committed[0])
	}

	if len(result.Incomplete) != 1 {
		t.Errorf("Expected 1 incomplete transaction, got %d", len(result.Incomplete))
	} else if result.Incomplete[0] != "txn_incomplete" {
		t.Errorf("Expected txn_incomplete, got %s", result.Incomplete[0])
	}

	if len(result.Abandoned) != 1 {
		t.Errorf("Expected 1 abandoned transaction, got %d", len(result.Abandoned))
	} else if result.Abandoned[0] != "txn_abandoned" {
		t.Errorf("Expected txn_abandoned, got %s", result.Abandoned[0])
	}

	// Check timestamp is recent
	if time.Since(result.ScannedAt) > time.Second {
		t.Error("ScannedAt timestamp seems outdated")
	}
}

func TestCheckTransactionState(t *testing.T) {
	tmpDir := t.TempDir()
	scanner := NewScanner(tmpDir)

	testCases := []struct {
		name     string
		setup    func(string) error
		expected string
	}{
		{
			name: "committed",
			setup: func(dir string) error {
				if err := os.WriteFile(filepath.Join(dir, "status.commit"), []byte("done"), 0644); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "status.intent"), []byte("ready"), 0644)
			},
			expected: "committed",
		},
		{
			name: "intent_only",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "status.intent"), []byte("ready"), 0644)
			},
			expected: "intent_only",
		},
		{
			name: "incomplete_with_manifest",
			setup: func(dir string) error {
				return os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0644)
			},
			expected: "incomplete",
		},
		{
			name: "incomplete_with_stage",
			setup: func(dir string) error {
				return os.MkdirAll(filepath.Join(dir, "stage"), 0755)
			},
			expected: "incomplete",
		},
		{
			name: "abandoned",
			setup: func(dir string) error {
				// Create empty directory
				return nil
			},
			expected: "abandoned",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tmpDir, tc.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test directory: %v", err)
			}

			// Setup test case
			if err := tc.setup(testDir); err != nil {
				t.Fatalf("Test setup failed: %v", err)
			}

			// Check state
			state := scanner.checkTransactionState(testDir)
			if state != tc.expected {
				t.Errorf("State mismatch: got %s, want %s", state, tc.expected)
			}
		})
	}
}

func TestFormatTxnIDs(t *testing.T) {
	testCases := []struct {
		name     string
		ids      []TxnID
		expected string
	}{
		{
			name:     "empty",
			ids:      []TxnID{},
			expected: "[]",
		},
		{
			name:     "single",
			ids:      []TxnID{"txn_001"},
			expected: "[txn_001]",
		},
		{
			name:     "three",
			ids:      []TxnID{"txn_001", "txn_002", "txn_003"},
			expected: "[txn_001,txn_002,txn_003]",
		},
		{
			name:     "many",
			ids:      []TxnID{"txn_001", "txn_002", "txn_003", "txn_004", "txn_005"},
			expected: "[txn_001,txn_002,txn_003,txn_004,txn_005]",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatTxnIDs(tc.ids)
			if result != tc.expected {
				t.Errorf("Format mismatch: got %s, want %s", result, tc.expected)
			}
		})
	}
}

func TestScanWithErrors(t *testing.T) {
	tmpDir := t.TempDir()
	txnDir := filepath.Join(tmpDir, "txn")

	// Create base directory
	if err := os.MkdirAll(txnDir, 0755); err != nil {
		t.Fatalf("Failed to create txn directory: %v", err)
	}

	// Create a file instead of directory to trigger walk error
	badPath := filepath.Join(txnDir, "bad_txn")
	if err := os.WriteFile(badPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("Failed to create bad file: %v", err)
	}

	// Create a proper transaction directory too
	goodTxnDir := filepath.Join(txnDir, "good_txn")
	if err := os.MkdirAll(goodTxnDir, 0755); err != nil {
		t.Fatalf("Failed to create good txn dir: %v", err)
	}

	scanner := NewScanner(txnDir)
	result, err := scanner.Scan()

	// Should not return error but collect errors in result
	if err != nil {
		t.Fatalf("Scan should not fail: %v", err)
	}

	// Should have found 1 transaction (good_txn)
	if result.TotalFound != 1 {
		t.Errorf("Expected 1 transaction found, got %d", result.TotalFound)
	}

	// Errors field might be empty since bad_txn is a file, not a dir
	// The walker will just skip it
}
