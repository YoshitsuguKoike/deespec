package txn

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTransactionLifecycle(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "txn_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec", "var", "txn")
	manager := NewManager(txnDir)

	ctx := context.Background()

	// Test Begin
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Verify initial state
	if tx.Status != StatusPending {
		t.Errorf("Expected status PENDING, got %s", tx.Status)
	}

	// Verify directories created
	if _, err := os.Stat(tx.BaseDir); os.IsNotExist(err) {
		t.Error("Base directory not created")
	}
	if _, err := os.Stat(tx.StageDir); os.IsNotExist(err) {
		t.Error("Stage directory not created")
	}

	// Verify manifest saved
	manifestPath := filepath.Join(tx.BaseDir, "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Error("Manifest file not created")
	}

	// Test StageFile
	testContent := []byte("test file content")
	testDst := "test/file.txt"

	err = manager.StageFile(tx, testDst, testContent)
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	// Verify staged file exists
	stagedPath := filepath.Join(tx.StageDir, testDst)
	if content, err := os.ReadFile(stagedPath); err != nil {
		t.Errorf("Failed to read staged file: %v", err)
	} else if string(content) != string(testContent) {
		t.Errorf("Staged content mismatch: got %q, want %q", content, testContent)
	}

	// Verify manifest updated
	if len(tx.Manifest.Files) != 1 {
		t.Errorf("Expected 1 file in manifest, got %d", len(tx.Manifest.Files))
	}

	// Test MarkIntent
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// Verify status changed
	if tx.Status != StatusIntent {
		t.Errorf("Expected status INTENT, got %s", tx.Status)
	}

	// Verify intent marker created
	intentPath := filepath.Join(tx.BaseDir, "status.intent")
	if _, err := os.Stat(intentPath); os.IsNotExist(err) {
		t.Error("Intent marker not created")
	}

	// Test Commit
	journalCalled := false
	err = manager.Commit(tx, tempDir, func() error {
		journalCalled = true
		return nil
	})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify journal callback was called
	if !journalCalled {
		t.Error("Journal callback not called during commit")
	}

	// Verify status changed
	if tx.Status != StatusCommit {
		t.Errorf("Expected status COMMIT, got %s", tx.Status)
	}

	// Verify commit marker created
	commitPath := filepath.Join(tx.BaseDir, "status.commit")
	if _, err := os.Stat(commitPath); os.IsNotExist(err) {
		t.Error("Commit marker not created")
	}

	// Verify file renamed to final destination
	finalPath := filepath.Join(tempDir, testDst)
	if content, err := os.ReadFile(finalPath); err != nil {
		t.Errorf("Failed to read final file: %v", err)
	} else if string(content) != string(testContent) {
		t.Errorf("Final content mismatch: got %q, want %q", content, testContent)
	}

	// Test Cleanup
	err = manager.Cleanup(tx)
	if err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}

	// Verify transaction directory removed
	if _, err := os.Stat(tx.BaseDir); !os.IsNotExist(err) {
		t.Error("Transaction directory not removed after cleanup")
	}
}

func TestTransactionStateValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_validation_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec", "var", "txn")
	manager := NewManager(txnDir)

	ctx := context.Background()

	// Create transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Test invalid state transitions
	t.Run("Cannot commit without intent", func(t *testing.T) {
		err := manager.Commit(tx, tempDir, nil)
		if err == nil {
			t.Error("Expected error when committing without intent")
		}
	})

	// Stage a dummy file first
	err = manager.StageFile(tx, "dummy.txt", []byte("dummy content"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	t.Run("Cannot stage after intent", func(t *testing.T) {
		err := manager.StageFile(tx, "another.txt", []byte("data"))
		if err == nil {
			t.Error("Expected error when staging after intent")
		}
	})

	t.Run("Cannot mark intent twice", func(t *testing.T) {
		err := manager.MarkIntent(tx)
		if err == nil {
			t.Error("Expected error when marking intent twice")
		}
	})

	// Commit the transaction
	err = manager.Commit(tx, tempDir, nil)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	t.Run("Idempotent commit", func(t *testing.T) {
		// Per Step 6 feedback: commits should be idempotent
		err := manager.Commit(tx, tempDir, nil)
		if err != nil {
			t.Errorf("Idempotent commit should succeed, got error: %v", err)
		}
		// Verify status is still commit
		if tx.Status != StatusCommit {
			t.Errorf("Expected status COMMIT after idempotent commit, got %s", tx.Status)
		}
	})

	t.Run("Cannot cleanup uncommitted transaction", func(t *testing.T) {
		// Create new transaction
		tx2, _ := manager.Begin(ctx)
		err := manager.Cleanup(tx2)
		if err == nil {
			t.Error("Expected error when cleaning up uncommitted transaction")
		}
	})
}

func TestMultipleFileStaging(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_multi_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec", "var", "txn")
	manager := NewManager(txnDir)

	ctx := context.Background()

	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage multiple files
	files := map[string]string{
		"dir1/file1.txt": "content1",
		"dir1/file2.txt": "content2",
		"dir2/file3.txt": "content3",
	}

	for dst, content := range files {
		err := manager.StageFile(tx, dst, []byte(content))
		if err != nil {
			t.Fatalf("Failed to stage %s: %v", dst, err)
		}
	}

	// Verify all files staged
	if len(tx.Manifest.Files) != 3 {
		t.Errorf("Expected 3 files in manifest, got %d", len(tx.Manifest.Files))
	}

	// Mark intent and commit
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	err = manager.Commit(tx, tempDir, nil)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Verify all files in final location
	for dst, expectedContent := range files {
		finalPath := filepath.Join(tempDir, dst)
		content, err := os.ReadFile(finalPath)
		if err != nil {
			t.Errorf("Failed to read %s: %v", dst, err)
		} else if string(content) != expectedContent {
			t.Errorf("Content mismatch for %s: got %q, want %q", dst, content, expectedContent)
		}
	}
}

func TestManifestValidation(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_validate_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec", "var", "txn")
	manager := NewManager(txnDir)

	ctx := context.Background()

	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Try to mark intent with empty manifest (no files)
	err = manager.MarkIntent(tx)
	if err == nil {
		t.Error("Expected error when marking intent with no files")
	}
}

func TestIntentMarkerFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_format_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec", "var", "txn")
	manager := NewManager(txnDir)

	ctx := context.Background()

	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage a file and mark intent
	err = manager.StageFile(tx, "test.txt", []byte("test"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// Read and verify intent marker format
	intentPath := filepath.Join(tx.BaseDir, "status.intent")
	data, err := os.ReadFile(intentPath)
	if err != nil {
		t.Fatalf("Failed to read intent marker: %v", err)
	}

	var intent Intent
	err = json.Unmarshal(data, &intent)
	if err != nil {
		t.Fatalf("Failed to parse intent marker: %v", err)
	}

	// Verify required fields
	if intent.TxnID == "" {
		t.Error("Intent missing TxnID")
	}
	if intent.MarkedAt.IsZero() {
		t.Error("Intent missing MarkedAt timestamp")
	}
	if !intent.Ready {
		t.Error("Intent Ready flag should be true")
	}

	// Verify timestamp is in UTC
	if intent.MarkedAt.Location() != time.UTC {
		t.Error("Intent timestamp should be in UTC")
	}
}

func TestCommitMarkerFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "txn_commit_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec", "var", "txn")
	manager := NewManager(txnDir)

	ctx := context.Background()

	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Complete transaction
	err = manager.StageFile(tx, "test.txt", []byte("test"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	err = manager.Commit(tx, tempDir, nil)
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Read and verify commit marker format
	commitPath := filepath.Join(tx.BaseDir, "status.commit")
	data, err := os.ReadFile(commitPath)
	if err != nil {
		t.Fatalf("Failed to read commit marker: %v", err)
	}

	var commit Commit
	err = json.Unmarshal(data, &commit)
	if err != nil {
		t.Fatalf("Failed to parse commit marker: %v", err)
	}

	// Verify required fields
	if commit.TxnID == "" {
		t.Error("Commit missing TxnID")
	}
	if commit.CommittedAt.IsZero() {
		t.Error("Commit missing CommittedAt timestamp")
	}
	if !commit.Success {
		t.Error("Commit Success flag should be true")
	}
	if len(commit.CommittedFiles) != 1 {
		t.Errorf("Expected 1 committed file, got %d", len(commit.CommittedFiles))
	}

	// Verify timestamp is in UTC
	if commit.CommittedAt.Location() != time.UTC {
		t.Error("Commit timestamp should be in UTC")
	}
}
