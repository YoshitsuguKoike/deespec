package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

// FBDraftRepository defines operations for FBDraft persistence
type FBDraftRepository interface {
	// PersistDraft saves an FBDraft to the file system
	PersistDraft(ctx context.Context, draft dto.FBDraft, sbiDir string) (string, error)

	// RecordDraftInJournal records an FBDraft in the journal
	RecordDraftInJournal(ctx context.Context, draft dto.FBDraft, journalPath string, turn int) error

	// IsAlreadyRegistered checks if a draft for the target task is already registered
	IsAlreadyRegistered(ctx context.Context, targetTaskID string, journalPath string) (bool, error)
}
