package transaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
	"gopkg.in/yaml.v3"
)

// RegisterTransactionService handles transactional registration of SBI specs
type RegisterTransactionService struct {
	txnBaseDir  string
	journalPath string
	db          *sql.DB
	warnLog     func(format string, args ...interface{})
}

// NewRegisterTransactionService creates a new transaction service
func NewRegisterTransactionService(
	txnBaseDir string,
	journalPath string,
	db *sql.DB,
	warnLog func(format string, args ...interface{}),
) *RegisterTransactionService {
	if txnBaseDir == "" {
		txnBaseDir = ".deespec/var/txn"
	}
	if journalPath == "" {
		journalPath = ".deespec/var/journal.ndjson"
	}
	if warnLog == nil {
		warnLog = func(format string, args ...interface{}) {}
	}

	return &RegisterTransactionService{
		txnBaseDir:  txnBaseDir,
		journalPath: journalPath,
		db:          db,
		warnLog:     warnLog,
	}
}

// ExecuteRegisterTransaction performs atomic registration using TX
func (s *RegisterTransactionService) ExecuteRegisterTransaction(
	ctx context.Context,
	spec *dto.RegisterSpec,
	specPath string,
	journalEntry map[string]interface{},
) error {
	// Validate inputs early
	if spec.ID == "" {
		return fmt.Errorf("spec ID is required")
	}
	if specPath == "" {
		return fmt.Errorf("spec path is required")
	}

	// Initialize TX manager
	manager := txn.NewManager(s.txnBaseDir)

	// Begin transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Stage 1: Prepare spec metadata file (meta.yaml)
	metaData := map[string]interface{}{
		"id":        spec.ID,
		"title":     spec.Title,
		"labels":    spec.Labels,
		"spec_path": specPath,
		"status":    "registered",
	}

	metaYAML, err := yaml.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("failed to marshal meta.yaml: %w", err)
	}

	// Stage the meta.yaml file
	// Note: specPath contains the full path, but we need to strip .deespec/ prefix for staging
	stagePath := specPath
	if strings.HasPrefix(stagePath, ".deespec/") {
		stagePath = strings.TrimPrefix(stagePath, ".deespec/")
	}
	relMetaPath := filepath.Join(stagePath, "meta.yaml")
	if err := manager.StageFile(tx, relMetaPath, metaYAML); err != nil {
		return fmt.Errorf("failed to stage meta.yaml: %w", err)
	}

	// Stage 2: Prepare empty spec content file
	specContentPath := filepath.Join(stagePath, "spec.md")
	specContent := fmt.Sprintf("# %s\n\nID: %s\n\n## Description\n\n<!-- Add spec details here -->\n",
		spec.Title, spec.ID)

	if err := manager.StageFile(tx, specContentPath, []byte(specContent)); err != nil {
		return fmt.Errorf("failed to stage spec.md: %w", err)
	}

	// Mark intent - all files are staged
	if err := manager.MarkIntent(tx); err != nil {
		return fmt.Errorf("failed to mark intent: %w", err)
	}

	// Save to SQLite before committing files
	if s.db != nil {
		if err := s.saveSBIToSQLite(ctx, spec, specPath); err != nil {
			return fmt.Errorf("failed to save SBI to SQLite: %w", err)
		}
	}

	// Commit phase with journal integration
	err = manager.Commit(tx, ".deespec", func() error {
		// Append to journal as part of the transaction commit
		return s.appendJournalEntryTX(journalEntry)
	})
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Cleanup transaction directory after successful commit
	if err := manager.Cleanup(tx); err != nil {
		// Non-fatal: just log warning
		s.warnLog("failed to cleanup transaction: %v", err)
	}

	return nil
}

// appendJournalEntryTX appends a journal entry with full durability guarantees.
// DURABILITY: Uses O_APPEND + fsync(file) + fsync(parent dir) to ensure atomic,
// durable writes. This is called within Commit's withJournal callback, ensuring
// journal durability before the transaction is marked as committed.
func (s *RegisterTransactionService) appendJournalEntryTX(journalEntry map[string]interface{}) error {
	journalDir := filepath.Dir(s.journalPath)

	// Ensure journal directory exists
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return fmt.Errorf("failed to create journal directory: %w", err)
	}

	// Marshal journal entry
	data, err := json.Marshal(journalEntry)
	if err != nil {
		return fmt.Errorf("failed to marshal journal entry: %w", err)
	}

	// Open journal file in append mode with O_APPEND for atomic appends
	file, err := os.OpenFile(s.journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open journal file: %w", err)
	}
	defer file.Close()

	// Write entry
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write journal entry: %w", err)
	}
	if _, err := file.Write([]byte("\n")); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Fsync file and parent directory
	if err := fs.FsyncFile(file); err != nil {
		// Log warning but continue (as per architecture)
		s.warnLog("journal fsync failed: %v", err)
	}
	if err := fs.FsyncDir(journalDir); err != nil {
		s.warnLog("journal dir fsync failed: %v", err)
	}

	return nil
}

// saveSBIToSQLite saves SBI metadata to SQLite database
func (s *RegisterTransactionService) saveSBIToSQLite(ctx context.Context, spec *dto.RegisterSpec, specPath string) error {
	// Get next sequence number
	var sequence int
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(sequence), 0) + 1 FROM sbis").Scan(&sequence)
	if err != nil {
		return fmt.Errorf("failed to get next sequence: %w", err)
	}

	// Generate ULID for SBI ID
	// Note: spec.ID from user input is different from database ID (ULID)
	sbiID := spec.ID // Use user-provided ID for now

	// Prepare INSERT query with sequence and registered_at
	query := `
		INSERT INTO sbis (id, title, description, status, current_step, parent_pbi_id,
		                  estimated_hours, priority, sequence, registered_at, labels, assigned_agent, file_paths,
		                  current_turn, current_attempt, max_turns, max_attempts, last_error, artifact_paths,
		                  created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	labelsJSON := "[]"
	if len(spec.Labels) > 0 {
		data, err := json.Marshal(spec.Labels)
		if err != nil {
			return fmt.Errorf("failed to marshal labels: %w", err)
		}
		labelsJSON = string(data)
	}

	_, err = s.db.ExecContext(ctx, query,
		sbiID,        // id
		spec.Title,   // title
		"",           // description
		"pending",    // status
		"registered", // current_step
		nil,          // parent_pbi_id
		0.0,          // estimated_hours
		0,            // priority (default)
		sequence,     // sequence
		now,          // registered_at
		labelsJSON,   // labels
		"",           // assigned_agent
		"[]",         // file_paths
		1,            // current_turn
		1,            // current_attempt
		10,           // max_turns
		3,            // max_attempts
		"",           // last_error
		"[]",         // artifact_paths
		now,          // created_at
		now,          // updated_at
	)
	if err != nil {
		return fmt.Errorf("failed to insert SBI to SQLite: %w", err)
	}

	return nil
}
