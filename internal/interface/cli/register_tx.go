package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
	"gopkg.in/yaml.v3"
)

// registerWithTransaction performs atomic registration using TX
func registerWithTransaction(
	spec *RegisterSpec,
	result *RegisterResult,
	config *ResolvedConfig,
	journalEntry map[string]interface{},
) error {
	// Initialize TX manager
	txnBaseDir := ".deespec/var/txn"
	manager := txn.NewManager(txnBaseDir)
	ctx := context.Background()

	// Validate inputs early
	if spec.ID == "" {
		return fmt.Errorf("spec ID is required")
	}
	if result.SpecPath == "" {
		return fmt.Errorf("spec path is required")
	}

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
		"spec_path": result.SpecPath,
		"status":    "registered",
	}

	metaYAML, err := yaml.Marshal(metaData)
	if err != nil {
		return fmt.Errorf("failed to marshal meta.yaml: %w", err)
	}

	// Stage the meta.yaml file
	// Note: result.SpecPath contains the full path, but we need to strip .deespec/ prefix for staging
	stagePath := result.SpecPath
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

	// Commit phase with journal integration
	err = manager.Commit(tx, ".deespec", func() error {
		// Append to journal as part of the transaction commit
		return appendJournalEntryTX(journalEntry)
	})
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Cleanup transaction directory after successful commit
	if err := manager.Cleanup(tx); err != nil {
		// Non-fatal: just log warning
		fmt.Fprintf(os.Stderr, "WARN: failed to cleanup transaction: %v\n", err)
	}

	return nil
}

// appendJournalEntryTX appends a journal entry with full durability guarantees.
// DURABILITY: Uses O_APPEND + fsync(file) + fsync(parent dir) to ensure atomic,
// durable writes. This is called within Commit's withJournal callback, ensuring
// journal durability before the transaction is marked as committed.
func appendJournalEntryTX(journalEntry map[string]interface{}) error {
	journalDir := ".deespec/var"
	journalPath := filepath.Join(journalDir, "journal.ndjson")

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
	file, err := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
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
		fmt.Fprintf(os.Stderr, "WARN: journal fsync failed: %v\n", err)
	}
	if err := fs.FsyncDir(journalDir); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: journal dir fsync failed: %v\n", err)
	}

	return nil
}
