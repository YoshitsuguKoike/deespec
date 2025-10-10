package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// Mock SBI Repository for testing
type mockSBIRepo struct {
	sbis map[string]*sbi.SBI
}

func newMockSBIRepo() *mockSBIRepo {
	return &mockSBIRepo{
		sbis: make(map[string]*sbi.SBI),
	}
}

// Mock Lock Service for testing
type mockLockService struct {
	locks map[string]bool // lockID -> isLocked
}

func newMockLockService() *mockLockService {
	return &mockLockService{
		locks: make(map[string]bool),
	}
}

func (m *mockLockService) AcquireRunLock(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLockService) ReleaseRunLock(ctx context.Context, lockID lock.LockID) error {
	return fmt.Errorf("not implemented")
}

func (m *mockLockService) ExtendRunLock(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	return fmt.Errorf("not implemented")
}

func (m *mockLockService) FindRunLock(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLockService) ListRunLocks(ctx context.Context) ([]*lock.RunLock, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLockService) AcquireStateLock(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error) {
	// Check if already locked
	if m.locks[lockID.String()] {
		return nil, nil // Lock already held
	}

	// Acquire lock
	m.locks[lockID.String()] = true

	// Create mock state lock
	stateLock, _ := lock.NewStateLock(lockID, lockType, ttl)
	return stateLock, nil
}

func (m *mockLockService) ReleaseStateLock(ctx context.Context, lockID lock.LockID) error {
	delete(m.locks, lockID.String())
	return nil
}

func (m *mockLockService) ExtendStateLock(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	return nil
}

func (m *mockLockService) FindStateLock(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLockService) ListStateLocks(ctx context.Context) ([]*lock.StateLock, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLockService) Start(ctx context.Context) error {
	return nil
}

func (m *mockLockService) Stop() error {
	return nil
}

func (m *mockSBIRepo) Find(ctx context.Context, id repository.SBIID) (*sbi.SBI, error) {
	s, ok := m.sbis[string(id)]
	if !ok {
		return nil, fmt.Errorf("SBI not found: %s", id)
	}
	return s, nil
}

func (m *mockSBIRepo) Save(ctx context.Context, s *sbi.SBI) error {
	m.sbis[s.ID().String()] = s
	return nil
}

func (m *mockSBIRepo) Delete(ctx context.Context, id repository.SBIID) error {
	delete(m.sbis, string(id))
	return nil
}

