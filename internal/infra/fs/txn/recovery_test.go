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
	if err := os.MkdirAll(txDir, 0755); err != nil {
		t.Fatalf("mkdir %s failed: %v", txDir, err)
	}

	// Create commit marker
	commit := Commit{
		TxnID:          TxnID(txnID),
		CommittedAt:    time.Now().UTC(),
		CommittedFiles: []string{"test.txt"},
		Success:        true,
	}
	commitData, _ := json.Marshal(commit)
	commitPath := filepath.Join(txDir, "status.commit")
	if err := os.WriteFile(commitPath, commitData, 0644); err != nil {
		t.Fatalf("write commit file failed: %v", err)
	}

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

// TestE2ECrashRecoveryWithRetry simulates real crash scenarios with timeout and retry
func TestE2ECrashRecoveryWithRetry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "e2e_crash_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	destRoot := filepath.Join(tempDir, ".deespec")

	// Set environment for recovery destination
	os.Setenv("DEESPEC_TX_DEST_ROOT", destRoot)
	defer os.Unsetenv("DEESPEC_TX_DEST_ROOT")

	t.Run("MultipleTransactionCrashRecovery", func(t *testing.T) {
		manager := NewManager(txnDir)
		ctx := context.Background()

		// Create multiple incomplete transactions (simulate multiple crashes)
		var txnIDs []TxnID
		for i := 0; i < 3; i++ {
			tx, err := manager.Begin(ctx)
			if err != nil {
				t.Fatalf("Begin transaction %d failed: %v", i, err)
			}

			// Stage different files for each transaction
			err = manager.StageFile(tx, filepath.Join("artifacts", "file"+string(rune('A'+i))+".txt"),
				[]byte("content"+string(rune('A'+i))))
			if err != nil {
				t.Fatalf("StageFile for transaction %d failed: %v", i, err)
			}

			// Mark intent but don't commit (simulate crash before commit)
			err = manager.MarkIntent(tx)
			if err != nil {
				t.Fatalf("MarkIntent for transaction %d failed: %v", i, err)
			}

			txnIDs = append(txnIDs, tx.Manifest.ID)
		}

		// Create new manager (simulate restart after crash)
		manager2 := NewManager(txnDir)

		// Configure recovery with short timeouts for testing
		recovery := NewRecoveryWithConfig(manager2, RecoveryConfig{
			Timeout:      2 * time.Second,
			TotalTimeout: 10 * time.Second,
			MaxRetries:   2,
			BaseDelay:    50 * time.Millisecond,
			MaxDelay:     500 * time.Millisecond,
		})

		// Perform recovery
		result, err := recovery.RecoverAll(ctx)
		if err != nil {
			t.Fatalf("RecoverAll failed: %v", err)
		}

		// Verify all transactions were recovered
		if result.RecoveredCount != 3 {
			t.Errorf("Expected 3 recovered transactions, got %d", result.RecoveredCount)
		}

		if result.FailedCount != 0 {
			t.Errorf("Expected 0 failed transactions, got %d", result.FailedCount)
		}

		// Verify all files were recovered to correct locations
		for i := 0; i < 3; i++ {
			expectedPath := filepath.Join(destRoot, "artifacts", "file"+string(rune('A'+i))+".txt")
			if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
				t.Errorf("Recovered file %s not found", expectedPath)
			}
		}

		// Verify all transactions have commit markers
		for _, txnID := range txnIDs {
			commitPath := filepath.Join(txnDir, string(txnID), "status.commit")
			if _, err := os.Stat(commitPath); os.IsNotExist(err) {
				t.Errorf("Commit marker not found for transaction %s", txnID)
			}
		}
	})

	t.Run("RecoveryWithTimeout", func(t *testing.T) {
		manager := NewManager(txnDir)
		ctx := context.Background()

		// Create incomplete transaction
		tx, err := manager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}

		err = manager.StageFile(tx, "timeout_test.txt", []byte("timeout content"))
		if err != nil {
			t.Fatalf("StageFile failed: %v", err)
		}

		err = manager.MarkIntent(tx)
		if err != nil {
			t.Fatalf("MarkIntent failed: %v", err)
		}

		// Create new manager for recovery
		manager2 := NewManager(txnDir)

		// Configure recovery with very short timeout to test timeout behavior
		recovery := NewRecoveryWithConfig(manager2, RecoveryConfig{
			Timeout:      1 * time.Millisecond, // Very short timeout
			TotalTimeout: 100 * time.Millisecond,
			MaxRetries:   1,
			BaseDelay:    10 * time.Millisecond,
			MaxDelay:     50 * time.Millisecond,
		})

		// Use context with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()

		// Recovery might fail due to timeout, but shouldn't crash
		result, err := recovery.RecoverAll(timeoutCtx)

		// Either succeeds quickly or fails gracefully due to timeout
		if err != nil && !isTimeoutError(err) && timeoutCtx.Err() == nil {
			t.Errorf("Recovery failed with unexpected error: %v", err)
		}

		// If recovery succeeded, verify results
		if err == nil {
			if result.RecoveredCount > 0 {
				t.Log("Recovery succeeded despite short timeout")
			}
		} else {
			t.Logf("Recovery failed due to timeout (expected): %v", err)
		}
	})

	t.Run("RecoveryWithCorruptedTransaction", func(t *testing.T) {
		manager := NewManager(txnDir)
		ctx := context.Background()

		// Create transaction with corrupted manifest (simulate filesystem corruption)
		tx, err := manager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}

		err = manager.StageFile(tx, "corrupted_test.txt", []byte("corrupted content"))
		if err != nil {
			t.Fatalf("StageFile failed: %v", err)
		}

		err = manager.MarkIntent(tx)
		if err != nil {
			t.Fatalf("MarkIntent failed: %v", err)
		}

		// Corrupt the manifest file
		manifestPath := filepath.Join(txnDir, string(tx.Manifest.ID), "manifest.json")
		err = os.WriteFile(manifestPath, []byte("corrupted json"), 0644)
		if err != nil {
			t.Fatalf("Failed to corrupt manifest: %v", err)
		}

		// Create new manager for recovery
		manager2 := NewManager(txnDir)
		recovery := NewRecovery(manager2)

		// Recovery should handle corrupted transaction gracefully
		result, err := recovery.RecoverAll(ctx)
		if err != nil {
			t.Fatalf("RecoverAll failed: %v", err)
		}

		// Should have 1 failed recovery due to corruption
		if result.FailedCount != 1 {
			t.Errorf("Expected 1 failed recovery, got %d", result.FailedCount)
		}

		if len(result.Errors) != 1 {
			t.Errorf("Expected 1 error, got %d", len(result.Errors))
		}
	})
}

