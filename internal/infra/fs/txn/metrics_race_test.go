package txn

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestMetricsRaceConditions tests concurrent access to metrics with race detector
// Run with: go test -race -run TestMetricsRaceConditions
func TestMetricsRaceConditions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metrics_race_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	metricsPath := filepath.Join(tempDir, "metrics.json")

	t.Run("ConcurrentIncrements", func(t *testing.T) {
		// Test concurrent increment operations
		metrics := &MetricsCollector{
			SchemaVersion: MetricsSchemaVersion,
		}

		var wg sync.WaitGroup
		concurrency := 10
		increments := 100

		// Start concurrent increment goroutines
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < increments; j++ {
					metrics.IncrementCommitSuccess()
					metrics.IncrementCommitFailed()
					metrics.IncrementCASConflict()
					metrics.IncrementRecovery()
				}
			}()
		}

		wg.Wait()

		// Verify all increments were recorded
		expectedCount := int64(concurrency * increments)
		if metrics.CommitSuccess != expectedCount {
			t.Errorf("Expected %d commit successes, got %d", expectedCount, metrics.CommitSuccess)
		}
		if metrics.CommitFailed != expectedCount {
			t.Errorf("Expected %d commit failures, got %d", expectedCount, metrics.CommitFailed)
		}
		if metrics.CASConflicts != expectedCount {
			t.Errorf("Expected %d CAS conflicts, got %d", expectedCount, metrics.CASConflicts)
		}
		if metrics.RecoveryCount != expectedCount {
			t.Errorf("Expected %d recoveries, got %d", expectedCount, metrics.RecoveryCount)
		}
	})

	t.Run("ConcurrentReadWrite", func(t *testing.T) {
		// Test concurrent read and write operations
		metrics := &MetricsCollector{
			SchemaVersion: MetricsSchemaVersion,
		}

		var wg sync.WaitGroup
		readGoroutines := 5
		writeGoroutines := 5
		operations := 50

		// Start read goroutines
		for i := 0; i < readGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					_ = metrics.GetSnapshot()
					_ = metrics.GetTotalCommits()
					_ = metrics.GetSuccessRate()
				}
			}()
		}

		// Start write goroutines
		for i := 0; i < writeGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					metrics.IncrementCommitSuccess()
					metrics.IncrementCASConflict()
				}
			}()
		}

		wg.Wait()

		// Verify data consistency
		expectedSuccesses := int64(writeGoroutines * operations)
		expectedConflicts := int64(writeGoroutines * operations)

		if metrics.CommitSuccess != expectedSuccesses {
			t.Errorf("Expected %d commit successes, got %d", expectedSuccesses, metrics.CommitSuccess)
		}
		if metrics.CASConflicts != expectedConflicts {
			t.Errorf("Expected %d CAS conflicts, got %d", expectedConflicts, metrics.CASConflicts)
		}
	})

	t.Run("ConcurrentFileAccess", func(t *testing.T) {
		// Test concurrent file save/load operations
		var wg sync.WaitGroup
		goroutines := 8
		operations := 25

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < operations; j++ {
					// Create metrics instance
					metrics := &MetricsCollector{
						SchemaVersion: MetricsSchemaVersion,
					}

					// Perform some increments
					metrics.IncrementCommitSuccess()
					metrics.IncrementCommitFailed()

					// Save to file (with file locking)
					if err := metrics.SaveMetrics(metricsPath); err != nil {
						t.Errorf("Goroutine %d: SaveMetrics failed: %v", id, err)
						return
					}

					// Load from file (with file locking)
					loaded, err := LoadMetrics(metricsPath)
					if err != nil {
						t.Errorf("Goroutine %d: LoadMetrics failed: %v", id, err)
						return
					}

					// Verify schema version
					if loaded.SchemaVersion != MetricsSchemaVersion {
						t.Errorf("Goroutine %d: Expected schema version %d, got %d",
							id, MetricsSchemaVersion, loaded.SchemaVersion)
					}

					// Small delay to increase chance of race conditions
					time.Sleep(time.Millisecond)
				}
			}(i)
		}

		wg.Wait()

		// Final verification - load metrics and check monotonicity
		finalMetrics, err := LoadMetrics(metricsPath)
		if err != nil {
			t.Fatalf("Failed to load final metrics: %v", err)
		}

		// Verify metrics are monotonic (should be >= 0)
		if finalMetrics.CommitSuccess < 0 {
			t.Errorf("Commit success should be non-negative, got %d", finalMetrics.CommitSuccess)
		}
		if finalMetrics.CommitFailed < 0 {
			t.Errorf("Commit failed should be non-negative, got %d", finalMetrics.CommitFailed)
		}
	})

	t.Run("ConcurrentSnapshotOperations", func(t *testing.T) {
		// Test concurrent snapshot creation
		metrics := &MetricsCollector{
			SchemaVersion: MetricsSchemaVersion,
			CommitSuccess: 100,
			CommitFailed:  10,
		}

		var wg sync.WaitGroup
		snapshotGoroutines := 4

		for i := 0; i < snapshotGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				// Create snapshots concurrently
				if err := metrics.CreateSnapshot(metricsPath); err != nil {
					t.Errorf("Goroutine %d: CreateSnapshot failed: %v", id, err)
				}

				// Also test rotation
				if err := metrics.RotateMetrics(metricsPath, false); err != nil {
					t.Errorf("Goroutine %d: RotateMetrics failed: %v", id, err)
				}
			}(i)
		}

		wg.Wait()

		// Verify snapshots were created
		snapshotDir := filepath.Join(filepath.Dir(metricsPath), "snapshots")
		entries, err := os.ReadDir(snapshotDir)
		if err != nil {
			t.Fatalf("Failed to read snapshots directory: %v", err)
		}

		if len(entries) < snapshotGoroutines {
			t.Errorf("Expected at least %d snapshots, found %d", snapshotGoroutines, len(entries))
		}
	})
}

// TestMetricsDeadlockDetection tests for potential deadlocks in metrics operations
func TestMetricsDeadlockDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metrics_deadlock_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	metricsPath := filepath.Join(tempDir, "metrics.json")

	// Test scenario: concurrent save operations with read operations
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	metrics := &MetricsCollector{
		SchemaVersion: MetricsSchemaVersion,
	}

	var wg sync.WaitGroup

	// Start multiple goroutines performing different operations
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Mix of operations that could potentially deadlock
					metrics.IncrementCommitSuccess()
					_ = metrics.GetSnapshot()

					if err := metrics.SaveMetrics(metricsPath); err != nil {
						t.Errorf("Goroutine %d: SaveMetrics failed: %v", id, err)
					}

					_, err := LoadMetrics(metricsPath)
					if err != nil {
						t.Errorf("Goroutine %d: LoadMetrics failed: %v", id, err)
					}
				}
			}
		}(i)
	}

	// Wait for test completion or timeout
	done := make(chan bool)
	go func() {
		wg.Wait()
		done <- true
	}()

	select {
	case <-done:
		t.Log("Deadlock test completed successfully")
	case <-ctx.Done():
		t.Fatal("Test timed out - possible deadlock detected")
	}
}
