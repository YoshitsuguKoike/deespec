package service

import (
	"context"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// SBIExecutionService provides business logic for SBI execution management
type SBIExecutionService struct {
	sbiRepo     repository.SBIRepository
	lockService LockService
}

// NewSBIExecutionService creates a new SBI execution service
func NewSBIExecutionService(sbiRepo repository.SBIRepository, lockService LockService) *SBIExecutionService {
	return &SBIExecutionService{
		sbiRepo:     sbiRepo,
		lockService: lockService,
	}
}

// PickNextSBI selects the next SBI to execute based on priority rules
// Priority:
// 1. SBIs in PICKED or IMPLEMENTING status (continue implementation)
// 2. SBIs in REVIEWING status (continue review process)
// 3. SBIs in PENDING status (start new execution)
func (s *SBIExecutionService) PickNextSBI(ctx context.Context) (*sbi.SBI, error) {
	// First, try to find SBIs that are already in progress (PICKED, IMPLEMENTING, or REVIEWING)
	// These should be prioritized to continue existing work
	inProgressFilter := repository.SBIFilter{
		Statuses: []model.Status{
			model.StatusPicked,
			model.StatusImplementing,
			model.StatusReviewing, // Added: Review is part of the workflow
		},
		Limit: 1,
	}

	inProgressSBIs, err := s.sbiRepo.List(ctx, inProgressFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list in-progress SBIs: %w", err)
	}

	if len(inProgressSBIs) > 0 {
		// Found an in-progress SBI, return it
		return inProgressSBIs[0], nil
	}

	// No in-progress SBIs found, look for pending SBIs to start new work
	pendingFilter := repository.SBIFilter{
		Statuses: []model.Status{model.StatusPending},
		Limit:    1,
	}

	pendingSBIs, err := s.sbiRepo.List(ctx, pendingFilter)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending SBIs: %w", err)
	}

	if len(pendingSBIs) > 0 {
		// Found a pending SBI, return it
		return pendingSBIs[0], nil
	}

	// No tasks available to execute
	return nil, nil
}

// GetSBIByID retrieves an SBI by its ID
func (s *SBIExecutionService) GetSBIByID(ctx context.Context, id string) (*sbi.SBI, error) {
	if id == "" {
		return nil, fmt.Errorf("SBI ID cannot be empty")
	}

	sbiID := repository.SBIID(id)
	return s.sbiRepo.Find(ctx, sbiID)
}

// UpdateSBI updates an SBI entity in the repository
func (s *SBIExecutionService) UpdateSBI(ctx context.Context, sbi *sbi.SBI) error {
	return s.sbiRepo.Save(ctx, sbi)
}

// AcquireSBILock acquires a lock for an SBI to prevent concurrent execution
// Returns the acquired lock or nil if lock is already held by another process
func (s *SBIExecutionService) AcquireSBILock(ctx context.Context, sbiID string, ttl time.Duration) (*lock.StateLock, error) {
	if s.lockService == nil {
		// Lock service not available, skip locking
		return nil, nil
	}

	lockID, err := lock.NewLockID(fmt.Sprintf("sbi/%s", sbiID))
	if err != nil {
		return nil, fmt.Errorf("failed to create lock ID: %w", err)
	}

	stateLock, err := s.lockService.AcquireStateLock(ctx, lockID, lock.LockTypeWrite, ttl)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire SBI lock: %w", err)
	}

	return stateLock, nil
}

// ReleaseSBILock releases the lock for an SBI
func (s *SBIExecutionService) ReleaseSBILock(ctx context.Context, sbiID string) error {
	if s.lockService == nil {
		// Lock service not available, skip unlocking
		return nil
	}

	lockID, err := lock.NewLockID(fmt.Sprintf("sbi/%s", sbiID))
	if err != nil {
		return fmt.Errorf("failed to create lock ID: %w", err)
	}

	if err := s.lockService.ReleaseStateLock(ctx, lockID); err != nil {
		return fmt.Errorf("failed to release SBI lock: %w", err)
	}

	return nil
}

// PickAndLockNextSBI picks the next SBI and acquires a lock for it
// Returns the SBI and lock, or (nil, nil, nil) if no tasks available or lock already held
func (s *SBIExecutionService) PickAndLockNextSBI(ctx context.Context, ttl time.Duration) (*sbi.SBI, *lock.StateLock, error) {
	// Pick next SBI
	nextSBI, err := s.PickNextSBI(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to pick next SBI: %w", err)
	}

	if nextSBI == nil {
		// No tasks available
		return nil, nil, nil
	}

	// Try to acquire lock for this SBI
	sbiLock, err := s.AcquireSBILock(ctx, nextSBI.ID().String(), ttl)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to acquire lock for SBI %s: %w", nextSBI.ID(), err)
	}

	if sbiLock == nil {
		// Lock already held by another process, return nil to indicate unavailable
		return nil, nil, nil
	}

	return nextSBI, sbiLock, nil
}
