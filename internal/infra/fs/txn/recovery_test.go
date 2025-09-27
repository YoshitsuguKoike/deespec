package txn

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestForwardRecovery(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recovery_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create and partially complete a transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage files
	err = manager.StageFile(tx, "test1.txt", []byte("content1"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}
	err = manager.StageFile(tx, "test2.txt", []byte("content2"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	// Mark intent but don't commit (simulate crash)
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	txnID := tx.Manifest.ID

	// Simulate crash by creating new manager instance
	manager2 := NewManager(txnDir)
	recovery := NewRecovery(manager2)

	// Set destination root for test - files will be placed directly here
	destRoot := filepath.Join(tempDir, ".deespec")
	os.Setenv("DEESPEC_TX_DEST_ROOT", destRoot)
	defer os.Unsetenv("DEESPEC_TX_DEST_ROOT")

	// Perform recovery
	result, err := recovery.RecoverAll(ctx)
	if err != nil {
		t.Fatalf("RecoverAll failed: %v", err)
	}

	// Verify recovery results
	if result.RecoveredCount != 1 {
		t.Errorf("Expected 1 recovered, got %d", result.RecoveredCount)
	}

	// Verify files were moved to final destination
	finalPath1 := filepath.Join(tempDir, ".deespec", "test1.txt")
	if _, err := os.Stat(finalPath1); os.IsNotExist(err) {
		t.Error("test1.txt not found after recovery")
	}

	finalPath2 := filepath.Join(tempDir, ".deespec", "test2.txt")
	if _, err := os.Stat(finalPath2); os.IsNotExist(err) {
		t.Error("test2.txt not found after recovery")
	}

	// Verify commit marker was created
	commitPath := filepath.Join(txnDir, string(txnID), "status.commit")
	if _, err := os.Stat(commitPath); os.IsNotExist(err) {
		t.Errorf("Commit marker not created during recovery at %s", commitPath)
		// List contents to debug
		files, _ := os.ReadDir(filepath.Join(txnDir, string(txnID)))
		t.Logf("Transaction directory contents:")
		for _, f := range files {
			t.Logf("  - %s", f.Name())
		}
	}
}

func TestIdempotentCommit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "idempotent_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	err = manager.StageFile(tx, "test.txt", []byte("content"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// First commit
	err = manager.Commit(tx, tempDir, nil)
	if err != nil {
		t.Fatalf("First commit failed: %v", err)
	}

	// Second commit (should be idempotent)
	err = manager.Commit(tx, tempDir, nil)
	if err != nil {
		t.Errorf("Idempotent commit failed: %v", err)
	}

	// Status should still be commit
	if tx.Status != StatusCommit {
		t.Errorf("Expected status COMMIT, got %s", tx.Status)
	}
}

func TestCleanupAfterCommit(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cleanup_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	recovery := NewRecovery(manager)

	// Create a committed transaction
	txnID := "txn_test_cleanup"
	txDir := filepath.Join(txnDir, txnID)
	os.MkdirAll(txDir, 0755)

	// Create commit marker
	commit := Commit{
		TxnID:          TxnID(txnID),
		CommittedAt:    time.Now().UTC(),
		CommittedFiles: []string{"test.txt"},
		Success:        true,
	}
	commitData, _ := json.Marshal(commit)
	commitPath := filepath.Join(txDir, "status.commit")
	os.WriteFile(commitPath, commitData, 0644)

	// Run recovery (should cleanup committed transaction)
	ctx := context.Background()
	result, err := recovery.RecoverAll(ctx)
	if err != nil {
		t.Fatalf("RecoverAll failed: %v", err)
	}

	if result.CleanedCount != 1 {
		t.Errorf("Expected 1 cleaned, got %d", result.CleanedCount)
	}

	// Verify directory was removed
	if _, err := os.Stat(txDir); !os.IsNotExist(err) {
		t.Error("Transaction directory not removed after cleanup")
	}
}

func TestEarlyEXDEVDetection(t *testing.T) {
	// This test is platform-specific and may not work in all environments
	// It demonstrates the EXDEV detection logic

	tempDir, err := os.MkdirTemp("", "exdev_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// The EXDEV detection happens in StageFile
	// In a real cross-filesystem scenario, this would fail
	err = manager.StageFile(tx, "test.txt", []byte("content"))

	// On same filesystem, this should succeed
	if err != nil {
		// If it fails with EXDEV, that means detection is working
		if contains(err.Error(), "EXDEV") {
			t.Log("EXDEV detection working (cross-filesystem detected)")
		} else {
			t.Errorf("Unexpected error: %v", err)
		}
	} else {
		t.Log("Same filesystem - no EXDEV error (expected)")
	}
}
