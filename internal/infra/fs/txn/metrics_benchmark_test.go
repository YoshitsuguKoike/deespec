package txn

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// BenchmarkMetricsCollectorMemory measures memory footprint of MetricsCollector operations
func BenchmarkMetricsCollectorMemory(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "metrics_benchmark_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	metricsPath := filepath.Join(tempDir, "metrics.json")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// 1. Basic counter operations memory usage
		metrics := &MetricsCollector{
			SchemaVersion: MetricsSchemaVersion,
		}

		// Measure counter increment operations
		metrics.IncrementCommitSuccess()
		metrics.IncrementCommitFailed()
		metrics.IncrementCASConflict()
		metrics.IncrementRecovery()

		// 2. File locking overhead during save/load
		err := metrics.SaveMetrics(metricsPath)
		if err != nil {
			b.Fatalf("SaveMetrics failed: %v", err)
		}

		// 3. Snapshot creation memory allocation
		_ = metrics.GetSnapshot() // Use snapshot to ensure it's not optimized away

		// 4. Schema versioning memory impact (loading existing metrics)
		loadedMetrics, err := LoadMetrics(metricsPath)
		if err != nil {
			b.Fatalf("LoadMetrics failed: %v", err)
		}
		_ = loadedMetrics // Use loaded metrics
	}
}

// BenchmarkMetricsCollectorConcurrent measures memory footprint under concurrent access
func BenchmarkMetricsCollectorConcurrent(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "metrics_benchmark_concurrent_*")
	if err != nil {
		b.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	metricsPath := filepath.Join(tempDir, "metrics.json")

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		workerCount := 4

		for j := 0; j < workerCount; j++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Each worker creates its own metrics instance
				metrics := &MetricsCollector{
					SchemaVersion: MetricsSchemaVersion,
				}

				// Perform operations
				metrics.IncrementCommitSuccess()
				err := metrics.SaveMetrics(metricsPath)
				if err != nil {
					// Ignore file locking conflicts in benchmark
					return
				}

				// Load and snapshot operations
				if loadedMetrics, err := LoadMetrics(metricsPath); err == nil {
					_ = loadedMetrics.GetSnapshot()
				}
			}()
		}

		wg.Wait()
	}
}

// BenchmarkMetricsCollectorBaseline measures baseline memory without file operations
func BenchmarkMetricsCollectorBaseline(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Only in-memory operations for baseline measurement
		metrics := &MetricsCollector{
			SchemaVersion: MetricsSchemaVersion,
		}

		// Counter operations only
		metrics.IncrementCommitSuccess()
		metrics.IncrementCommitFailed()
		metrics.IncrementCASConflict()
		metrics.IncrementRecovery()

		// Snapshot creation without file I/O
		_ = metrics.GetSnapshot()
	}
}
