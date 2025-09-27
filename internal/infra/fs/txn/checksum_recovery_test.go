package txn

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestChecksumMismatchRecoveryIntegration tests recovery behavior when checksum validation fails
// This validates that intent→pre-commit checksum mismatch leads to safe recovery failure
func TestChecksumMismatchRecoveryIntegration(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "checksum_recovery_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	destRoot := filepath.Join(tempDir, ".deespec")

	manager := NewManager(txnDir)
	ctx := context.Background()

	// Test Scenario 1: Checksum mismatch during commit should fail gracefully
	t.Run("ChecksumMismatchFailsGracefully", func(t *testing.T) {
		// Create transaction
		tx, err := manager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}

		// Stage file with valid content
		originalContent := []byte("original content for checksum test")
		err = manager.StageFile(tx, "test_file.txt", originalContent)
		if err != nil {
			t.Fatalf("StageFile failed: %v", err)
		}

		// Mark intent
		err = manager.MarkIntent(tx)
		if err != nil {
			t.Fatalf("MarkIntent failed: %v", err)
		}

		// Simulate checksum corruption by modifying staged file after intent
		stagePath := filepath.Join(tx.StageDir, "test_file.txt")
		corruptedContent := []byte("corrupted content after intent")
		err = os.WriteFile(stagePath, corruptedContent, 0644)
		if err != nil {
			t.Fatalf("Failed to corrupt staged file: %v", err)
		}

		// Commit should fail due to checksum mismatch
		err = manager.Commit(tx, destRoot, nil)
		if err == nil {
			t.Error("Commit should fail when staged file is corrupted")
		}

		expectedError := "staged file checksum validation failed"
		if !containsHelper(err.Error(), expectedError) {
			t.Errorf("Expected error containing '%s', got: %v", expectedError, err)
		}

		// Verify transaction directory still exists (not cleaned up on failure)
		if _, err := os.Stat(tx.BaseDir); err != nil {
			t.Errorf("Transaction directory should exist after failed commit: %v", err)
		}

		// Verify no files were created in destination
		destFile := filepath.Join(destRoot, "test_file.txt")
		if _, err := os.Stat(destFile); !os.IsNotExist(err) {
			t.Error("Destination file should not exist after failed commit")
		}
	})

	// Test Scenario 2: Recovery should detect and handle corrupted transactions safely
	t.Run("RecoveryDetectsCorruptedTransaction", func(t *testing.T) {
		// Create transaction with corrupted staged file
		tx, err := manager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}

		// Stage file
		testContent := []byte("recovery test content")
		err = manager.StageFile(tx, "recovery_test.txt", testContent)
		if err != nil {
			t.Fatalf("StageFile failed: %v", err)
		}

		// Mark intent
		err = manager.MarkIntent(tx)
		if err != nil {
			t.Fatalf("MarkIntent failed: %v", err)
		}

		// Corrupt staged file to simulate data corruption
		stagePath := filepath.Join(tx.StageDir, "recovery_test.txt")
		corruptedContent := []byte("data corruption simulation")
		err = os.WriteFile(stagePath, corruptedContent, 0644)
		if err != nil {
			t.Fatalf("Failed to corrupt staged file: %v", err)
		}

		// Simulate system crash by not completing commit
		// (transaction remains in intent state with corrupted data)

		// Run recovery
		recovery := NewRecovery(manager)
		recoveryResult, err := recovery.RecoverAll(ctx)
		if err != nil {
			t.Fatalf("RecoverAll failed: %v", err)
		}

		// Recovery should detect corruption and not complete the transaction
		if recoveryResult.RecoveredCount > 0 {
			t.Errorf("Recovery should not complete corrupted transactions, but recovered %d", recoveryResult.RecoveredCount)
		}

		if recoveryResult.FailedCount == 0 {
			t.Error("Recovery should report failed transactions due to checksum corruption")
		}

		// Verify no files were created in destination after recovery attempt
		destFile := filepath.Join(destRoot, "recovery_test.txt")
		if _, err := os.Stat(destFile); !os.IsNotExist(err) {
			t.Error("Destination file should not exist after failed recovery")
		}

		// Verify transaction directory still exists (safe failure)
		if _, err := os.Stat(tx.BaseDir); err != nil {
			t.Errorf("Transaction directory should remain for manual investigation: %v", err)
		}
	})

	// Test Scenario 3: Parallel checksum validation during recovery
	t.Run("ParallelChecksumValidationDuringRecovery", func(t *testing.T) {
		// Create multiple transactions with mixed valid/invalid checksums
		var transactions []*Transaction

		// Create 3 transactions
		for i := 0; i < 3; i++ {
			tx, err := manager.Begin(ctx)
			if err != nil {
				t.Fatalf("Begin failed for tx %d: %v", i, err)
			}
			transactions = append(transactions, tx)

			// Stage file
			content := []byte("parallel test content " + string(rune('A'+i)))
			fileName := "parallel_test_" + string(rune('A'+i)) + ".txt"
			err = manager.StageFile(tx, fileName, content)
			if err != nil {
				t.Fatalf("StageFile failed for tx %d: %v", i, err)
			}

			// Mark intent
			err = manager.MarkIntent(tx)
			if err != nil {
				t.Fatalf("MarkIntent failed for tx %d: %v", i, err)
			}

			// Corrupt the middle transaction (index 1)
			if i == 1 {
				stagePath := filepath.Join(tx.StageDir, fileName)
				corruptedContent := []byte("corrupted parallel content")
				err = os.WriteFile(stagePath, corruptedContent, 0644)
				if err != nil {
					t.Fatalf("Failed to corrupt tx %d: %v", i, err)
				}
			}
		}

		// Run parallel checksum validation through recovery
		recovery := NewRecovery(manager)
		recoveryResult, err := recovery.RecoverAll(ctx)
		if err != nil {
			t.Fatalf("RecoverAll failed: %v", err)
		}

		// Should recover 2 transactions and fail 1
		expectedRecovered := 2
		expectedFailed := 1

		if recoveryResult.RecoveredCount != expectedRecovered {
			t.Errorf("Expected %d recovered transactions, got %d", expectedRecovered, recoveryResult.RecoveredCount)
		}

		if recoveryResult.FailedCount != expectedFailed {
			t.Errorf("Expected %d failed transactions, got %d", expectedFailed, recoveryResult.FailedCount)
		}

		// Verify only valid transactions created destination files
		validFiles := []string{"parallel_test_A.txt", "parallel_test_C.txt"}
		for _, fileName := range validFiles {
			destFile := filepath.Join(destRoot, fileName)
			if _, err := os.Stat(destFile); err != nil {
				t.Errorf("Valid transaction file should exist: %s, error: %v", fileName, err)
			}
		}

		// Verify corrupted transaction did not create destination file
		corruptedFile := filepath.Join(destRoot, "parallel_test_B.txt")
		if _, err := os.Stat(corruptedFile); !os.IsNotExist(err) {
			t.Error("Corrupted transaction file should not exist: parallel_test_B.txt")
		}
	})

	// Test Scenario 4: Large transaction with worker pool checksum validation
	t.Run("LargeTransactionWorkerPoolValidation", func(t *testing.T) {
		// Create transaction with multiple files
		tx, err := manager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}

		// Stage multiple files for worker pool testing
		fileCount := 8
		for i := 0; i < fileCount; i++ {
			content := []byte("worker pool test content " + string(rune('0'+i)))
			fileName := "worker_test_" + string(rune('0'+i)) + ".txt"
			err = manager.StageFile(tx, fileName, content)
			if err != nil {
				t.Fatalf("StageFile failed for file %d: %v", i, err)
			}
		}

		// Mark intent
		err = manager.MarkIntent(tx)
		if err != nil {
			t.Fatalf("MarkIntent failed: %v", err)
		}

		// Use parallel checksum validation
		var filePaths []string
		for i := 0; i < fileCount; i++ {
			fileName := "worker_test_" + string(rune('0'+i)) + ".txt"
			filePaths = append(filePaths, filepath.Join(tx.StageDir, fileName))
		}

		// Test parallel checksum calculation
		results := CalculateChecksumsParallel(filePaths, ChecksumSHA256, 3)

		// Verify all checksums were calculated successfully
		if len(results) != fileCount {
			t.Errorf("Expected %d checksum results, got %d", fileCount, len(results))
		}

		for filePath, result := range results {
			if result.Error != nil {
				t.Errorf("Checksum calculation failed for %s: %v", filePath, result.Error)
			}
			if result.Checksum == nil {
				t.Errorf("Checksum should not be nil for %s", filePath)
			}
		}

		// Complete transaction
		err = manager.Commit(tx, destRoot, nil)
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify all files were created
		for i := 0; i < fileCount; i++ {
			fileName := "worker_test_" + string(rune('0'+i)) + ".txt"
			destFile := filepath.Join(destRoot, fileName)
			if _, err := os.Stat(destFile); err != nil {
				t.Errorf("Worker pool transaction file should exist: %s, error: %v", fileName, err)
			}
		}
	})
}

// Helper function for string contains check (local version to avoid conflicts)
func containsHelper(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || hasSubstringHelper(s, substr))
}

func hasSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
