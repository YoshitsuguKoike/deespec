package txn

import (
	"fmt"
	"time"
)

// TxnID represents a unique transaction identifier.
// Format: "txn_<timestamp>_<random>" for uniqueness and sortability.
type TxnID string

// Status represents the current state of a transaction.
type Status string

const (
	// StatusPending indicates transaction is initialized but not started
	StatusPending Status = "pending"

	// StatusIntent indicates all files are staged and ready to commit
	StatusIntent Status = "intent"

	// StatusCommit indicates transaction has been successfully committed
	StatusCommit Status = "commit"

	// StatusAborted indicates transaction was explicitly aborted
	StatusAborted Status = "aborted"

	// StatusFailed indicates transaction failed during execution
	StatusFailed Status = "failed"
)

// FileOperation represents a single file operation within a transaction.
type FileOperation struct {
	// Type of operation: "create", "update", "delete", "rename"
	Type string `json:"type"`

	// Source path (for rename operations)
	Source string `json:"source,omitempty"`

	// Destination path
	Destination string `json:"destination"`

	// SHA256 checksum of the file content (for verification)
	Checksum string `json:"checksum,omitempty"`

	// File size in bytes
	Size int64 `json:"size,omitempty"`

	// Permission bits (e.g., 0644)
	Mode uint32 `json:"mode,omitempty"`
}

// Manifest represents the transaction plan.
// It describes what changes will be made atomically.
type Manifest struct {
	// Unique transaction ID
	ID TxnID `json:"id"`

	// Human-readable description
	Description string `json:"description"`

	// List of file operations to perform
	Files []FileOperation `json:"files"`

	// Creation timestamp
	CreatedAt time.Time `json:"created_at"`

	// Optional deadline for transaction completion
	Deadline *time.Time `json:"deadline,omitempty"`

	// Metadata for tracking (e.g., user, source command)
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// Intent represents the ready-to-commit state marker.
// This file's presence indicates all staging is complete.
type Intent struct {
	// Transaction ID this intent belongs to
	TxnID TxnID `json:"txn_id"`

	// Timestamp when intent was marked
	MarkedAt time.Time `json:"marked_at"`

	// Checksums of all staged files for validation
	Checksums map[string]string `json:"checksums"`

	// Ready flag (always true when intent exists)
	Ready bool `json:"ready"`
}

// Commit represents the successful completion marker.
// This file's presence indicates the transaction was committed.
type Commit struct {
	// Transaction ID this commit belongs to
	TxnID TxnID `json:"txn_id"`

	// Timestamp when commit completed
	CommittedAt time.Time `json:"committed_at"`

	// Journal sequence number (if applicable)
	JournalSeq int64 `json:"journal_seq,omitempty"`

	// Files that were successfully committed
	CommittedFiles []string `json:"committed_files"`

	// Success flag (always true when commit exists)
	Success bool `json:"success"`
}

// Undo represents rollback information (optional).
// Contains before-images for potential rollback.
type Undo struct {
	// Transaction ID this undo belongs to
	TxnID TxnID `json:"txn_id"`

	// Backup locations of original files
	Backups map[string]string `json:"backups"`

	// Timestamp when undo was prepared
	PreparedAt time.Time `json:"prepared_at"`

	// Whether undo is still valid
	Valid bool `json:"valid"`
}

// Transaction represents the complete transaction state.
// This is the main type for managing atomic multi-file operations.
type Transaction struct {
	// Transaction manifest (plan)
	Manifest *Manifest `json:"manifest"`

	// Current status
	Status Status `json:"status"`

	// Intent marker (when ready to commit)
	Intent *Intent `json:"intent,omitempty"`

	// Commit marker (when successfully committed)
	Commit *Commit `json:"commit,omitempty"`

	// Undo information (optional)
	Undo *Undo `json:"undo,omitempty"`

	// Base directory for this transaction
	// Format: .deespec/var/txn/<txn_id>/
	BaseDir string `json:"base_dir"`

	// Stage directory for preparing files
	// Format: .deespec/var/txn/<txn_id>/stage/
	StageDir string `json:"stage_dir"`

	// Undo directory for backups (optional)
	// Format: .deespec/var/txn/<txn_id>/undo/
	UndoDir string `json:"undo_dir,omitempty"`
}

// RecoveryInfo represents information needed for crash recovery.
type RecoveryInfo struct {
	// List of incomplete transactions found
	IncompleteTxns []TxnID `json:"incomplete_txns"`

	// Transactions with intent but no commit (need forward recovery)
	IntentOnly []TxnID `json:"intent_only"`

	// Transactions with partial staging
	PartialStage []TxnID `json:"partial_stage"`

	// Transactions that can be safely cleaned up
	CleanupReady []TxnID `json:"cleanup_ready"`

	// Scan timestamp
	ScannedAt time.Time `json:"scanned_at"`
}

// TxnError represents transaction-specific errors.
type TxnError struct {
	// Transaction ID where error occurred
	TxnID TxnID

	// Operation that failed
	Operation string

	// Underlying error
	Err error

	// Whether the error is recoverable
	Recoverable bool
}

// Error implements the error interface.
func (e *TxnError) Error() string {
	recovery := "unrecoverable"
	if e.Recoverable {
		recovery = "recoverable"
	}
	return fmt.Sprintf("transaction %s: %s failed (%s): %v",
		e.TxnID, e.Operation, recovery, e.Err)
}

// Unwrap returns the underlying error.
func (e *TxnError) Unwrap() error {
	return e.Err
}