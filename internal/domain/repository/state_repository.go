package repository

import "context"

// ExecutionState represents the current execution state
// This is a temporary structure until we migrate State from CLI layer to Domain layer
type ExecutionState struct {
	Version        int
	Current        string // Legacy: plan/implement/test/review/done
	Status         string // New: READY/WIP/REVIEW/REVIEW&WIP/DONE
	Turn           int
	WIP            string // Work In Progress - current SBI ID (empty = no WIP)
	LeaseExpiresAt string // UTC RFC3339Nano (empty when no WIP)
	Inputs         map[string]string
	LastArtifacts  map[string]string
	Decision       string // PENDING/NEEDS_CHANGES/SUCCEEDED/FAILED
	Attempt        int    // Implementation attempt number (1-3)
	UpdatedAt      string
}

// StateRepository manages the execution state persistence
type StateRepository interface {
	// Load retrieves the current execution state
	Load(ctx context.Context) (*ExecutionState, error)

	// Save persists the execution state
	// Returns error if version mismatch (optimistic locking)
	Save(ctx context.Context, state *ExecutionState) error

	// SaveAtomic performs atomic save of state and journal record
	SaveAtomic(ctx context.Context, state *ExecutionState, journalRecord map[string]interface{}) error
}
