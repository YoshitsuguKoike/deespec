package txn

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Recovery configuration constants
const (
	DefaultRecoveryTimeout = 30 * time.Second       // Maximum time for single transaction recovery
	DefaultTotalTimeout    = 5 * time.Minute        // Maximum time for complete recovery process
	DefaultMaxRetries      = 3                      // Maximum retry attempts per transaction
	DefaultRetryBaseDelay  = 100 * time.Millisecond // Base delay for exponential backoff
	DefaultRetryMaxDelay   = 2 * time.Second        // Maximum retry delay
)

// Recovery handles transaction recovery operations
type Recovery struct {
	manager      *Manager
	timeout      time.Duration
	totalTimeout time.Duration
	maxRetries   int
	baseDelay    time.Duration
	maxDelay     time.Duration
}

// RecoveryConfig configures recovery behavior
type RecoveryConfig struct {
	Timeout      time.Duration
	TotalTimeout time.Duration
	MaxRetries   int
	BaseDelay    time.Duration
	MaxDelay     time.Duration
}

// NewRecovery creates a new recovery handler with default configuration
func NewRecovery(manager *Manager) *Recovery {
	return NewRecoveryWithConfig(manager, RecoveryConfig{
		Timeout:      DefaultRecoveryTimeout,
		TotalTimeout: DefaultTotalTimeout,
		MaxRetries:   DefaultMaxRetries,
		BaseDelay:    DefaultRetryBaseDelay,
		MaxDelay:     DefaultRetryMaxDelay,
	})
}

// NewRecoveryWithConfig creates a new recovery handler with custom configuration
func NewRecoveryWithConfig(manager *Manager, config RecoveryConfig) *Recovery {
	return &Recovery{
		manager:      manager,
		timeout:      config.Timeout,
		totalTimeout: config.TotalTimeout,
		maxRetries:   config.MaxRetries,
		baseDelay:    config.BaseDelay,
		maxDelay:     config.MaxDelay,
	}
}

// RecoverAll performs recovery for all incomplete transactions with timeout and retry
func (r *Recovery) RecoverAll(ctx context.Context) (*RecoveryResult, error) {
	startTime := time.Now()
	result := &RecoveryResult{
		StartedAt:      startTime,
		RecoveredCount: 0,
		CleanedCount:   0,
		FailedCount:    0,
		Errors:         []error{},
	}

	// Apply total timeout to the context
	totalCtx, cancel := context.WithTimeout(ctx, r.totalTimeout)
	defer cancel()

	// Scan for incomplete transactions
	scanner := NewScanner(r.manager.baseDir)
	scanResult, err := scanner.Scan()
	if err != nil {
		return result, fmt.Errorf("failed to scan transactions: %w", err)
	}

	// Process transactions that need forward recovery (intent without commit)
	for _, txnID := range scanResult.IntentOnly {
		// Check context timeout
		if totalCtx.Err() != nil {
			result.Errors = append(result.Errors, fmt.Errorf("recovery cancelled due to timeout"))
			break
		}

		if err := r.recoverTransactionWithRetry(totalCtx, txnID); err != nil {
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

// recoverTransactionWithRetry performs forward recovery with retry logic
func (r *Recovery) recoverTransactionWithRetry(ctx context.Context, txnID TxnID) error {
	var lastErr error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		// Create timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, r.timeout)

		err := r.recoverTransaction(attemptCtx, txnID)
		cancel()

		if err == nil {
			if attempt > 0 {
				fmt.Fprintf(os.Stderr, "INFO: Transaction recovery succeeded on retry txn.id=%s txn.retry.attempt=%d\n",
					txnID, attempt)
			}
			return nil
		}

		lastErr = err

		// Check if context was cancelled (don't retry on timeout/cancellation)
		if attemptCtx.Err() != nil || ctx.Err() != nil {
			fmt.Fprintf(os.Stderr, "WARN: Transaction recovery timeout/cancelled txn.id=%s txn.retry.attempt=%d\n",
				txnID, attempt)
			break
		}

		// Don't retry on the last attempt
		if attempt < r.maxRetries {
			// Calculate exponential backoff delay
			delay := r.baseDelay * time.Duration(1<<uint(attempt))
			if delay > r.maxDelay {
				delay = r.maxDelay
			}

			fmt.Fprintf(os.Stderr, "WARN: Transaction recovery failed, retrying txn.id=%s txn.retry.attempt=%d txn.retry.delay_ms=%d error=%v\n",
				txnID, attempt, delay.Milliseconds(), err)

			// Wait for backoff delay
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return fmt.Errorf("transaction recovery failed after %d attempts: %w", r.maxRetries+1, lastErr)
}

// recoverTransaction performs forward recovery for a single transaction
func (r *Recovery) recoverTransaction(ctx context.Context, txnID TxnID) error {
	// Check context at start
	if ctx.Err() != nil {
		return ctx.Err()
	}

	txnDir := filepath.Join(r.manager.baseDir, string(txnID))

	// Load manifest
	manifestPath := filepath.Join(txnDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	// Check context after I/O operation
	if ctx.Err() != nil {
		return ctx.Err()
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

	// Determine destination root (prefer explicit env, then DEE_HOME, then local .deespec)
	destRoot := os.Getenv("DEESPEC_TX_DEST_ROOT")
	if destRoot == "" {
		if home := os.Getenv("DEE_HOME"); home != "" {
			destRoot = home
		} else {
			destRoot = ".deespec"
		}
	}

	// Check context before expensive commit operation
	if ctx.Err() != nil {
		return ctx.Err()
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
