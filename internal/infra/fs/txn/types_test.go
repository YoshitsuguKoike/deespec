package txn

import (
	"errors"
	"testing"
	"time"
)

func TestTxnStatus(t *testing.T) {
	// Test status constants are defined
	statuses := []Status{
		StatusPending,
		StatusIntent,
		StatusCommit,
		StatusAborted,
		StatusFailed,
	}

	expectedValues := []string{
		"pending",
		"intent",
		"commit",
		"aborted",
		"failed",
	}

	for i, status := range statuses {
		if string(status) != expectedValues[i] {
			t.Errorf("Status mismatch: got %s, want %s", status, expectedValues[i])
		}
	}
}

func TestFileOperation(t *testing.T) {
	// Test FileOperation struct
	op := FileOperation{
		Type:        "create",
		Destination: "testdata/path/to/file.txt",
		Checksum:    "abc123",
		Size:        1024,
		Mode:        0644,
	}

	if op.Type != "create" {
		t.Errorf("Type mismatch: got %s, want create", op.Type)
	}
	if op.Destination != "testdata/path/to/file.txt" {
		t.Errorf("Destination mismatch: got %s, want testdata/path/to/file.txt", op.Destination)
	}
	if op.Size != 1024 {
		t.Errorf("Size mismatch: got %d, want 1024", op.Size)
	}

	// Test rename operation
	renameOp := FileOperation{
		Type:        "rename",
		Source:      "testdata/old/path.txt",
		Destination: "testdata/new/path.txt",
	}

	if renameOp.Source != "testdata/old/path.txt" {
		t.Errorf("Source mismatch: got %s, want testdata/old/path.txt", renameOp.Source)
	}
}

func TestManifest(t *testing.T) {
	now := time.Now()
	deadline := now.Add(1 * time.Hour)

	manifest := Manifest{
		ID:          TxnID("txn_12345_abcdef"),
		Description: "Test transaction",
		Files: []FileOperation{
			{
				Type:        "create",
				Destination: "testdata/test/file1.txt",
				Size:        100,
			},
			{
				Type:        "update",
				Destination: "testdata/test/file2.txt",
				Size:        200,
			},
		},
		CreatedAt: now,
		Deadline:  &deadline,
		Meta: map[string]interface{}{
			"user":    "testuser",
			"command": "test-command",
		},
	}

	if manifest.ID != "txn_12345_abcdef" {
		t.Errorf("ID mismatch: got %s, want txn_12345_abcdef", manifest.ID)
	}
	if len(manifest.Files) != 2 {
		t.Errorf("Files count mismatch: got %d, want 2", len(manifest.Files))
	}
	if manifest.Meta["user"] != "testuser" {
		t.Errorf("Meta user mismatch: got %v, want testuser", manifest.Meta["user"])
	}
}

func TestIntent(t *testing.T) {
	intent := Intent{
		TxnID:    TxnID("txn_test_123"),
		MarkedAt: time.Now(),
		Checksums: map[string]string{
			"testdata/file1": "checksum1",
			"testdata/file2": "checksum2",
		},
		Ready: true,
	}

	if !intent.Ready {
		t.Error("Intent should be ready")
	}
	if len(intent.Checksums) != 2 {
		t.Errorf("Checksums count mismatch: got %d, want 2", len(intent.Checksums))
	}
	if intent.Checksums["testdata/file1"] != "checksum1" {
		t.Errorf("Checksum mismatch: got %s, want checksum1", intent.Checksums["testdata/file1"])
	}
}

func TestCommit(t *testing.T) {
	commit := Commit{
		TxnID:       TxnID("txn_commit_test"),
		CommittedAt: time.Now(),
		JournalSeq:  42,
		CommittedFiles: []string{
			"testdata/path/file1",
			"testdata/path/file2",
			"testdata/path/file3",
		},
		Success: true,
	}

	if !commit.Success {
		t.Error("Commit should be successful")
	}
	if commit.JournalSeq != 42 {
		t.Errorf("JournalSeq mismatch: got %d, want 42", commit.JournalSeq)
	}
	if len(commit.CommittedFiles) != 3 {
		t.Errorf("CommittedFiles count mismatch: got %d, want 3", len(commit.CommittedFiles))
	}
}

func TestTransaction(t *testing.T) {
	tx := Transaction{
		Manifest: &Manifest{
			ID:          TxnID("txn_full_test"),
			Description: "Full transaction test",
			Files:       []FileOperation{},
			CreatedAt:   time.Now(),
		},
		Status:   StatusPending,
		BaseDir:  ".deespec/var/txn/txn_full_test/",
		StageDir: ".deespec/var/txn/txn_full_test/stage/",
		UndoDir:  ".deespec/var/txn/txn_full_test/undo/",
	}

	if tx.Status != StatusPending {
		t.Errorf("Status mismatch: got %s, want %s", tx.Status, StatusPending)
	}
	if tx.Manifest.ID != "txn_full_test" {
		t.Errorf("Manifest ID mismatch: got %s, want txn_full_test", tx.Manifest.ID)
	}
	if tx.BaseDir != ".deespec/var/txn/txn_full_test/" {
		t.Errorf("BaseDir mismatch: got %s", tx.BaseDir)
	}
}

func TestRecoveryInfo(t *testing.T) {
	recovery := RecoveryInfo{
		IncompleteTxns: []TxnID{
			"txn_incomplete_1",
			"txn_incomplete_2",
		},
		IntentOnly: []TxnID{
			"txn_intent_1",
		},
		PartialStage: []TxnID{
			"txn_partial_1",
		},
		CleanupReady: []TxnID{
			"txn_cleanup_1",
			"txn_cleanup_2",
			"txn_cleanup_3",
		},
		ScannedAt: time.Now(),
	}

	if len(recovery.IncompleteTxns) != 2 {
		t.Errorf("IncompleteTxns count mismatch: got %d, want 2", len(recovery.IncompleteTxns))
	}
	if len(recovery.IntentOnly) != 1 {
		t.Errorf("IntentOnly count mismatch: got %d, want 1", len(recovery.IntentOnly))
	}
	if len(recovery.CleanupReady) != 3 {
		t.Errorf("CleanupReady count mismatch: got %d, want 3", len(recovery.CleanupReady))
	}
}

func TestTxnError(t *testing.T) {
	baseErr := errors.New("disk full")
	txnErr := &TxnError{
		TxnID:       TxnID("txn_error_test"),
		Operation:   "stage_file",
		Err:         baseErr,
		Recoverable: false,
	}

	// Test Error() method
	errStr := txnErr.Error()
	expectedStr := "transaction txn_error_test: stage_file failed (unrecoverable): disk full"
	if errStr != expectedStr {
		t.Errorf("Error string mismatch:\ngot:  %s\nwant: %s", errStr, expectedStr)
	}

	// Test recoverable error
	recoverableErr := &TxnError{
		TxnID:       TxnID("txn_retry"),
		Operation:   "acquire_lock",
		Err:         errors.New("lock busy"),
		Recoverable: true,
	}

	errStr2 := recoverableErr.Error()
	if !contains(errStr2, "recoverable") {
		t.Errorf("Expected 'recoverable' in error string: %s", errStr2)
	}

	// Test Unwrap
	if txnErr.Unwrap() != baseErr {
		t.Error("Unwrap should return the base error")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(s) > len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
