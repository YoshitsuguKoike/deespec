package txn

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRollbackBasic(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rollback_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create a transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage a file
	testContent := []byte("test rollback content")
	err = manager.StageFile(tx, "test_file.txt", testContent)
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	// Verify staged file exists
	stagePath := filepath.Join(tx.StageDir, "test_file.txt")
	if _, err := os.Stat(stagePath); os.IsNotExist(err) {
		t.Error("Staged file should exist before rollback")
	}

	// Mark intent (but don't commit)
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// Verify intent marker exists
	intentPath := filepath.Join(tx.BaseDir, "status.intent")
	if _, err := os.Stat(intentPath); os.IsNotExist(err) {
		t.Error("Intent marker should exist before rollback")
	}

	// Perform rollback
	err = manager.Rollback(tx, "test rollback")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify transaction directory was cleaned up
	if _, err := os.Stat(tx.BaseDir); !os.IsNotExist(err) {
		t.Error("Transaction directory should not exist after rollback")
	}

	// Verify transaction status
	if tx.Status != StatusAborted {
		t.Errorf("Expected status ABORTED, got %s", tx.Status)
	}
}

func TestRollbackWithUndo(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rollback_undo_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create a transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Create original file to be overwritten
	originalFile := filepath.Join(tempDir, "original.txt")
	originalContent := []byte("original content")
	err = os.WriteFile(originalFile, originalContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create original file: %v", err)
	}

	// Set up undo information
	undoPath := filepath.Join(tx.UndoDir, "original.txt.backup")
	err = os.WriteFile(undoPath, originalContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create undo file: %v", err)
	}

	tx.Undo = &Undo{
		TxnID:      tx.Manifest.ID,
		PreparedAt: time.Now().UTC(),
		Valid:      true,
		RestoreOps: []RestoreOp{
			{
				Type:       "overwrite",
				TargetPath: originalFile,
				UndoPath:   undoPath,
			},
		},
	}

	// Overwrite the original file
	newContent := []byte("modified content")
	err = os.WriteFile(originalFile, newContent, 0644)
	if err != nil {
		t.Fatalf("Failed to overwrite file: %v", err)
	}

	// Verify file was overwritten
	data, err := os.ReadFile(originalFile)
	if err != nil {
		t.Fatalf("Failed to read overwritten file: %v", err)
	}
	if string(data) != "modified content" {
		t.Error("File should contain modified content before rollback")
	}

	// Perform rollback
	err = manager.Rollback(tx, "test undo rollback")
	if err != nil {
		t.Fatalf("Rollback with undo failed: %v", err)
	}

	// Verify file was restored to original content
	data, err = os.ReadFile(originalFile)
	if err != nil {
		t.Fatalf("Failed to read restored file: %v", err)
	}
	if string(data) != "original content" {
		t.Errorf("File should contain original content after rollback, got: %s", string(data))
	}

	// Verify transaction was cleaned up
	if _, err := os.Stat(tx.BaseDir); !os.IsNotExist(err) {
		t.Error("Transaction directory should not exist after rollback")
	}
}

func TestRollbackCommittedTransaction(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rollback_committed_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create and commit a transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

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

	// Try to rollback committed transaction (should fail)
	err = manager.Rollback(tx, "should fail")
	if err == nil {
		t.Error("Rollback of committed transaction should fail")
	}

	expectedError := "cannot rollback committed transaction"
	if !contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
	}

	// Transaction should still be committed
	if tx.Status != StatusCommit {
		t.Errorf("Transaction status should remain COMMIT, got %s", tx.Status)
	}
}

func TestRollbackMetrics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rollback_metrics_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create a transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	err = manager.StageFile(tx, "metrics_test.txt", []byte("metrics"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	// Capture start time for metrics validation
	startTime := time.Now()

	// Perform rollback
	err = manager.Rollback(tx, "metrics test")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Verify rollback completed within reasonable time
	elapsed := time.Since(startTime)
	if elapsed > 5*time.Second {
		t.Error("Rollback took too long (possible performance issue)")
	}

	// Verify transaction status
	if tx.Status != StatusAborted {
		t.Errorf("Expected status ABORTED for metrics test, got %s", tx.Status)
	}

	// Note: In a real implementation, you might capture stderr output
	// to verify specific metric log messages were written
}

// Helper functions are defined in test_helpers.go