func (m *mockSBIRepo) List(ctx context.Context, filter repository.SBIFilter) ([]*sbi.SBI, error) {
	var result []*sbi.SBI

	for _, s := range m.sbis {
		// Filter by statuses if specified
		if len(filter.Statuses) > 0 {
			matched := false
			for _, status := range filter.Statuses {
				if s.Status() == status {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		result = append(result, s)

		// Apply limit if specified
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}

	return result, nil
}

func (m *mockSBIRepo) FindByPBIID(ctx context.Context, pbiID repository.PBIID) ([]*sbi.SBI, error) {
	var result []*sbi.SBI
	for _, s := range m.sbis {
		parentID := s.ParentTaskID()
		if parentID != nil && parentID.String() == string(pbiID) {
			result = append(result, s)
		}
	}
	return result, nil
}

func (m *mockSBIRepo) GetNextSequence(ctx context.Context) (int, error) {
	return len(m.sbis) + 1, nil
}

func (m *mockSBIRepo) ResetSBIState(ctx context.Context, id repository.SBIID, toStatus string) error {
	return nil // Not implemented for tests
}

func TestSBIExecutionService_PickNextSBI_InProgress(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBIs
	sbi1, err := sbi.NewSBI("Task 1", "Description 1", nil, sbi.SBIMetadata{})
	require.NoError(t, err)
	err = sbi1.UpdateStatus(model.StatusPicked)
	require.NoError(t, err)

	sbi2, err := sbi.NewSBI("Task 2", "Description 2", nil, sbi.SBIMetadata{})
	require.NoError(t, err)
	// sbi2 is PENDING by default

	// Save to repository
	err = repo.Save(ctx, sbi1)
	require.NoError(t, err)
	err = repo.Save(ctx, sbi2)
	require.NoError(t, err)

	// Execute
	picked, err := service.PickNextSBI(ctx)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, picked)
	assert.Equal(t, sbi1.ID().String(), picked.ID().String())
	assert.Equal(t, model.StatusPicked, picked.Status())
}

func TestSBIExecutionService_PickNextSBI_Pending(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBIs
	sbi1, err := sbi.NewSBI("Task 1", "Description 1", nil, sbi.SBIMetadata{})
	require.NoError(t, err)
	// sbi1 is PENDING by default

	// Create sbi2 with DONE status using ReconstructSBI
	// (Direct PENDING->DONE transition is not allowed by status validation)
	taskID2, err := model.NewTaskIDFromString("SBI-002")
	require.NoError(t, err)
	sbi2 := sbi.ReconstructSBI(
		taskID2,
		"Task 2",
		"Description 2",
		model.StatusDone,
		model.StepDone,
		nil,
		sbi.SBIMetadata{},
		&sbi.ExecutionState{
			CurrentTurn:    model.NewTurn(),
			CurrentAttempt: model.NewAttempt(),
			MaxTurns:       8,
			MaxAttempts:    3,
		},
		model.NewTimestamp().Value(),
		model.NewTimestamp().Value(),
	)

	// Save to repository
	err = repo.Save(ctx, sbi1)
	require.NoError(t, err)
	err = repo.Save(ctx, sbi2)
	require.NoError(t, err)

	// Execute
	picked, err := service.PickNextSBI(ctx)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, picked)
	assert.Equal(t, sbi1.ID().String(), picked.ID().String())
	assert.Equal(t, model.StatusPending, picked.Status())
}

func TestSBIExecutionService_PickNextSBI_NoTasks(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create only completed SBI using ReconstructSBI
	taskID, err := model.NewTaskIDFromString("SBI-001")
	require.NoError(t, err)
	sbi1 := sbi.ReconstructSBI(
		taskID,
		"Task 1",
		"Description 1",
		model.StatusDone,
		model.StepDone,
		nil,
		sbi.SBIMetadata{},
		&sbi.ExecutionState{
			CurrentTurn:    model.NewTurn(),
			CurrentAttempt: model.NewAttempt(),
			MaxTurns:       8,
			MaxAttempts:    3,
		},
		model.NewTimestamp().Value(),
		model.NewTimestamp().Value(),
	)

	err = repo.Save(ctx, sbi1)
	require.NoError(t, err)

	// Execute
	picked, err := service.PickNextSBI(ctx)

	// Verify
	require.NoError(t, err)
	assert.Nil(t, picked) // No tasks available
}

func TestSBIExecutionService_GetSBIByID(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBI
	testSBI, err := sbi.NewSBI("Test Task", "Test Description", nil, sbi.SBIMetadata{})
	require.NoError(t, err)

	err = repo.Save(ctx, testSBI)
	require.NoError(t, err)

	// Execute
	retrieved, err := service.GetSBIByID(ctx, testSBI.ID().String())

	// Verify
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, testSBI.ID().String(), retrieved.ID().String())
	assert.Equal(t, testSBI.Title(), retrieved.Title())
}

func TestSBIExecutionService_UpdateSBI(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBI
	testSBI, err := sbi.NewSBI("Test Task", "Test Description", nil, sbi.SBIMetadata{})
	require.NoError(t, err)
	// testSBI is PENDING by default

	err = repo.Save(ctx, testSBI)
	require.NoError(t, err)

	// Update status
	err = testSBI.UpdateStatus(model.StatusPicked)
	require.NoError(t, err)

	// Execute
	err = service.UpdateSBI(ctx, testSBI)

	// Verify
	require.NoError(t, err)

	// Retrieve and verify update
	sbiID := repository.SBIID(testSBI.ID().String())
	retrieved, err := repo.Find(ctx, sbiID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusPicked, retrieved.Status())
}

