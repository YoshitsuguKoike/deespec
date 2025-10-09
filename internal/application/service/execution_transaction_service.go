package service

import (
	"bytes"
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

// State represents the execution state
// TODO: This should eventually be moved to domain layer
type State struct {
	Version        int               `json:"version"`
	Current        string            `json:"current"` // Legacy: plan/implement/test/review/done
	Status         string            `json:"status"`  // New: READY/WIP/REVIEW/REVIEW&WIP/DONE
	Turn           int               `json:"turn"`
	WIP            string            `json:"wip"`              // Work In Progress - current SBI ID (empty = no WIP)
	LeaseExpiresAt string            `json:"lease_expires_at"` // UTC RFC3339Nano (empty when no WIP)
	Inputs         map[string]string `json:"inputs"`
	LastArtifacts  map[string]string `json:"last_artifacts"`
	Decision       string            `json:"decision"` // PENDING/NEEDS_CHANGES/SUCCEEDED/FAILED
	Attempt        int               `json:"attempt"`  // Implementation attempt number (1-3)
	Meta           StateMeta         `json:"meta"`
}

// StateMeta holds state metadata
type StateMeta struct {
	UpdatedAt string `json:"updated_at"`
}

// ExecutionTransactionService handles atomic state and journal persistence
type ExecutionTransactionService struct {
	warnLog      func(format string, args ...interface{})
	globalConfig interface{ Home() string } // Config interface for destRoot resolution
}

// NewExecutionTransactionService creates a new execution transaction service
func NewExecutionTransactionService(
	warnLog func(format string, args ...interface{}),
	config interface{ Home() string },
) *ExecutionTransactionService {
	if warnLog == nil {
		warnLog = func(format string, args ...interface{}) {}
	}
	return &ExecutionTransactionService{
		warnLog:      warnLog,
		globalConfig: config,
	}
}

// SaveStateAndJournalTX saves state.json and appends journal entry atomically using transaction
func (s *ExecutionTransactionService) SaveStateAndJournalTX(
	state *State,
	journalRec map[string]interface{},
	paths app.Paths,
	prevVersion int,
) error {
	// Load metrics for tracking first
	metricsPath := filepath.Join(paths.Var, "metrics.json")
	metrics, err := txn.LoadMetrics(metricsPath)
	if err != nil {
		s.warnLog("Failed to load metrics: %v\n", err)
		metrics = &txn.MetricsCollector{} // Use fresh metrics on error
	}

	// Validate CAS version first
	if err := s.validateCASVersion(state, prevVersion, paths.State, metrics, metricsPath); err != nil {
		return err
	}

	// Prepare transaction manager
	txnDir := filepath.Join(paths.Var, "txn")
	manager := txn.NewManager(txnDir)
	ctx := context.Background()

	// Begin transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		return fmt.Errorf("state-tx begin: %w", err)
	}

	// Increment version and update timestamp for state
	state.Version++
	state.Meta.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	// Marshal state to stable JSON (fixed key order + trailing LF)
	stateData, err := s.MarshalStableJSON(state)
	if err != nil {
		return fmt.Errorf("state-tx marshal: %w", err)
	}

	// Stage state.json
	relStatePath := "var/state.json"
	if err := manager.StageFile(tx, relStatePath, stateData); err != nil {
		return fmt.Errorf("state-tx stage: %w", err)
	}

	// Mark intent - all staging complete
	if err := manager.MarkIntent(tx); err != nil {
		return fmt.Errorf("state-tx intent: %w", err)
	}

	// Determine destination root for finalization
	destRoot := s.resolveDestRoot(paths)

	// Commit with journal append
	err = manager.Commit(tx, destRoot, func() error {
		// Append to journal atomically
		return s.appendJournalEntryInTX(journalRec, paths.Journal)
	})

	if err != nil {
		// Transaction failed - state version was not actually incremented
		state.Version--
		metrics.IncrementCommitFailed()
		metrics.SaveMetrics(metricsPath) // Best effort save
		return fmt.Errorf("state-tx commit: %w", err)
	}

	// Transaction committed successfully
	metrics.IncrementCommitSuccess()
	metrics.SaveMetrics(metricsPath) // Best effort save

	// Clean up transaction directory after successful commit
	if err := manager.Cleanup(tx); err != nil {
		// Non-fatal: just log warning
		s.warnLog("failed to cleanup transaction: %v\n", err)
	}

	return nil
}

