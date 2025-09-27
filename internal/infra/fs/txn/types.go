package txn

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// TxnID represents a unique transaction identifier.
// Format: "txn_<timestamp>_<random>" for uniqueness and sortability.
type TxnID string

// Status represents the current state of a transaction.
type Status string

const (
	// StatusPending indicates transaction is initialized but not started
	StatusPending Status = "PENDING"

	// StatusIntent indicates all files are staged and ready to commit
	StatusIntent Status = "INTENT"

	// StatusCommit indicates transaction has been successfully committed
	StatusCommit Status = "COMMIT"

	// StatusAborted indicates transaction was explicitly aborted
	StatusAborted Status = "ABORTED"

	// StatusFailed indicates transaction failed during execution
	StatusFailed Status = "FAILED"
)

// IsValid checks if the status value is valid
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusIntent, StatusCommit, StatusAborted, StatusFailed:
		return true
	default:
		return false
	}
}

// UnmarshalJSON validates status values during JSON deserialization
func (s *Status) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return fmt.Errorf("invalid status format: %w", err)
	}

	status := Status(str)
	if !status.IsValid() {
		return fmt.Errorf("invalid status value: %q", str)
	}

	*s = status
	return nil
}

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

	// Detailed checksum information (Step 11)
	ChecksumInfo *FileChecksum `json:"checksum_info,omitempty"`

	// File size in bytes
	Size int64 `json:"size,omitempty"`

	// Permission bits (e.g., 0644)
	Mode uint32 `json:"mode,omitempty"`
}

// Validate checks if the file operation has valid required fields
func (f *FileOperation) Validate() error {
	// Type is required
	if f.Type == "" {
		return fmt.Errorf("file operation type is required")
	}

	// Validate operation type
	switch f.Type {
	case "create", "update", "delete", "rename":
		// valid
	default:
		return fmt.Errorf("invalid file operation type: %q", f.Type)
	}

	// Destination is required for all operations
	if f.Destination == "" {
		return fmt.Errorf("destination path is required")
	}

	// For rename operations, source is required
	if f.Type == "rename" && f.Source == "" {
		return fmt.Errorf("source path is required for rename operation")
	}

	// For create/update operations, checksum will be required (Step 11)
	// but we don't enforce it yet to maintain backward compatibility

	return nil
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

// Validate checks if the manifest has valid required fields
func (m *Manifest) Validate() error {
	// ID is required
	if m.ID == "" {
		return fmt.Errorf("transaction ID is required")
	}

	// At least one file operation is required
	if len(m.Files) == 0 {
		return fmt.Errorf("at least one file operation is required")
	}

	// Validate each file operation
	for i, op := range m.Files {
		if err := op.Validate(); err != nil {
			return fmt.Errorf("file operation[%d]: %w", i, err)
		}
	}

	// CreatedAt should not be zero
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("creation timestamp is required")
	}

	return nil
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

	// Structured restore operations for rollback
	RestoreOps []RestoreOp `json:"restore_ops"`

	// Timestamp when undo was prepared
	PreparedAt time.Time `json:"prepared_at"`

	// Whether undo is still valid
	Valid bool `json:"valid"`
}

// RestoreOp represents a single restore operation for rollback
type RestoreOp struct {
	// Type of restore operation: "overwrite", "delete", "create"
	Type string `json:"type"`

	// Target path to restore
	TargetPath string `json:"target_path"`

	// Path to undo data (for overwrite/create operations)
	UndoPath string `json:"undo_path,omitempty"`

	// Original file permissions (for create operations)
	Permissions os.FileMode `json:"permissions,omitempty"`
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
