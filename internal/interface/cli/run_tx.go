package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs/txn"
)

// SaveStateAndJournalTX saves state.json and appends journal entry atomically using transaction
func SaveStateAndJournalTX(
	state *State,
	journalRec map[string]interface{},
	paths app.Paths,
	prevVersion int,
) error {
	// Validate CAS version first (like original saveStateCAS)
	if state.Version != prevVersion {
		return fmt.Errorf("version changed (expected %d, got %d)", prevVersion, state.Version)
	}

	// Read current state from disk to verify CAS
	if _, err := os.Stat(paths.State); err == nil {
		currentState, err := loadState(paths.State)
		if err != nil {
			return fmt.Errorf("failed to load current state for CAS: %w", err)
		}
		if currentState.Version != prevVersion {
			return fmt.Errorf("version changed on disk (expected %d, got %d)", prevVersion, currentState.Version)
		}
	}

	// Prepare transaction manager
	txnDir := filepath.Join(paths.Var, "txn")
	manager := txn.NewManager(txnDir)
	ctx := context.Background()

	// Begin transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	// Increment version and update timestamp for state
	state.Version++
	state.Meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Marshal state to JSON
	stateData, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Stage state.json
	relStatePath := "var/state.json"
	if err := manager.StageFile(tx, relStatePath, stateData); err != nil {
		return fmt.Errorf("stage state.json: %w", err)
	}

	// Mark intent - all staging complete
	if err := manager.MarkIntent(tx); err != nil {
		return fmt.Errorf("mark intent: %w", err)
	}

	// Commit with journal append
	err = manager.Commit(tx, ".deespec", func() error {
		// Append to journal atomically
		return appendJournalEntryInTX(journalRec, paths.Journal)
	})

	if err != nil {
		// Transaction failed - state version was not actually incremented
		state.Version--
		return fmt.Errorf("commit transaction: %w", err)
	}

	// Clean up transaction directory after successful commit
	if err := manager.Cleanup(tx); err != nil {
		// Non-fatal: just log warning
		fmt.Fprintf(os.Stderr, "WARN: failed to cleanup transaction: %v\n", err)
	}

	return nil
}

// appendJournalEntryInTX appends journal entry with full durability
// This is called within the Commit's withJournal callback
func appendJournalEntryInTX(journalRec map[string]interface{}, journalPath string) error {
	// Ensure journal directory exists
	journalDir := filepath.Dir(journalPath)
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return fmt.Errorf("create journal directory: %w", err)
	}

	// Marshal journal entry
	data, err := json.Marshal(journalRec)
	if err != nil {
		return fmt.Errorf("marshal journal entry: %w", err)
	}

	// Open journal file in append mode with O_APPEND for atomic appends
	file, err := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open journal file: %w", err)
	}
	defer file.Close()

	// Write entry
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write journal entry: %w", err)
	}
	if _, err := file.Write([]byte("\n")); err != nil {
		return fmt.Errorf("write newline: %w", err)
	}

	// DURABILITY: O_APPEND → fsync(file) → fsync(parent dir)
	// This ensures journal durability before transaction is marked as committed
	if err := fs.FsyncFile(file); err != nil {
		// Log warning but continue (as per architecture)
		fmt.Fprintf(os.Stderr, "WARN: journal fsync failed: %v\n", err)
	}
	if err := fs.FsyncDir(journalDir); err != nil {
		fmt.Fprintf(os.Stderr, "WARN: journal dir fsync failed: %v\n", err)
	}

	return nil
}

// UseTXForStateJournal returns true if transaction mode should be used
// for state.json and journal updates. Can be controlled via environment variable.
func UseTXForStateJournal() bool {
	// Enable TX mode by default or via environment
	return os.Getenv("DEESPEC_DISABLE_STATE_TX") != "1"
}