func TestSBIExecutionService_AcquireSBILock(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBI
	testSBI, err := sbi.NewSBI("Test Task", "Test Description", nil, sbi.SBIMetadata{})
	require.NoError(t, err)

	// Acquire lock
	stateLock, err := service.AcquireSBILock(ctx, testSBI.ID().String(), 5*time.Minute)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, stateLock)
	assert.Equal(t, "sbi/"+testSBI.ID().String(), stateLock.LockID().String())

	// Verify lock is held in mock service
	assert.True(t, lockService.locks["sbi/"+testSBI.ID().String()])
}

func TestSBIExecutionService_AcquireSBILock_AlreadyLocked(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBI
	testSBI, err := sbi.NewSBI("Test Task", "Test Description", nil, sbi.SBIMetadata{})
	require.NoError(t, err)

	// First lock acquisition
	lock1, err := service.AcquireSBILock(ctx, testSBI.ID().String(), 5*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, lock1)

	// Second lock acquisition (should fail - lock already held)
	lock2, err := service.AcquireSBILock(ctx, testSBI.ID().String(), 5*time.Minute)
	require.NoError(t, err)
	assert.Nil(t, lock2) // Lock already held
}

func TestSBIExecutionService_ReleaseSBILock(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBI
	testSBI, err := sbi.NewSBI("Test Task", "Test Description", nil, sbi.SBIMetadata{})
	require.NoError(t, err)

	// Acquire lock
	stateLock, err := service.AcquireSBILock(ctx, testSBI.ID().String(), 5*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, stateLock)

	// Verify lock is held
	assert.True(t, lockService.locks["sbi/"+testSBI.ID().String()])

	// Release lock
	err = service.ReleaseSBILock(ctx, testSBI.ID().String())
	require.NoError(t, err)

	// Verify lock is released
	assert.False(t, lockService.locks["sbi/"+testSBI.ID().String()])
}

func TestSBIExecutionService_PickAndLockNextSBI(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBI
	testSBI, err := sbi.NewSBI("Test Task", "Test Description", nil, sbi.SBIMetadata{})
	require.NoError(t, err)

	err = repo.Save(ctx, testSBI)
	require.NoError(t, err)

	// Execute pick and lock
	pickedSBI, stateLock, err := service.PickAndLockNextSBI(ctx, 5*time.Minute)

	// Verify
	require.NoError(t, err)
	require.NotNil(t, pickedSBI)
	require.NotNil(t, stateLock)
	assert.Equal(t, testSBI.ID().String(), pickedSBI.ID().String())
	assert.True(t, lockService.locks["sbi/"+testSBI.ID().String()])
}

func TestSBIExecutionService_PickAndLockNextSBI_NoTasks(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Execute pick and lock (no tasks available)
	pickedSBI, stateLock, err := service.PickAndLockNextSBI(ctx, 5*time.Minute)

	// Verify
	require.NoError(t, err)
	assert.Nil(t, pickedSBI)
	assert.Nil(t, stateLock)
}

func TestSBIExecutionService_PickAndLockNextSBI_LockAlreadyHeld(t *testing.T) {
	// Setup
	repo := newMockSBIRepo()
	lockService := newMockLockService()
	service := NewSBIExecutionService(repo, lockService)
	ctx := context.Background()

	// Create test SBI
	testSBI, err := sbi.NewSBI("Test Task", "Test Description", nil, sbi.SBIMetadata{})
	require.NoError(t, err)

	err = repo.Save(ctx, testSBI)
	require.NoError(t, err)

	// Pre-acquire the lock
	lockID, err := lock.NewLockID("sbi/" + testSBI.ID().String())
	require.NoError(t, err)
	_, err = lockService.AcquireStateLock(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)

	// Execute pick and lock (should fail - lock already held)
	pickedSBI, stateLock, err := service.PickAndLockNextSBI(ctx, 5*time.Minute)

	// Verify
	require.NoError(t, err)
	assert.Nil(t, pickedSBI) // Lock already held
	assert.Nil(t, stateLock)
}
