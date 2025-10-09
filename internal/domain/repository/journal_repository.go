package repository

import "context"

// JournalRecord represents a single journal entry
type JournalRecord struct {
	Timestamp string        // UTC RFC3339Nano
	Turn      int           // Turn number
	Step      string        // Workflow step
	Status    string        // Execution status
	Attempt   int           // Attempt number
	Decision  string        // Review decision
	ElapsedMs int64         // Execution time in milliseconds
	Error     string        // Error message if any
	Artifacts []interface{} // Artifact paths and metadata
}

// JournalRepository manages execution journal persistence
type JournalRepository interface {
	// Append adds a new record to the journal
	Append(ctx context.Context, record *JournalRecord) error

	// Load retrieves all journal records
	Load(ctx context.Context) ([]*JournalRecord, error)

	// FindByTurn retrieves records for a specific turn
	FindByTurn(ctx context.Context, turn int) ([]*JournalRecord, error)

	// FindBySBI retrieves records for a specific SBI
	// This requires journal records to include SBI ID
	FindBySBI(ctx context.Context, sbiID string) ([]*JournalRecord, error)
}
