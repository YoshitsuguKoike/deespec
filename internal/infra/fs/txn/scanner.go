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
		s.Logger.Printf("INFO: No transaction directory found at %s (clean state)", s.BaseDir)
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
				s.Logger.Printf("WARN: Found transaction %s with intent but no commit (needs forward recovery)", txnID)
			case "committed":
				result.Committed = append(result.Committed, txnID)
				s.Logger.Printf("INFO: Found completed transaction %s (can be cleaned up)", txnID)
			case "incomplete":
				result.Incomplete = append(result.Incomplete, txnID)
				s.Logger.Printf("WARN: Found incomplete transaction %s (partial staging)", txnID)
			case "abandoned":
				result.Abandoned = append(result.Abandoned, txnID)
				s.Logger.Printf("WARN: Found abandoned transaction %s (no markers)", txnID)
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

// logSummary logs a summary of the scan results.
func (s *Scanner) logSummary(result *ScanResult) {
	if result.TotalFound == 0 {
		return
	}

	s.Logger.Printf("SUMMARY: Scanned %d transaction(s):", result.TotalFound)

	if len(result.IntentOnly) > 0 {
		s.Logger.Printf("  - %d need forward recovery (intent without commit)", len(result.IntentOnly))
		s.Logger.Printf("    IDs: %s", formatTxnIDs(result.IntentOnly))
	}

	if len(result.Incomplete) > 0 {
		s.Logger.Printf("  - %d incomplete (partial staging)", len(result.Incomplete))
		s.Logger.Printf("    IDs: %s", formatTxnIDs(result.Incomplete))
	}

	if len(result.Abandoned) > 0 {
		s.Logger.Printf("  - %d abandoned (no markers)", len(result.Abandoned))
		s.Logger.Printf("    IDs: %s", formatTxnIDs(result.Abandoned))
	}

	if len(result.Committed) > 0 {
		s.Logger.Printf("  - %d committed (ready for cleanup)", len(result.Committed))
		s.Logger.Printf("    IDs: %s", formatTxnIDs(result.Committed))
	}

	if len(result.Errors) > 0 {
		s.Logger.Printf("  - %d errors encountered during scan", len(result.Errors))
	}

	// Step 5: Only log, no recovery action
	s.Logger.Printf("NOTE: Step 5 - No recovery actions taken (logging only)")
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

// formatTxnIDs formats transaction IDs for logging.
func formatTxnIDs(ids []TxnID) string {
	if len(ids) == 0 {
		return "none"
	}
	if len(ids) <= 3 {
		strs := make([]string, len(ids))
		for i, id := range ids {
			strs[i] = string(id)
		}
		return strings.Join(strs, ", ")
	}
	// Show first 3 and count
	strs := make([]string, 3)
	for i := 0; i < 3; i++ {
		strs[i] = string(ids[i])
	}
	return fmt.Sprintf("%s... (%d total)", strings.Join(strs, ", "), len(ids))
}