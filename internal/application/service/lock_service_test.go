package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

// MockRunLockRepository is a mock implementation of RunLockRepository for testing
type MockRunLockRepository struct {
	mu               sync.RWMutex
	acquiredLocks    map[string]*lock.RunLock
	heartbeatCounts  map[string]int
	cleanupCallCount int
}

func NewMockRunLockRepository() *MockRunLockRepository {
	return &MockRunLockRepository{
		acquiredLocks:   make(map[string]*lock.RunLock),
		heartbeatCounts: make(map[string]int),
	}
}

func (m *MockRunLockRepository) Acquire(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.acquiredLocks[lockID.String()]; exists {
		return nil, nil // Already acquired
	}

	runLock, err := lock.NewRunLock(lockID, ttl)
	if err != nil {
		return nil, err
	}

	m.acquiredLocks[lockID.String()] = runLock
	m.heartbeatCounts[lockID.String()] = 0
	return runLock, nil
}

func (m *MockRunLockRepository) Release(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.acquiredLocks, lockID.String())
	delete(m.heartbeatCounts, lockID.String())
	return nil
}

func (m *MockRunLockRepository) Find(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if runLock, exists := m.acquiredLocks[lockID.String()]; exists {
		return runLock, nil
	}
	return nil, lock.ErrLockNotFound
}

func (m *MockRunLockRepository) UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.acquiredLocks[lockID.String()]; !exists {
		return lock.ErrLockNotFound
	}
	m.heartbeatCounts[lockID.String()]++
	return nil
}

func (m *MockRunLockRepository) Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if runLock, exists := m.acquiredLocks[lockID.String()]; exists {
		// Update expiration in the existing lock
		_ = runLock
		return nil
	}
	return lock.ErrLockNotFound
}

func (m *MockRunLockRepository) CleanupExpired(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupCallCount++
	return 0, nil
}

func (m *MockRunLockRepository) List(ctx context.Context) ([]*lock.RunLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	locks := make([]*lock.RunLock, 0, len(m.acquiredLocks))
	for _, l := range m.acquiredLocks {
		locks = append(locks, l)
	}
	return locks, nil
}

// GetHeartbeatCount returns the heartbeat count for a lock (thread-safe)
func (m *MockRunLockRepository) GetHeartbeatCount(lockID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.heartbeatCounts[lockID]
}

// GetCleanupCallCount returns the cleanup call count (thread-safe)
func (m *MockRunLockRepository) GetCleanupCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cleanupCallCount
}

// GetLocksCount returns the number of acquired locks (thread-safe)
func (m *MockRunLockRepository) GetLocksCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.acquiredLocks)
}

// MockStateLockRepository is a mock implementation of StateLockRepository for testing
type MockStateLockRepository struct {
	mu               sync.RWMutex
	acquiredLocks    map[string]*lock.StateLock
	heartbeatCounts  map[string]int
	cleanupCallCount int
}

func NewMockStateLockRepository() *MockStateLockRepository {
	return &MockStateLockRepository{
		acquiredLocks:   make(map[string]*lock.StateLock),
		heartbeatCounts: make(map[string]int),
	}
}

func (m *MockStateLockRepository) Acquire(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.acquiredLocks[lockID.String()]; exists {
		return nil, nil // Already acquired
	}

	stateLock, err := lock.NewStateLock(lockID, lockType, ttl)
	if err != nil {
		return nil, err
	}

	m.acquiredLocks[lockID.String()] = stateLock
	m.heartbeatCounts[lockID.String()] = 0
	return stateLock, nil
}

func (m *MockStateLockRepository) Release(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.acquiredLocks, lockID.String())
	delete(m.heartbeatCounts, lockID.String())
	return nil
}

func (m *MockStateLockRepository) Find(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if stateLock, exists := m.acquiredLocks[lockID.String()]; exists {
		return stateLock, nil
	}
	return nil, lock.ErrLockNotFound
}

func (m *MockStateLockRepository) UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.acquiredLocks[lockID.String()]; !exists {
		return lock.ErrLockNotFound
	}
	m.heartbeatCounts[lockID.String()]++
	return nil
}

func (m *MockStateLockRepository) Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stateLock, exists := m.acquiredLocks[lockID.String()]; exists {
		// Update expiration in the existing lock
		_ = stateLock
		return nil
	}
	return lock.ErrLockNotFound
}

