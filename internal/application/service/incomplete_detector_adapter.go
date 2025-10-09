package service

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// IncompleteDetectorAdapter adapts IncompleteDetectionService and FBDraftRepository
// to implement the IncompleteDetector interface
type IncompleteDetectorAdapter struct {
	service    *IncompleteDetectionService
	repository repository.FBDraftRepository
}

// NewIncompleteDetectorAdapter creates a new adapter
func NewIncompleteDetectorAdapter(
	service *IncompleteDetectionService,
	repository repository.FBDraftRepository,
) *IncompleteDetectorAdapter {
	return &IncompleteDetectorAdapter{
		service:    service,
		repository: repository,
	}
}

// DetectIncomplete implements IncompleteDetector.DetectIncomplete
func (a *IncompleteDetectorAdapter) DetectIncomplete(
	ctx context.Context,
	task *dto.SBITaskDTO,
	pickContext *dto.PickContext,
) ([]dto.FBDraft, error) {
	return a.service.DetectIncomplete(ctx, task, pickContext)
}

// PersistFBDraft implements IncompleteDetector.PersistFBDraft
func (a *IncompleteDetectorAdapter) PersistFBDraft(
	ctx context.Context,
	draft dto.FBDraft,
	sbiDir string,
) (string, error) {
	return a.repository.PersistDraft(ctx, draft, sbiDir)
}

// RecordFBDraftInJournal implements IncompleteDetector.RecordFBDraftInJournal
func (a *IncompleteDetectorAdapter) RecordFBDraftInJournal(
	ctx context.Context,
	draft dto.FBDraft,
	journalPath string,
	turn int,
) error {
	return a.repository.RecordDraftInJournal(ctx, draft, journalPath, turn)
}
