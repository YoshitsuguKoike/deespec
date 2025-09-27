package txn

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Schema version for metrics.json compatibility
const (
	MetricsSchemaVersion = 1
)

// MetricsCollector collects and manages transaction metrics for doctor --json
type MetricsCollector struct {
	mu            sync.RWMutex
	CommitSuccess int64  `json:"commit_success"`
	CommitFailed  int64  `json:"commit_failed"`
	CASConflicts  int64  `json:"cas_conflicts"`
	RecoveryCount int64  `json:"recovery_count"`
	LastUpdate    string `json:"last_update"`
	SchemaVersion int    `json:"schema_version"`
}

// GlobalMetrics is the global metrics instance
var GlobalMetrics = &MetricsCollector{}

// Clone creates a copy of the metrics collector
func (m *MetricsCollector) Clone() *MetricsCollector {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &MetricsCollector{
		CommitSuccess: m.CommitSuccess,
		CommitFailed:  m.CommitFailed,
		CASConflicts:  m.CASConflicts,
		RecoveryCount: m.RecoveryCount,
		SchemaVersion: m.SchemaVersion,
		LastUpdate:    m.LastUpdate,
	}
}

// Merge combines two metrics collectors with monotonic increase
func (m *MetricsCollector) Merge(other *MetricsCollector) *MetricsCollector {
	if other == nil {
		return m
	}
	return &MetricsCollector{
		CommitSuccess: m.CommitSuccess + other.CommitSuccess,
		CommitFailed:  m.CommitFailed + other.CommitFailed,
		CASConflicts:  m.CASConflicts + other.CASConflicts,
		RecoveryCount: m.RecoveryCount + other.RecoveryCount,
		SchemaVersion: m.SchemaVersion,
		LastUpdate:    time.Now().UTC().Format(time.RFC3339),
	}
}

// acquireFileLock locks a file descriptor for exclusive access
func acquireFileLock(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_EX)
}

// releaseFileLock releases a file lock
func releaseFileLock(fd int) error {
	return syscall.Flock(fd, syscall.LOCK_UN)
}

// readMetricsJSONLocked reads metrics from a locked file
func readMetricsJSONLocked(file *os.File) (*MetricsCollector, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Size() == 0 {
		return &MetricsCollector{}, nil
	}

	data := make([]byte, stat.Size())
	if _, err := file.ReadAt(data, 0); err != nil {
		return nil, err
	}

	var metrics MetricsCollector
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, err
	}
	return &metrics, nil
}

// writeJSONAtomicLocked writes JSON to a locked file atomically
func writeJSONAtomicLocked(file *os.File, metrics *MetricsCollector) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}

	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.Seek(0, 0); err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

// LoadMetrics loads metrics from disk or creates new instance with file locking
func LoadMetrics(metricsPath string) (*MetricsCollector, error) {
	// Try to open file for reading with lock
	file, err := os.OpenFile(metricsPath, os.O_RDONLY, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			// Return fresh metrics if file doesn't exist
			return &MetricsCollector{
				SchemaVersion: MetricsSchemaVersion,
				LastUpdate:    time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
		return nil, fmt.Errorf("open metrics file: %w", err)
	}
	defer file.Close()

	// Acquire shared lock for reading
	if err := lockFile(int(file.Fd()), syscall.LOCK_SH); err != nil {
		return nil, fmt.Errorf("acquire read lock: %w", err)
	}
	defer func() {
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_UN); err != nil {
			log.Printf("WARN: metrics unlock failed: %v", err)
		}
	}()

	// Read file content
	data, err := os.ReadFile(metricsPath)
	if err != nil {
		return nil, fmt.Errorf("read metrics file: %w", err)
	}

	var metrics MetricsCollector
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("unmarshal metrics: %w", err)
	}

	// Handle schema version compatibility
	if metrics.SchemaVersion == 0 {
		// Legacy format - set current schema version
		metrics.SchemaVersion = MetricsSchemaVersion
	} else if metrics.SchemaVersion > MetricsSchemaVersion {
		return nil, fmt.Errorf("unsupported schema version %d (max supported: %d)",
			metrics.SchemaVersion, MetricsSchemaVersion)
	}

	return &metrics, nil
}

// SaveMetrics saves metrics to disk with file locking and monotonic guarantees
func (m *MetricsCollector) SaveMetrics(metricsPath string) error {
	// Take snapshot of counters without holding lock during file I/O
	snap := m.Clone()
	snap.SchemaVersion = MetricsSchemaVersion
	snap.LastUpdate = time.Now().UTC().Format(time.RFC3339)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(metricsPath), 0755); err != nil {
		return fmt.Errorf("create metrics dir: %w", err)
	}

	// Open file and acquire exclusive lock
	file, err := os.OpenFile(metricsPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("open metrics file: %w", err)
	}
	defer file.Close()

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("metrics flock EX: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	// Read existing metrics and merge
	disk, _ := readMetricsJSONLocked(file)
	merged := snap.Merge(disk) // Monotonic increase
	merged.SchemaVersion = MetricsSchemaVersion
	merged.LastUpdate = time.Now().UTC().Format(time.RFC3339)

	// Write merged metrics atomically
	if err := writeJSONAtomicLocked(file, merged); err != nil {
		return fmt.Errorf("write metrics: %w", err)
	}

	return nil
}

// IncrementCommitSuccess increments the commit success counter
func (m *MetricsCollector) IncrementCommitSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CommitSuccess++
	if !isTestEnvironment() {
		fmt.Fprintf(os.Stderr, "INFO: Transaction committed successfully txn.state.commit.success=true txn.commit.total=%d\n", m.CommitSuccess)
	}
}