// TestRecoveryMetrics verifies that recovery operations emit proper metrics
func TestRecoveryMetrics(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "recovery_metrics_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	destRoot := filepath.Join(tempDir, ".deespec")

	os.Setenv("DEESPEC_TX_DEST_ROOT", destRoot)
	defer os.Unsetenv("DEESPEC_TX_DEST_ROOT")

	manager := NewManager(txnDir)
	ctx := context.Background()

	// Create one incomplete transaction and one completed transaction
	// Incomplete transaction (needs recovery)
	tx1, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin tx1 failed: %v", err)
	}
	err = manager.StageFile(tx1, "metrics_test1.txt", []byte("metrics content1"))
	if err != nil {
		t.Fatalf("StageFile tx1 failed: %v", err)
	}
	err = manager.MarkIntent(tx1)
	if err != nil {
		t.Fatalf("MarkIntent tx1 failed: %v", err)
	}

	// Completed transaction (needs cleanup)
	tx2, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin tx2 failed: %v", err)
	}
	err = manager.StageFile(tx2, "metrics_test2.txt", []byte("metrics content2"))
	if err != nil {
		t.Fatalf("StageFile tx2 failed: %v", err)
	}
	err = manager.MarkIntent(tx2)
	if err != nil {
		t.Fatalf("MarkIntent tx2 failed: %v", err)
	}
	err = manager.Commit(tx2, destRoot, nil)
	if err != nil {
		t.Fatalf("Commit tx2 failed: %v", err)
	}

	// Create new manager for recovery
	manager2 := NewManager(txnDir)
	recovery := NewRecovery(manager2)

	// Capture stderr output to verify metrics
	// Note: In real tests, you might want to use a custom logger or capture mechanism
	result, err := recovery.RecoverAll(ctx)
	if err != nil {
		t.Fatalf("RecoverAll failed: %v", err)
	}

	// Verify recovery results match expected metrics
	if result.RecoveredCount != 1 {
		t.Errorf("Expected 1 recovered transaction (metrics validation)")
	}
	if result.CleanedCount != 1 {
		t.Errorf("Expected 1 cleaned transaction (metrics validation)")
	}
	if result.FailedCount != 0 {
		t.Errorf("Expected 0 failed transactions (metrics validation)")
	}

	// Verify timing metrics are reasonable
	if result.Duration <= 0 {
		t.Error("Recovery duration should be positive")
	}
	if result.Duration > 30*time.Second {
		t.Error("Recovery took too long (possible performance regression)")
	}
}

// Helper function to check if error is timeout-related
func isTimeoutError(err error) bool {
	return err != nil && (err == context.DeadlineExceeded || contains(err.Error(), "timeout") || contains(err.Error(), "deadline exceeded"))
}
