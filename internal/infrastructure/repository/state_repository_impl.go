package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
)

// StateRepositoryImpl implements repository.StateRepository using file-based storage
type StateRepositoryImpl struct {
	statePath string
}

// NewStateRepositoryImpl creates a new file-based state repository
func NewStateRepositoryImpl(statePath string) *StateRepositoryImpl {
	return &StateRepositoryImpl{
		statePath: statePath,
	}
}

// Load retrieves the current execution state from file
func (r *StateRepositoryImpl) Load(ctx context.Context) (*repository.ExecutionState, error) {
	b, err := os.ReadFile(r.statePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var fileState struct {
		Version        int               `json:"version"`
		Current        string            `json:"current"`
		Status         string            `json:"status"`
		Turn           int               `json:"turn"`
		WIP            string            `json:"wip"`
		LeaseExpiresAt string            `json:"lease_expires_at"`
		Inputs         map[string]string `json:"inputs"`
		LastArtifacts  map[string]string `json:"last_artifacts"`
		Decision       string            `json:"decision"`
		Attempt        int               `json:"attempt"`
		Meta           struct {
			UpdatedAt string `json:"updated_at"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(b, &fileState); err != nil {
		return nil, fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return &repository.ExecutionState{
		Version:        fileState.Version,
		Current:        fileState.Current,
		Status:         fileState.Status,
		Turn:           fileState.Turn,
		WIP:            fileState.WIP,
		LeaseExpiresAt: fileState.LeaseExpiresAt,
		Inputs:         fileState.Inputs,
		LastArtifacts:  fileState.LastArtifacts,
		Decision:       fileState.Decision,
		Attempt:        fileState.Attempt,
		UpdatedAt:      fileState.Meta.UpdatedAt,
	}, nil
}

// Save persists the execution state to file
func (r *StateRepositoryImpl) Save(ctx context.Context, state *repository.ExecutionState) error {
	// Increment version
	state.Version++
	state.UpdatedAt = time.Now().Local().Format(time.RFC3339)

	fileState := struct {
		Version        int               `json:"version"`
		Current        string            `json:"current"`
		Status         string            `json:"status"`
		Turn           int               `json:"turn"`
		WIP            string            `json:"wip"`
		LeaseExpiresAt string            `json:"lease_expires_at"`
		Inputs         map[string]string `json:"inputs"`
		LastArtifacts  map[string]string `json:"last_artifacts"`
		Decision       string            `json:"decision"`
		Attempt        int               `json:"attempt"`
		Meta           struct {
			UpdatedAt string `json:"updated_at"`
		} `json:"meta"`
	}{
		Version:        state.Version,
		Current:        state.Current,
		Status:         state.Status,
		Turn:           state.Turn,
		WIP:            state.WIP,
		LeaseExpiresAt: state.LeaseExpiresAt,
		Inputs:         state.Inputs,
		LastArtifacts:  state.LastArtifacts,
		Decision:       state.Decision,
		Attempt:        state.Attempt,
	}
	fileState.Meta.UpdatedAt = state.UpdatedAt

	if err := fs.AtomicWriteJSON(r.statePath, fileState); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// SaveAtomic performs atomic save of state and journal record
// For now, this saves them separately (not truly atomic)
// TODO: Implement true atomic transaction when migrating to SQLite
func (r *StateRepositoryImpl) SaveAtomic(ctx context.Context, state *repository.ExecutionState, journalRecord map[string]interface{}) error {
	// Save state first
	if err := r.Save(ctx, state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	// Save journal record
	// Note: This uses the global journal path - should be injected
	// For now, use the app package's journal writer
	// This is a temporary solution until we have proper dependency injection

	// TODO: Implement proper journal append here or remove this method
	// For now, journal is handled separately in the CLI layer

	return nil
}
