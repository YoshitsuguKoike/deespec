package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

// SBITaskRepository defines operations for loading and querying SBI tasks
// from the file-based system (legacy architecture)
type SBITaskRepository interface {
	// LoadAllTasks loads all tasks from the specs directory
	LoadAllTasks(ctx context.Context, specsDir string) ([]*dto.SBITaskDTO, error)

	// GetCompletedTasks returns a map of completed task IDs from journal
	GetCompletedTasks(ctx context.Context, journalPath string) (map[string]bool, error)

	// GetLastJournalEntry returns the last journal entry
	GetLastJournalEntry(ctx context.Context, journalPath string) (map[string]interface{}, error)

	// RecordPickInJournal records a task pick event in the journal
	RecordPickInJournal(ctx context.Context, task *dto.SBITaskDTO, turn int, journalPath string) error
}
