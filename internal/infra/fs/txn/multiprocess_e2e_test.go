package txn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestMultiProcessConcurrentMetrics tests that multiple processes can safely update metrics
func TestMultiProcessConcurrentMetrics(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-process E2E test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "multiprocess_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up shared metrics file
	metricsPath := filepath.Join(tempDir, "metrics.json")

	// Number of concurrent processes to simulate
	processCount := 3
	operationsPerProcess := 20

	// Channel to collect results from goroutines
	results := make(chan error, processCount)

	// Start multiple goroutines simulating separate processes
	var wg sync.WaitGroup
	for i := 0; i < processCount; i++ {
		wg.Add(1)
		go func(processID int) {
			defer wg.Done()

			// Simulate process-level operations
			for j := 0; j < operationsPerProcess; j++ {
				// Create new metrics instance (simulating separate process)
				metrics := &MetricsCollector{
					SchemaVersion: MetricsSchemaVersion,
				}

				// Simulate various operations
				switch j % 4 {
				case 0:
					metrics.IncrementCommitSuccess()
				case 1:
					metrics.IncrementCommitFailed()
				case 2:
					metrics.IncrementCASConflict()
				case 3:
					metrics.IncrementRecovery()
				}

				// Save to shared file (this is where file locking is tested)
				if err := metrics.SaveMetrics(metricsPath); err != nil {
					results <- fmt.Errorf("Process %d operation %d failed: %v", processID, j, err)
					return
				}

				// Small delay to increase chance of conflicts
				time.Sleep(time.Millisecond * 5)
			}

			// Successful completion
			results <- nil
		}(i)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	var errors []error
	for result := range results {
		if result != nil {
			errors = append(errors, result)
		}
	}

	// Check for errors
	if len(errors) > 0 {
		t.Errorf("Multi-process operations failed:")
		for _, err := range errors {
			t.Errorf("  %v", err)
		}
	}

	// Verify final metrics state
	finalMetrics, err := LoadMetrics(metricsPath)
	if err != nil {
		t.Fatalf("Failed to load final metrics: %v", err)
	}

	// Verify metrics are reasonable (we can't predict exact values due to concurrency)
	totalOperations := int64(processCount * operationsPerProcess)
	expectedTotal := totalOperations / 4 // Each type gets ~25% of operations

	t.Logf("Final metrics after %d processes × %d operations:", processCount, operationsPerProcess)
	t.Logf("  CommitSuccess: %d", finalMetrics.CommitSuccess)
	t.Logf("  CommitFailed: %d", finalMetrics.CommitFailed)
	t.Logf("  CASConflicts: %d", finalMetrics.CASConflicts)
	t.Logf("  RecoveryCount: %d", finalMetrics.RecoveryCount)
	t.Logf("  SchemaVersion: %d", finalMetrics.SchemaVersion)

	// Sanity checks (values should be reasonable but not exact due to concurrency)
	if finalMetrics.CommitSuccess < 1 || finalMetrics.CommitSuccess > expectedTotal*2 {
		t.Errorf("Unexpected CommitSuccess count: %d (expected around %d)", finalMetrics.CommitSuccess, expectedTotal)
	}

	if finalMetrics.SchemaVersion != MetricsSchemaVersion {
		t.Errorf("Schema version mismatch: got %d, expected %d", finalMetrics.SchemaVersion, MetricsSchemaVersion)
	}

	// Verify no corruption (file should be valid JSON)
	if finalMetrics.LastUpdate == "" {
		t.Error("LastUpdate should not be empty")
	}
}

// TestMultiProcessRegisterAndStateTransactions simulates real-world scenario
func TestMultiProcessRegisterAndStateTransactions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-process register/state-tx E2E test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "multiprocess_register_e2e_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up DeeSpec-like directory structure
	deespecDir := filepath.Join(tempDir, ".deespec")
	txnDir := filepath.Join(deespecDir, "var", "txn")
	metricsPath := filepath.Join(deespecDir, "var", "metrics.json")

	err = os.MkdirAll(txnDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	manager := NewManager(txnDir)
	ctx := context.Background()

	// Simulate multiple processes doing register/state-tx operations
	processCount := 2
	transactionsPerProcess := 5

	var wg sync.WaitGroup
	errors := make(chan error, processCount)

	for i := 0; i < processCount; i++ {
		wg.Add(1)
		go func(processID int) {
			defer wg.Done()

			for j := 0; j < transactionsPerProcess; j++ {
				// Create transaction
				tx, err := manager.Begin(ctx)
				if err != nil {
					errors <- fmt.Errorf("Process %d: Begin failed: %v", processID, err)
					return
				}

				// Stage files (simulating register operation)
				specContent := fmt.Sprintf("# Spec from Process %d, Transaction %d\n\nID: process-%d-txn-%d\n\n## Description\n\nMulti-process E2E test specification.", processID, j, processID, j)
				metaContent := fmt.Sprintf("id: process-%d-txn-%d\ntitle: Process %d Transaction %d\nstatus: registered\n", processID, j, processID, j)

				err = manager.StageFile(tx, fmt.Sprintf("specs/process-%d-txn-%d/spec.md", processID, j), []byte(specContent))
				if err != nil {
					errors <- fmt.Errorf("Process %d: StageFile spec failed: %v", processID, err)
					return
				}

				err = manager.StageFile(tx, fmt.Sprintf("specs/process-%d-txn-%d/meta.yaml", processID, j), []byte(metaContent))
				if err != nil {
					errors <- fmt.Errorf("Process %d: StageFile meta failed: %v", processID, err)
					return
				}

				// Mark intent
				err = manager.MarkIntent(tx)
				if err != nil {
					errors <- fmt.Errorf("Process %d: MarkIntent failed: %v", processID, err)
					return
				}

				// Commit with metrics update
				err = manager.Commit(tx, deespecDir, func() error {
					// Update shared metrics (this tests file locking)
					GlobalMetrics.IncrementCommitSuccess()
					return GlobalMetrics.SaveMetrics(metricsPath)
				})

				if err != nil {
					errors <- fmt.Errorf("Process %d: Commit failed: %v", processID, err)
					GlobalMetrics.IncrementCommitFailed()
					if err := GlobalMetrics.SaveMetrics(metricsPath); err != nil {
						t.Errorf("SaveMetrics failed: %v", err)
					}
					return
				}

				// Small delay to allow for race conditions
				time.Sleep(time.Millisecond * 10)
			}
		}(i)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(errors)
	}()

	// Collect errors
	var errorList []error
	for err := range errors {
		if err != nil {
			errorList = append(errorList, err)
		}
	}

	if len(errorList) > 0 {
		t.Errorf("Multi-process register/state-tx operations failed:")
		for _, err := range errorList {
			t.Errorf("  %v", err)
		}
	}

	// Verify final state
	finalMetrics, err := LoadMetrics(metricsPath)
	if err != nil {
		t.Fatalf("Failed to load final metrics: %v", err)
	}

	expectedSuccesses := int64(processCount * transactionsPerProcess)
	t.Logf("Multi-process register/state-tx test completed:")
	t.Logf("  Expected successes: %d", expectedSuccesses)
	t.Logf("  Actual successes: %d", finalMetrics.CommitSuccess)
	t.Logf("  Actual failures: %d", finalMetrics.CommitFailed)

	// Verify metrics consistency
	if finalMetrics.CommitSuccess < expectedSuccesses-1 {
		t.Errorf("Too few successful commits: got %d, expected around %d", finalMetrics.CommitSuccess, expectedSuccesses)
	}

	// Verify files were created correctly
	for i := 0; i < processCount; i++ {
		for j := 0; j < transactionsPerProcess; j++ {
			specPath := filepath.Join(deespecDir, fmt.Sprintf("specs/process-%d-txn-%d/spec.md", i, j))
			metaPath := filepath.Join(deespecDir, fmt.Sprintf("specs/process-%d-txn-%d/meta.yaml", i, j))

			if _, err := os.Stat(specPath); err != nil {
				t.Errorf("Spec file not found: %s", specPath)
			}

			if _, err := os.Stat(metaPath); err != nil {
				t.Errorf("Meta file not found: %s", metaPath)
			}
		}
	}

	t.Logf("Multi-process E2E test completed successfully")
	t.Logf("File locking and metrics synchronization verified")
}

// TestMultiProcessMetricsConsistency tests metrics consistency under high concurrency
func TestMultiProcessMetricsConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping multi-process consistency test in short mode")
	}

	// Disable metrics rotation for stable final count testing
	t.Setenv("DEESPEC_DISABLE_METRICS_ROTATION", "1")

	tempDir, err := os.MkdirTemp("", "multiprocess_consistency_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	metricsPath := filepath.Join(tempDir, "metrics.json")

	// High concurrency test
	processCount := 5
	operationsPerProcess := 100

	// Use channels to synchronize start
	startSignal := make(chan struct{})
	results := make(chan error, processCount)

	var wg sync.WaitGroup

	// Start all processes at roughly the same time
	for i := 0; i < processCount; i++ {
		wg.Add(1)
		go func(processID int) {
			defer wg.Done()

			// Wait for start signal
			<-startSignal

			for j := 0; j < operationsPerProcess; j++ {
				metrics := &MetricsCollector{
					SchemaVersion: MetricsSchemaVersion,
				}

				// Always increment success counter for this test
				metrics.IncrementCommitSuccess()

				// High-frequency save operations
				if err := metrics.SaveMetrics(metricsPath); err != nil {
					results <- fmt.Errorf("Process %d op %d: %v", processID, j, err)
					return
				}
			}

			results <- nil
		}(i)
	}

	// Start all processes simultaneously
	close(startSignal)

	// Wait for completion
	go func() {
		wg.Wait()
		close(results)
	}()

	// Check results
	var errors []error
	for result := range results {
		if result != nil {
			errors = append(errors, result)
		}
	}

	if len(errors) > 0 {
		t.Errorf("High-concurrency consistency test failed:")
		for _, err := range errors {
			t.Errorf("  %v", err)
		}
	}

	// Verify final consistency
	finalMetrics, err := LoadMetrics(metricsPath)
	if err != nil {
		t.Fatalf("Failed to load final metrics: %v", err)
	}

	// Due to monotonic guarantees, the final count should be at least the number of operations
	// but may be higher due to the read-merge-write behavior
	minExpected := int64(processCount * operationsPerProcess)

	t.Logf("High-concurrency test results:")
	t.Logf("  Processes: %d", processCount)
	t.Logf("  Operations per process: %d", operationsPerProcess)
	t.Logf("  Minimum expected: %d", minExpected)
	t.Logf("  Actual final count: %d", finalMetrics.CommitSuccess)

	if finalMetrics.CommitSuccess < minExpected {
		t.Errorf("Final count too low: got %d, expected at least %d", finalMetrics.CommitSuccess, minExpected)
	}

	t.Logf("Multi-process consistency test passed - file locking prevented data corruption")
}
