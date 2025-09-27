package txn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Recovery handles transaction recovery operations
type Recovery struct {
	manager *Manager
}

// NewRecovery creates a new recovery handler
func NewRecovery(manager *Manager) *Recovery {
	return &Recovery{
		manager: manager,
	}
}

// RecoverAll performs recovery for all incomplete transactions
func (r *Recovery) RecoverAll(ctx context.Context) (*RecoveryResult, error) {
	startTime := time.Now()
	result := &RecoveryResult{
		StartedAt:      startTime,
		RecoveredCount: 0,
		CleanedCount:   0,
		FailedCount:    0,
		Errors:         []error{},
	}

	// Scan for incomplete transactions
	scanner := NewScanner(r.manager.baseDir)
	scanResult, err := scanner.Scan()
	if err != nil {
		return result, fmt.Errorf("failed to scan transactions: %w", err)
	}

	// Process transactions that need forward recovery (intent without commit)
	for _, txnID := range scanResult.IntentOnly {
		if err := r.recoverTransaction(ctx, txnID); err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, fmt.Errorf("failed to recover %s: %w", txnID, err))
			fmt.Fprintf(os.Stderr, "ERROR: Failed to recover transaction %s=%s %s=%v\n",
				MetricRecoverForwardFailed, txnID, "error", err)
		} else {
			result.RecoveredCount++
			fmt.Fprintf(os.Stderr, "INFO: Recovered transaction %s=%s\n",
				MetricRecoverForwardSuccess, txnID)
		}
	}

	// Clean up committed transactions
	for _, txnID := range scanResult.Committed {
		if err := r.cleanupTransaction(string(txnID)); err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("failed to cleanup %s: %w", txnID, err))
			fmt.Fprintf(os.Stderr, "WARN: Failed to cleanup transaction %s=%s %s=%v\n",
				MetricCleanupFailed, txnID, "error", err)
		} else {
			result.CleanedCount++
			fmt.Fprintf(os.Stderr, "INFO: Cleaned up transaction %s=%s\n",
				MetricCleanupSuccess, txnID)
		}
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(startTime)

	// Log summary metrics
	fmt.Fprintf(os.Stderr, "INFO: Recovery complete %s=%d %s=%d %s=%d %s=%d\n",
		MetricRecoverForwardCount, len(scanResult.IntentOnly),
		MetricRecoverForwardSuccess, result.RecoveredCount,
		MetricCleanupSuccess, result.CleanedCount,
		MetricRecoverDurationMs, result.Duration.Milliseconds())

	return result, nil
}

// recoverTransaction performs forward recovery for a single transaction
func (r *Recovery) recoverTransaction(ctx context.Context, txnID TxnID) error {
	txnDir := filepath.Join(r.manager.baseDir, string(txnID))

	// Load manifest
	manifestPath := filepath.Join(txnDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	// Load intent marker
	intentPath := filepath.Join(txnDir, "status.intent")
	intentData, err := os.ReadFile(intentPath)
	if err != nil {
		return fmt.Errorf("failed to read intent marker: %w", err)
	}

	var intent Intent
	if err := json.Unmarshal(intentData, &intent); err != nil {
		return fmt.Errorf("failed to parse intent: %w", err)
	}

	// Reconstruct transaction state
	tx := &Transaction{
		Manifest: &manifest,
		Status:   StatusIntent,
		Intent:   &intent,
		BaseDir:  txnDir,
		StageDir: filepath.Join(txnDir, "stage"),
		UndoDir:  filepath.Join(txnDir, "undo"),
	}

	// Perform forward recovery by completing the commit
	// Since we have intent marker, all files were successfully staged
	// We need to determine the correct destination root

	// For recovery, we need to determine the destination root
	// In production, this would be configurable or derived from context
	destRoot := ".deespec"
	if envRoot := os.Getenv("DEESPEC_TX_DEST_ROOT"); envRoot != "" {
		destRoot = envRoot
	}

	// Note: The journal callback is nil here because the original journal entry
	// should have been written before the crash. If not, we log a warning.
	err = r.manager.Commit(tx, destRoot, func() error {
		// Journal was likely already written before crash
		// If not, we could reconstruct and append here
		fmt.Fprintf(os.Stderr, "WARN: Forward recovery without journal callback for %s\n", txnID)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to complete commit during recovery: %w", err)
	}

	// Note: We do NOT cleanup here. The transaction directory is left intact
	// with the commit marker for verification and potential debugging.
	// Cleanup of committed transactions happens separately via the cleanup scan.

	return nil
}

// cleanupTransaction removes a completed transaction directory
func (r *Recovery) cleanupTransaction(txnID string) error {
	txnDir := filepath.Join(r.manager.baseDir, txnID)

	// Verify it has commit marker before cleanup
	commitPath := filepath.Join(txnDir, "status.commit")
	if _, err := os.Stat(commitPath); err != nil {
		return fmt.Errorf("cannot cleanup: no commit marker found")
	}

	// Remove the entire transaction directory
	if err := os.RemoveAll(txnDir); err != nil {
		return fmt.Errorf("failed to remove transaction directory: %w", err)
	}

	return nil
}

// RecoveryResult contains the results of a recovery operation
type RecoveryResult struct {
	StartedAt      time.Time
	CompletedAt    time.Time
	Duration       time.Duration
	RecoveredCount int
	CleanedCount   int
	FailedCount    int
	Errors         []error
}