// validateCASVersion validates the CAS version and updates metrics on conflict
func (s *ExecutionTransactionService) validateCASVersion(
	state *State,
	prevVersion int,
	statePath string,
	metrics *txn.MetricsCollector,
	metricsPath string,
) error {
	// Validate CAS version first (like original saveStateCAS)
	if state.Version != prevVersion {
		metrics.IncrementCASConflict()
		metrics.SaveMetrics(metricsPath) // Best effort save
		return fmt.Errorf("version changed (expected %d, got %d)", prevVersion, state.Version)
	}

	// Read current state from disk to verify CAS
	if _, err := os.Stat(statePath); err == nil {
		currentState, err := s.loadState(statePath)
		if err != nil {
			return fmt.Errorf("failed to load current state for CAS: %w", err)
		}
		if currentState.Version != prevVersion {
			metrics.IncrementCASConflict()
			metrics.SaveMetrics(metricsPath) // Best effort save
			return fmt.Errorf("version changed on disk (expected %d, got %d)", prevVersion, currentState.Version)
		}
	}

	return nil
}

// loadState loads state from file
func (s *ExecutionTransactionService) loadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// resolveDestRoot determines the destination root for transaction finalization
func (s *ExecutionTransactionService) resolveDestRoot(paths app.Paths) string {
	var destRoot string
	if s.globalConfig != nil {
		destRoot = s.globalConfig.Home()
	}
	if destRoot == "" {
		// Fallback logic if config is not available
		if paths.Home != "" {
			destRoot = filepath.Join(paths.Home, ".deespec")
		} else if paths.Var != "" {
			// If Var is <root>/.deespec/var, use its parent (<root>/.deespec)
			destRoot = filepath.Dir(paths.Var)
		} else {
			destRoot = ".deespec"
		}
	}
	return destRoot
}

// appendJournalEntryInTX appends journal entry with full durability
// This is called within the Commit's withJournal callback
func (s *ExecutionTransactionService) appendJournalEntryInTX(
	journalRec map[string]interface{},
	journalPath string,
) error {
	// Ensure journal directory exists
	journalDir := filepath.Dir(journalPath)
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return fmt.Errorf("state-tx journal mkdir: %w", err)
	}

	// Marshal journal entry
	data, err := json.Marshal(journalRec)
	if err != nil {
		return fmt.Errorf("state-tx journal marshal: %w", err)
	}

	// Open journal file in append mode with O_APPEND for atomic appends
	file, err := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("state-tx journal open: %w", err)
	}
	defer file.Close()

	// Write entry
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("state-tx journal write: %w", err)
	}
	if _, err := file.Write([]byte("\n")); err != nil {
		return fmt.Errorf("state-tx journal newline: %w", err)
	}

	// DURABILITY: O_APPEND → fsync(file) → fsync(parent dir)
	// This ensures journal durability before transaction is marked as committed
	if err := fs.FsyncFile(file); err != nil {
		// Log warning but continue (as per architecture)
		s.warnLog("journal fsync failed: %v\n", err)
	}
	if err := fs.FsyncDir(journalDir); err != nil {
		s.warnLog("journal dir fsync failed: %v\n", err)
	}

	return nil
}

// MarshalStableJSON marshals data to JSON with stable key ordering and trailing LF
// This ensures consistent output for CAS comparison and diff reviews
func (s *ExecutionTransactionService) MarshalStableJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false) // Preserve special characters for stability

	if err := encoder.Encode(v); err != nil {
		return nil, err
	}

	// Encoder.Encode already adds trailing newline, ensuring stable format
	return buf.Bytes(), nil
}