func (m *MockStateLockRepository) CleanupExpired(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupCallCount++
	return 0, nil
}

func (m *MockStateLockRepository) List(ctx context.Context) ([]*lock.StateLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	locks := make([]*lock.StateLock, 0, len(m.acquiredLocks))
	for _, l := range m.acquiredLocks {
		locks = append(locks, l)
	}
	return locks, nil
}

// GetHeartbeatCount returns the heartbeat count for a lock (thread-safe)
func (m *MockStateLockRepository) GetHeartbeatCount(lockID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.heartbeatCounts[lockID]
}

// GetCleanupCallCount returns the cleanup call count (thread-safe)
func (m *MockStateLockRepository) GetCleanupCallCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cleanupCallCount
}

// GetLocksCount returns the number of acquired locks (thread-safe)
func (m *MockStateLockRepository) GetLocksCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.acquiredLocks)
}

func TestLockService_AcquireAndReleaseRunLock(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := LockServiceConfig{
		HeartbeatInterval: 100 * time.Millisecond,
		CleanupInterval:   1 * time.Second,
	}

	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	lockID, err := lock.NewLockID("test-run-lock-001")
	require.NoError(t, err)

	// Acquire lock
	runLock, err := service.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock)
	assert.Equal(t, lockID, runLock.LockID())

	// Verify lock is in repository
	assert.Equal(t, 1, runLockRepo.GetLocksCount())

	// Release lock
	err = service.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)

	// Verify lock is removed from repository
	assert.Equal(t, 0, runLockRepo.GetLocksCount())
}

func TestLockService_AcquireAndReleaseStateLock(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := LockServiceConfig{
		HeartbeatInterval: 100 * time.Millisecond,
		CleanupInterval:   1 * time.Second,
	}

	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	lockID, err := lock.NewLockID("test-state-lock-001")
	require.NoError(t, err)

	// Acquire write lock
	stateLock, err := service.AcquireStateLock(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, stateLock)
	assert.Equal(t, lockID, stateLock.LockID())
	assert.Equal(t, lock.LockTypeWrite, stateLock.LockType())

	// Verify lock is in repository
	assert.Equal(t, 1, stateLockRepo.GetLocksCount())

	// Release lock
	err = service.ReleaseStateLock(ctx, lockID)
	require.NoError(t, err)

	// Verify lock is removed from repository
	assert.Equal(t, 0, stateLockRepo.GetLocksCount())
}

func TestLockService_RunLockHeartbeat(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := LockServiceConfig{
		HeartbeatInterval: 100 * time.Millisecond, // Fast heartbeat for testing
		CleanupInterval:   1 * time.Second,
	}

	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	lockID, err := lock.NewLockID("test-heartbeat-001")
	require.NoError(t, err)

	// Acquire lock
	_, err = service.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)

	// Wait for at least 3 heartbeats
	time.Sleep(350 * time.Millisecond)

	// Verify heartbeats were sent
	assert.GreaterOrEqual(t, runLockRepo.GetHeartbeatCount(lockID.String()), 3)

	// Release lock
	err = service.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)

	// Record heartbeat count after release
	countAfterRelease := runLockRepo.GetHeartbeatCount(lockID.String())

	// Wait a bit more
	time.Sleep(200 * time.Millisecond)

	// Verify heartbeats stopped (count should remain the same)
	assert.Equal(t, countAfterRelease, runLockRepo.GetHeartbeatCount(lockID.String()))
}

func TestLockService_StateLockHeartbeat(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := LockServiceConfig{
		HeartbeatInterval: 100 * time.Millisecond, // Fast heartbeat for testing
		CleanupInterval:   1 * time.Second,
	}

	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	lockID, err := lock.NewLockID("test-heartbeat-002")
	require.NoError(t, err)

	// Acquire lock
	_, err = service.AcquireStateLock(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)

	// Wait for at least 3 heartbeats
	time.Sleep(350 * time.Millisecond)

	// Verify heartbeats were sent
	assert.GreaterOrEqual(t, stateLockRepo.GetHeartbeatCount(lockID.String()), 3)

	// Release lock
	err = service.ReleaseStateLock(ctx, lockID)
	require.NoError(t, err)

	// Record heartbeat count after release
	countAfterRelease := stateLockRepo.GetHeartbeatCount(lockID.String())

	// Wait a bit more
	time.Sleep(200 * time.Millisecond)

	// Verify heartbeats stopped (count should remain the same)
	assert.Equal(t, countAfterRelease, stateLockRepo.GetHeartbeatCount(lockID.String()))
}

