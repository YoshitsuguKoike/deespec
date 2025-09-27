package txn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MetricsCollector collects and manages transaction metrics for doctor --json
type MetricsCollector struct {
	mu            sync.RWMutex
	CommitSuccess int64  `json:"commit_success"`
	CommitFailed  int64  `json:"commit_failed"`
	CASConflicts  int64  `json:"cas_conflicts"`
	RecoveryCount int64  `json:"recovery_count"`
	LastUpdate    string `json:"last_update"`
}

// GlobalMetrics is the global metrics instance
var GlobalMetrics = &MetricsCollector{}

// LoadMetrics loads metrics from disk or creates new instance
func LoadMetrics(metricsPath string) (*MetricsCollector, error) {
	data, err := os.ReadFile(metricsPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return fresh metrics if file doesn't exist
			return &MetricsCollector{
				LastUpdate: time.Now().UTC().Format(time.RFC3339),
			}, nil
		}
		return nil, fmt.Errorf("load metrics: %w", err)
	}

	var metrics MetricsCollector
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("unmarshal metrics: %w", err)
	}

	return &metrics, nil
}

// SaveMetrics saves metrics to disk
func (m *MetricsCollector) SaveMetrics(metricsPath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LastUpdate = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metrics: %w", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(metricsPath), 0755); err != nil {
		return fmt.Errorf("create metrics dir: %w", err)
	}

	// Write atomically using temp file
	tempPath := metricsPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("write temp metrics: %w", err)
	}

	if err := os.Rename(tempPath, metricsPath); err != nil {
		os.Remove(tempPath) // cleanup on failure
		return fmt.Errorf("rename metrics: %w", err)
	}

	return nil
}

// IncrementCommitSuccess increments the commit success counter
func (m *MetricsCollector) IncrementCommitSuccess() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CommitSuccess++
	fmt.Fprintf(os.Stderr, "INFO: Transaction committed successfully txn.state.commit.success=true txn.commit.total=%d\n", m.CommitSuccess)
}

// IncrementCommitFailed increments the commit failure counter
func (m *MetricsCollector) IncrementCommitFailed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CommitFailed++
	fmt.Fprintf(os.Stderr, "WARN: Transaction commit failed txn.state.commit.failed=true txn.failed.total=%d\n", m.CommitFailed)
}

// IncrementCASConflict increments the CAS conflict counter
func (m *MetricsCollector) IncrementCASConflict() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CASConflicts++
	fmt.Fprintf(os.Stderr, "WARN: CAS conflict detected txn.cas.conflict.count=%d\n", m.CASConflicts)
}

// IncrementRecovery increments the recovery operation counter
func (m *MetricsCollector) IncrementRecovery() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RecoveryCount++
	fmt.Fprintf(os.Stderr, "INFO: Recovery operation completed txn.recovery.count=%d\n", m.RecoveryCount)
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