// IncrementCommitFailed increments the commit failure counter
func (m *MetricsCollector) IncrementCommitFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CommitFailed++
	if !isTestEnvironment() {
		fmt.Fprintf(os.Stderr, "WARN: Transaction commit failed txn.state.commit.failed=true txn.failed.total=%d\n", m.CommitFailed)
	}
}

// IncrementCASConflict increments the CAS conflict counter
func (m *MetricsCollector) IncrementCASConflict() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CASConflicts++
	if !isTestEnvironment() {
		fmt.Fprintf(os.Stderr, "WARN: CAS conflict detected txn.cas.conflict.count=%d\n", m.CASConflicts)
	}
}

// IncrementRecovery increments the recovery operation counter
func (m *MetricsCollector) IncrementRecovery() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RecoveryCount++
	if !isTestEnvironment() {
		fmt.Fprintf(os.Stderr, "INFO: Recovery operation completed txn.recovery.count=%d\n", m.RecoveryCount)
	}
}

// GetSnapshot returns a read-only snapshot of current metrics
func (m *MetricsCollector) GetSnapshot() MetricsCollector {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return MetricsCollector{
		CommitSuccess: m.CommitSuccess,
		CommitFailed:  m.CommitFailed,
		CASConflicts:  m.CASConflicts,
		RecoveryCount: m.RecoveryCount,
		LastUpdate:    m.LastUpdate,
	}
}

// GetTotalCommits returns total commit attempts (success + failed)
func (m *MetricsCollector) GetTotalCommits() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.CommitSuccess + m.CommitFailed
}

// GetSuccessRate returns commit success rate as percentage
func (m *MetricsCollector) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := m.CommitSuccess + m.CommitFailed
	if total == 0 {
		return 0.0
	}
	return float64(m.CommitSuccess) / float64(total) * 100.0
}

// CreateSnapshot creates a timestamped snapshot of current metrics
func (m *MetricsCollector) CreateSnapshot(metricsPath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Generate snapshot filename with precise timestamp and nanoseconds
	now := time.Now().UTC()
	timestamp := now.Format("20060102_150405")
	nanos := now.Nanosecond()
	snapshotDir := filepath.Join(filepath.Dir(metricsPath), "snapshots")
	snapshotPath := filepath.Join(snapshotDir, fmt.Sprintf("metrics_%s_%09d.json", timestamp, nanos))

	// Ensure snapshot directory exists
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("create snapshot dir: %w", err)
	}

	// Create snapshot with current data
	snapshot := &MetricsCollector{
		CommitSuccess: m.CommitSuccess,
		CommitFailed:  m.CommitFailed,
		CASConflicts:  m.CASConflicts,
		RecoveryCount: m.RecoveryCount,
		SchemaVersion: m.SchemaVersion,
		LastUpdate:    time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}

	fmt.Fprintf(os.Stderr, "INFO: Metrics snapshot created path=%s\n", snapshotPath)
	return nil
}

// RotateMetrics creates a snapshot and optionally resets counters
func (m *MetricsCollector) RotateMetrics(metricsPath string, resetCounters bool) error {
	// Create snapshot first
	if err := m.CreateSnapshot(metricsPath); err != nil {
		return fmt.Errorf("create snapshot for rotation: %w", err)
	}

	if resetCounters {
		m.mu.Lock()
		defer m.mu.Unlock()

		// Reset all counters while preserving schema version
		m.CommitSuccess = 0
		m.CommitFailed = 0
		m.CASConflicts = 0
		m.RecoveryCount = 0
		m.LastUpdate = time.Now().UTC().Format(time.RFC3339)

		// Save reset metrics
		if err := m.SaveMetrics(metricsPath); err != nil {
			return fmt.Errorf("save reset metrics: %w", err)
		}

		fmt.Fprintf(os.Stderr, "INFO: Metrics rotated and reset counters=true\n")
	} else {
		fmt.Fprintf(os.Stderr, "INFO: Metrics rotated and reset counters=false\n")
	}

	return nil
}

// CleanupOldSnapshots removes snapshots older than specified days
func CleanupOldSnapshots(metricsPath string, retentionDays int) error {
	snapshotDir := filepath.Join(filepath.Dir(metricsPath), "snapshots")

	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No snapshots directory
		}
		return fmt.Errorf("read snapshots dir: %w", err)
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)
	cleaned := 0

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			snapshotPath := filepath.Join(snapshotDir, entry.Name())
			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoffTime) {
				if err := os.Remove(snapshotPath); err != nil {
					fmt.Fprintf(os.Stderr, "WARN: Failed to remove old snapshot %s: %v\n", snapshotPath, err)
				} else {
					cleaned++
				}
			}
		}
	}

	if cleaned > 0 {
		fmt.Fprintf(os.Stderr, "INFO: Cleaned up %d old metric snapshots older than %d days\n", cleaned, retentionDays)
	}

	return nil
}

// lockFile applies file locking with proper error handling
func lockFile(fd int, how int) error {
	if err := syscall.Flock(fd, how); err != nil {
		return fmt.Errorf("flock(%d, %d): %w", fd, how, err)
	}
	return nil
}

// isTestEnvironment checks if we should suppress verbose logging
func isTestEnvironment() bool {
	// Check if we're running under go test
	if strings.Contains(os.Args[0], ".test") || strings.HasSuffix(os.Args[0], ".test.exe") {
		return true
	}
	// Check for TEST environment variable
	if os.Getenv("DEESPEC_TEST_QUIET") != "" {
		return true
	}
	return false
}