func TestLockService_CleanupScheduler(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := LockServiceConfig{
		HeartbeatInterval: 1 * time.Second,
		CleanupInterval:   200 * time.Millisecond, // Fast cleanup for testing
	}

	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)
	defer service.Stop()

	// Wait for at least 2 cleanup cycles
	time.Sleep(500 * time.Millisecond)

	// Verify cleanup was called multiple times
	assert.GreaterOrEqual(t, runLockRepo.GetCleanupCallCount(), 2)
	assert.GreaterOrEqual(t, stateLockRepo.GetCleanupCallCount(), 2)
}

func TestLockService_Stop(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := LockServiceConfig{
		HeartbeatInterval: 100 * time.Millisecond,
		CleanupInterval:   100 * time.Millisecond,
	}

	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	// Start service
	err := service.Start(ctx)
	require.NoError(t, err)

	// Acquire multiple locks
	lockID1, _ := lock.NewLockID("test-stop-001")
	lockID2, _ := lock.NewLockID("test-stop-002")

	_, err = service.AcquireRunLock(ctx, lockID1, 5*time.Minute)
	require.NoError(t, err)

	_, err = service.AcquireStateLock(ctx, lockID2, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)

	// Wait for some heartbeats
	time.Sleep(250 * time.Millisecond)

	// Record counts before stop
	runHeartbeatsBefore := runLockRepo.GetHeartbeatCount(lockID1.String())
	stateHeartbeatsBefore := stateLockRepo.GetHeartbeatCount(lockID2.String())
	cleanupCallsBefore := runLockRepo.GetCleanupCallCount()

	// Stop service
	err = service.Stop()
	require.NoError(t, err)

	// Wait a bit
	time.Sleep(250 * time.Millisecond)

	// Verify heartbeats stopped
	assert.Equal(t, runHeartbeatsBefore, runLockRepo.GetHeartbeatCount(lockID1.String()))
	assert.Equal(t, stateHeartbeatsBefore, stateLockRepo.GetHeartbeatCount(lockID2.String()))

	// Verify cleanup stopped
	assert.Equal(t, cleanupCallsBefore, runLockRepo.GetCleanupCallCount())
}

func TestLockService_ExtendRunLock(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := DefaultLockServiceConfig()
	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-extend-001")
	require.NoError(t, err)

	// Acquire lock
	_, err = service.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)

	// Extend lock
	err = service.ExtendRunLock(ctx, lockID, 10*time.Minute)
	require.NoError(t, err)

	// Release lock
	err = service.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)
}

func TestLockService_ExtendStateLock(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := DefaultLockServiceConfig()
	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-extend-002")
	require.NoError(t, err)

	// Acquire lock
	_, err = service.AcquireStateLock(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)

	// Extend lock
	err = service.ExtendStateLock(ctx, lockID, 10*time.Minute)
	require.NoError(t, err)

	// Release lock
	err = service.ReleaseStateLock(ctx, lockID)
	require.NoError(t, err)
}

func TestLockService_ListLocks(t *testing.T) {
	runLockRepo := NewMockRunLockRepository()
	stateLockRepo := NewMockStateLockRepository()

	config := DefaultLockServiceConfig()
	service := NewLockService(runLockRepo, stateLockRepo, config)
	ctx := context.Background()

	// Acquire multiple run locks
	lockID1, _ := lock.NewLockID("test-list-001")
	lockID2, _ := lock.NewLockID("test-list-002")

	_, err := service.AcquireRunLock(ctx, lockID1, 5*time.Minute)
	require.NoError(t, err)

	_, err = service.AcquireRunLock(ctx, lockID2, 5*time.Minute)
	require.NoError(t, err)

	// List run locks
	runLocks, err := service.ListRunLocks(ctx)
	require.NoError(t, err)
	assert.Len(t, runLocks, 2)

	// Acquire multiple state locks
	lockID3, _ := lock.NewLockID("test-list-003")
	lockID4, _ := lock.NewLockID("test-list-004")

	_, err = service.AcquireStateLock(ctx, lockID3, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)

	_, err = service.AcquireStateLock(ctx, lockID4, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)

	// List state locks
	stateLocks, err := service.ListStateLocks(ctx)
	require.NoError(t, err)
	assert.Len(t, stateLocks, 2)
}
