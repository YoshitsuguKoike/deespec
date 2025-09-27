package txn

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Scanner provides transaction directory scanning capabilities.
// This is used at startup to detect incomplete transactions.
type Scanner struct {
	// Base directory to scan (e.g., .deespec/var/txn/)
	BaseDir string

	// Logger for warnings (can be customized)
	Logger *log.Logger
}

// NewScanner creates a new transaction scanner.
func NewScanner(baseDir string) *Scanner {
	if baseDir == "" {
		baseDir = ".deespec/var/txn"
	}
	return &Scanner{
		BaseDir: baseDir,
		Logger:  log.New(os.Stderr, "[TXN-SCAN] ", log.LstdFlags),
	}
}

// ScanResult represents the result of a transaction scan.
type ScanResult struct {
	// Total number of transaction directories found
	TotalFound int

	// Transactions with intent but no commit (need forward recovery)
	IntentOnly []TxnID

	// Transactions with commit marker (can be cleaned up)
	Committed []TxnID

	// Transactions with partial staging (incomplete)
	Incomplete []TxnID

	// Transactions with no markers (abandoned)
	Abandoned []TxnID

	// Scan timestamp
	ScannedAt time.Time

	// Any errors encountered during scan
	Errors []error
}

// Scan performs a scan of the transaction directory.
// For Step 5, this only logs warnings and does not perform any recovery.
func (s *Scanner) Scan() (*ScanResult, error) {
	result := &ScanResult{
		IntentOnly: []TxnID{},
		Committed:  []TxnID{},
		Incomplete: []TxnID{},
		Abandoned:  []TxnID{},
		ScannedAt:  time.Now().UTC(),
		Errors:     []error{},
	}

	// Check if base directory exists
	if _, err := os.Stat(s.BaseDir); os.IsNotExist(err) {
		// No txn directory means no incomplete transactions
		s.Logger.Printf("INFO: No transaction directory found txn.base_dir=%s txn.state=clean txn.count=0", s.BaseDir)
		return result, nil
	}

	// Walk through transaction directory
	err := filepath.WalkDir(s.BaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Log but continue scanning
			result.Errors = append(result.Errors, fmt.Errorf("scan error at %s: %w", path, err))
			return nil
		}

		// Skip the base directory itself
		if path == s.BaseDir {
			return nil
		}

		// Only process directories at the first level (transaction directories)
		if d.IsDir() && filepath.Dir(path) == s.BaseDir {
			txnID := TxnID(d.Name())
			result.TotalFound++

			// Check transaction state
			state := s.checkTransactionState(path)

			switch state {
			case "intent_only":
				result.IntentOnly = append(result.IntentOnly, txnID)
				s.Logger.Printf("WARN: Found transaction with intent but no commit txn.id=%s txn.state=intent_only txn.action=forward_recovery_needed", txnID)
			case "committed":
				result.Committed = append(result.Committed, txnID)
				s.Logger.Printf("INFO: Found completed transaction txn.id=%s txn.state=committed txn.action=cleanup_ready", txnID)
			case "incomplete":
				result.Incomplete = append(result.Incomplete, txnID)
				s.Logger.Printf("WARN: Found incomplete transaction txn.id=%s txn.state=incomplete txn.staging=partial", txnID)
			case "abandoned":
				result.Abandoned = append(result.Abandoned, txnID)
				s.Logger.Printf("WARN: Found abandoned transaction txn.id=%s txn.state=abandoned txn.markers=none", txnID)
			}

			// Don't descend into subdirectories
			if d.IsDir() {
				return fs.SkipDir
			}
		}

		return nil
	})

	if err != nil {
		return result, fmt.Errorf("failed to scan transaction directory: %w", err)
	}

	// Log summary
	s.logSummary(result)

	return result, nil
}

// checkTransactionState determines the state of a transaction directory.
func (s *Scanner) checkTransactionState(txnDir string) string {
	intentPath := filepath.Join(txnDir, "status.intent")
	commitPath := filepath.Join(txnDir, "status.commit")
	manifestPath := filepath.Join(txnDir, "manifest.json")
	stagePath := filepath.Join(txnDir, "stage")

	hasIntent := fileExists(intentPath)
	hasCommit := fileExists(commitPath)
	hasManifest := fileExists(manifestPath)
	hasStage := dirExists(stagePath)

	// Determine state based on markers
	if hasCommit {
		return "committed"
	}
	if hasIntent && !hasCommit {
		return "intent_only"
	}
	if hasManifest || hasStage {
		return "incomplete"
	}
	return "abandoned"
}

// logSummary logs a summary of the scan results with machine-readable keys.
func (s *Scanner) logSummary(result *ScanResult) {
	if result.TotalFound == 0 {
		return
	}

	// Main summary with machine-readable keys
	s.Logger.Printf("SUMMARY: Transaction scan complete txn.scan.total=%d txn.scan.timestamp=%s",
		result.TotalFound, result.ScannedAt.Format(time.RFC3339))

	if len(result.IntentOnly) > 0 {
		s.Logger.Printf("  - Forward recovery needed txn.scan.intent_only_count=%d txn.scan.intent_only_ids=%s",
			len(result.IntentOnly), formatTxnIDs(result.IntentOnly))
	}

	if len(result.Incomplete) > 0 {
		s.Logger.Printf("  - Incomplete transactions txn.scan.incomplete_count=%d txn.scan.incomplete_ids=%s",
			len(result.Incomplete), formatTxnIDs(result.Incomplete))
	}

	if len(result.Abandoned) > 0 {
		s.Logger.Printf("  - Abandoned transactions txn.scan.abandoned_count=%d txn.scan.abandoned_ids=%s",
			len(result.Abandoned), formatTxnIDs(result.Abandoned))
	}

	if len(result.Committed) > 0 {
		s.Logger.Printf("  - Ready for cleanup txn.scan.committed_count=%d txn.scan.committed_ids=%s",
			len(result.Committed), formatTxnIDs(result.Committed))
	}

	if len(result.Errors) > 0 {
		s.Logger.Printf("  - Scan errors txn.scan.error_count=%d", len(result.Errors))
	}

	// Performance consideration for large numbers
	if result.TotalFound > 100 {
		s.Logger.Printf("WARN: Large number of transactions found txn.scan.performance=consider_batch_processing txn.count=%d", result.TotalFound)
	}

	// Step 5: Only log, no recovery action
	// Step 8: Cleanup policy will be implemented for committed transactions
	s.Logger.Printf("NOTE: Recovery disabled txn.scan.recovery_action=none txn.cleanup_policy=pending_step8")
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// formatTxnIDs formats transaction IDs for machine-readable logging.
func formatTxnIDs(ids []TxnID) string {
	if len(ids) == 0 {
		return "[]"
	}
	if len(ids) <= 5 {
		// For small lists, show all IDs in array format
		strs := make([]string, len(ids))
		for i, id := range ids {
			strs[i] = string(id)
		}
		return "[" + strings.Join(strs, ",") + "]"
	}
	// For large lists, show first 3 with total count
	strs := make([]string, 3)
	for i := 0; i < 3; i++ {
		strs[i] = string(ids[i])
	}
	return fmt.Sprintf("[%s...] (total:%d)", strings.Join(strs, ","), len(ids))
}
