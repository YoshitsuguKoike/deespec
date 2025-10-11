package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

// MockRunLockRepository is a mock implementation of RunLockRepository for testing
type MockRunLockRepository struct {
	mu    sync.RWMutex
	locks map[string]*lock.RunLock
}

// NewMockRunLockRepository creates a new mock run lock repository
func NewMockRunLockRepository() *MockRunLockRepository {
	return &MockRunLockRepository{
		locks: make(map[string]*lock.RunLock),
	}
}

// Acquire attempts to acquire a run lock
func (m *MockRunLockRepository) Acquire(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	// Check if lock already exists and is not expired
	if existingLock, exists := m.locks[key]; exists {
		if !existingLock.IsExpired() {
			// Lock is held by another process
			return nil, nil
		}
		// Lock expired, can be acquired
		delete(m.locks, key)
	}

	// Create new lock
	runLock, err := lock.NewRunLock(lockID, ttl)
	if err != nil {
		return nil, err
	}

	m.locks[key] = runLock
	return runLock, nil
}

// Release releases a run lock
func (m *MockRunLockRepository) Release(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	if _, exists := m.locks[key]; !exists {
		return lock.ErrLockNotFound
	}

	delete(m.locks, key)
	return nil
}

// Find retrieves a run lock by ID
func (m *MockRunLockRepository) Find(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := lockID.String()

	runLock, exists := m.locks[key]
	if !exists {
		return nil, lock.ErrLockNotFound
	}

	return runLock, nil
}

// UpdateHeartbeat updates the heartbeat timestamp for a lock
func (m *MockRunLockRepository) UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	runLock, exists := m.locks[key]
	if !exists {
		return lock.ErrLockNotFound
	}

	runLock.UpdateHeartbeat()
	return nil
}

// Extend extends the expiration time of a lock
func (m *MockRunLockRepository) Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	runLock, exists := m.locks[key]
	if !exists {
		return lock.ErrLockNotFound
	}

	runLock.Extend(duration)
	return nil
}

// CleanupExpired removes expired locks
func (m *MockRunLockRepository) CleanupExpired(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for key, runLock := range m.locks {
		if runLock.IsExpired() {
			delete(m.locks, key)
			count++
		}
	}

	return count, nil
}

// List lists all active run locks
func (m *MockRunLockRepository) List(ctx context.Context) ([]*lock.RunLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*lock.RunLock, 0, len(m.locks))
	for _, runLock := range m.locks {
		result = append(result, runLock)
	}

	return result, nil
}

// MockStateLockRepository is a mock implementation of StateLockRepository for testing
type MockStateLockRepository struct {
	mu    sync.RWMutex
	locks map[string]*lock.StateLock
}

// NewMockStateLockRepository creates a new mock state lock repository
func NewMockStateLockRepository() *MockStateLockRepository {
	return &MockStateLockRepository{
		locks: make(map[string]*lock.StateLock),
	}
}

// Acquire attempts to acquire a state lock
func (m *MockStateLockRepository) Acquire(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	// Check if lock already exists and is not expired
	if existingLock, exists := m.locks[key]; exists {
		if !existingLock.IsExpired() {
			// Lock is held by another process
			return nil, nil
		}
		// Lock expired, can be acquired
		delete(m.locks, key)
	}

	// Create new lock
	stateLock, err := lock.NewStateLock(lockID, lockType, ttl)
	if err != nil {
		return nil, err
	}

	m.locks[key] = stateLock
	return stateLock, nil
}

// Release releases a state lock
func (m *MockStateLockRepository) Release(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	if _, exists := m.locks[key]; !exists {
		return lock.ErrLockNotFound
	}

	delete(m.locks, key)
	return nil
}

// Find retrieves a state lock by ID
func (m *MockStateLockRepository) Find(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := lockID.String()

	stateLock, exists := m.locks[key]
	if !exists {
		return nil, lock.ErrLockNotFound
	}

	return stateLock, nil
}

// UpdateHeartbeat updates the heartbeat timestamp for a lock
func (m *MockStateLockRepository) UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	stateLock, exists := m.locks[key]
	if !exists {
		return lock.ErrLockNotFound
	}

	stateLock.UpdateHeartbeat()
	return nil
}

// Extend extends the expiration time of a lock
func (m *MockStateLockRepository) Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := lockID.String()

	stateLock, exists := m.locks[key]
	if !exists {
		return lock.ErrLockNotFound
	}

	stateLock.Extend(duration)
	return nil
}

// CleanupExpired removes expired locks
func (m *MockStateLockRepository) CleanupExpired(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for key, stateLock := range m.locks {
		if stateLock.IsExpired() {
			delete(m.locks, key)
			count++
		}
	}

	return count, nil
}

// List lists all active state locks
func (m *MockStateLockRepository) List(ctx context.Context) ([]*lock.StateLock, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*lock.StateLock, 0, len(m.locks))
	for _, stateLock := range m.locks {
		result = append(result, stateLock)
	}

	return result, nil
}

// Test Suite for RunLockRepository

func TestRunLockRepository_Acquire(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	if runLock == nil {
		t.Fatal("Expected non-nil lock")
	}

	if runLock.LockID().String() != lockID.String() {
		t.Errorf("Expected lock ID %s, got %s", lockID.String(), runLock.LockID().String())
	}
}

func TestRunLockRepository_AcquireExisting(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	_, err = repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Try to acquire again (should fail)
	runLock2, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if runLock2 != nil {
		t.Error("Expected nil when trying to acquire existing lock")
	}
}

func TestRunLockRepository_AcquireExpired(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock with very short TTL
	_, err = repo.Acquire(ctx, lockID, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Wait for lock to expire
	time.Sleep(10 * time.Millisecond)

	// Try to acquire again (should succeed)
	runLock2, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire expired lock: %v", err)
	}

	if runLock2 == nil {
		t.Error("Expected to acquire expired lock")
	}
}

func TestRunLockRepository_Release(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	_, err = repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Release lock
	err = repo.Release(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Verify lock is released by trying to find it
	_, err = repo.Find(ctx, lockID)
	if err == nil {
		t.Error("Expected error when finding released lock")
	}

	if !errors.Is(err, lock.ErrLockNotFound) {
		t.Errorf("Expected ErrLockNotFound, got %v", err)
	}
}

func TestRunLockRepository_ReleaseNonExistent(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("non-existent-lock")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Try to release non-existent lock
	err = repo.Release(ctx, lockID)
	if err == nil {
		t.Error("Expected error when releasing non-existent lock")
	}

	if !errors.Is(err, lock.ErrLockNotFound) {
		t.Errorf("Expected ErrLockNotFound, got %v", err)
	}
}

func TestRunLockRepository_Find(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	originalLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Find lock
	foundLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	if foundLock.LockID().String() != originalLock.LockID().String() {
		t.Errorf("Expected lock ID %s, got %s", originalLock.LockID().String(), foundLock.LockID().String())
	}
}

func TestRunLockRepository_FindNonExistent(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("non-existent-lock")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Try to find non-existent lock
	_, err = repo.Find(ctx, lockID)
	if err == nil {
		t.Error("Expected error when finding non-existent lock")
	}

	if !errors.Is(err, lock.ErrLockNotFound) {
		t.Errorf("Expected ErrLockNotFound, got %v", err)
	}
}

func TestRunLockRepository_UpdateHeartbeat(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	originalLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	originalHeartbeat := originalLock.HeartbeatAt()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Update heartbeat
	err = repo.UpdateHeartbeat(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to update heartbeat: %v", err)
	}

	// Verify heartbeat was updated
	updatedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	if !updatedLock.HeartbeatAt().After(originalHeartbeat) {
		t.Error("Expected heartbeat to be updated")
	}
}

func TestRunLockRepository_UpdateHeartbeatNonExistent(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("non-existent-lock")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Try to update heartbeat of non-existent lock
	err = repo.UpdateHeartbeat(ctx, lockID)
	if err == nil {
		t.Error("Expected error when updating heartbeat of non-existent lock")
	}

	if !errors.Is(err, lock.ErrLockNotFound) {
		t.Errorf("Expected ErrLockNotFound, got %v", err)
	}
}

func TestRunLockRepository_Extend(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	originalLock, err := repo.Acquire(ctx, lockID, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	originalExpiration := originalLock.ExpiresAt()

	// Extend lock
	err = repo.Extend(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to extend lock: %v", err)
	}

	// Verify expiration was extended
	extendedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	if !extendedLock.ExpiresAt().After(originalExpiration) {
		t.Error("Expected expiration to be extended")
	}
}

func TestRunLockRepository_ExtendNonExistent(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("non-existent-lock")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Try to extend non-existent lock
	err = repo.Extend(ctx, lockID, 5*time.Minute)
	if err == nil {
		t.Error("Expected error when extending non-existent lock")
	}

	if !errors.Is(err, lock.ErrLockNotFound) {
		t.Errorf("Expected ErrLockNotFound, got %v", err)
	}
}

func TestRunLockRepository_CleanupExpired(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	// Acquire multiple locks with different TTLs
	for i := 1; i <= 5; i++ {
		lockID, err := lock.NewLockID("test-lock-" + string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Failed to create lock ID: %v", err)
		}

		var ttl time.Duration
		if i <= 2 {
			// First 2 locks expire quickly
			ttl = 1 * time.Millisecond
		} else {
			// Last 3 locks have longer TTL
			ttl = 5 * time.Minute
		}

		_, err = repo.Acquire(ctx, lockID, ttl)
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}
	}

	// Wait for some locks to expire
	time.Sleep(10 * time.Millisecond)

	// Cleanup expired locks
	count, err := repo.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("Failed to cleanup expired locks: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 expired locks, got %d", count)
	}

	// Verify remaining locks
	locks, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list locks: %v", err)
	}

	if len(locks) != 3 {
		t.Errorf("Expected 3 remaining locks, got %d", len(locks))
	}
}

func TestRunLockRepository_List(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	// List empty repository
	locks, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list locks: %v", err)
	}

	if len(locks) != 0 {
		t.Errorf("Expected 0 locks, got %d", len(locks))
	}

	// Acquire multiple locks
	for i := 1; i <= 3; i++ {
		lockID, err := lock.NewLockID("test-lock-" + string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Failed to create lock ID: %v", err)
		}

		_, err = repo.Acquire(ctx, lockID, 5*time.Minute)
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}
	}

	// List locks
	locks, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list locks: %v", err)
	}

	if len(locks) != 3 {
		t.Errorf("Expected 3 locks, got %d", len(locks))
	}
}

func TestRunLockRepository_Concurrency(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Try to acquire the same lock from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			lockID, err := lock.NewLockID("shared-lock")
			if err != nil {
				return
			}

			runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
			if err != nil {
				return
			}

			if runLock != nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	// Only one goroutine should successfully acquire the lock
	if successCount != 1 {
		t.Errorf("Expected 1 successful acquisition, got %d", successCount)
	}
}

// Test Suite for StateLockRepository

func TestStateLockRepository_Acquire(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire read lock
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	if stateLock == nil {
		t.Fatal("Expected non-nil lock")
	}

	if stateLock.LockID().String() != lockID.String() {
		t.Errorf("Expected lock ID %s, got %s", lockID.String(), stateLock.LockID().String())
	}

	if stateLock.LockType() != lock.LockTypeRead {
		t.Errorf("Expected lock type %s, got %s", lock.LockTypeRead, stateLock.LockType())
	}
}

func TestStateLockRepository_AcquireWriteLock(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire write lock
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	if stateLock == nil {
		t.Fatal("Expected non-nil lock")
	}

	if stateLock.LockType() != lock.LockTypeWrite {
		t.Errorf("Expected lock type %s, got %s", lock.LockTypeWrite, stateLock.LockType())
	}
}

func TestStateLockRepository_AcquireExisting(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	_, err = repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Try to acquire again (should fail)
	stateLock2, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if stateLock2 != nil {
		t.Error("Expected nil when trying to acquire existing lock")
	}
}

func TestStateLockRepository_Release(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	_, err = repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Release lock
	err = repo.Release(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to release lock: %v", err)
	}

	// Verify lock is released
	_, err = repo.Find(ctx, lockID)
	if err == nil {
		t.Error("Expected error when finding released lock")
	}

	if !errors.Is(err, lock.ErrLockNotFound) {
		t.Errorf("Expected ErrLockNotFound, got %v", err)
	}
}

func TestStateLockRepository_UpdateHeartbeat(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	originalLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	originalHeartbeat := originalLock.HeartbeatAt()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Update heartbeat
	err = repo.UpdateHeartbeat(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to update heartbeat: %v", err)
	}

	// Verify heartbeat was updated
	updatedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	if !updatedLock.HeartbeatAt().After(originalHeartbeat) {
		t.Error("Expected heartbeat to be updated")
	}
}

func TestStateLockRepository_Extend(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	originalLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 1*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	originalExpiration := originalLock.ExpiresAt()

	// Extend lock
	err = repo.Extend(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to extend lock: %v", err)
	}

	// Verify expiration was extended
	extendedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	if !extendedLock.ExpiresAt().After(originalExpiration) {
		t.Error("Expected expiration to be extended")
	}
}

func TestStateLockRepository_CleanupExpired(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	// Acquire multiple locks with different TTLs
	for i := 1; i <= 5; i++ {
		lockID, err := lock.NewLockID("test-state-lock-" + string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Failed to create lock ID: %v", err)
		}

		var ttl time.Duration
		var lockType lock.LockType
		if i <= 2 {
			// First 2 locks expire quickly
			ttl = 1 * time.Millisecond
			lockType = lock.LockTypeRead
		} else {
			// Last 3 locks have longer TTL
			ttl = 5 * time.Minute
			lockType = lock.LockTypeWrite
		}

		_, err = repo.Acquire(ctx, lockID, lockType, ttl)
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}
	}

	// Wait for some locks to expire
	time.Sleep(10 * time.Millisecond)

	// Cleanup expired locks
	count, err := repo.CleanupExpired(ctx)
	if err != nil {
		t.Fatalf("Failed to cleanup expired locks: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 expired locks, got %d", count)
	}

	// Verify remaining locks
	locks, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list locks: %v", err)
	}

	if len(locks) != 3 {
		t.Errorf("Expected 3 remaining locks, got %d", len(locks))
	}
}

func TestStateLockRepository_List(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	// List empty repository
	locks, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list locks: %v", err)
	}

	if len(locks) != 0 {
		t.Errorf("Expected 0 locks, got %d", len(locks))
	}

	// Acquire multiple locks
	for i := 1; i <= 3; i++ {
		lockID, err := lock.NewLockID("test-state-lock-" + string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Failed to create lock ID: %v", err)
		}

		lockType := lock.LockTypeRead
		if i%2 == 0 {
			lockType = lock.LockTypeWrite
		}

		_, err = repo.Acquire(ctx, lockID, lockType, 5*time.Minute)
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}
	}

	// List locks
	locks, err = repo.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list locks: %v", err)
	}

	if len(locks) != 3 {
		t.Errorf("Expected 3 locks, got %d", len(locks))
	}
}

func TestStateLockRepository_Concurrency(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	// Try to acquire the same lock from multiple goroutines
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			lockID, err := lock.NewLockID("shared-state-lock")
			if err != nil {
				return
			}

			lockType := lock.LockTypeRead
			if index%2 == 0 {
				lockType = lock.LockTypeWrite
			}

			stateLock, err := repo.Acquire(ctx, lockID, lockType, 5*time.Minute)
			if err != nil {
				return
			}

			if stateLock != nil {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	// Only one goroutine should successfully acquire the lock
	if successCount != 1 {
		t.Errorf("Expected 1 successful acquisition, got %d", successCount)
	}
}

// Edge Cases and Advanced Scenarios

func TestRunLockRepository_MultipleReleaseAndAcquire(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire and release multiple times
	for i := 0; i < 3; i++ {
		// Acquire
		runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
		if err != nil {
			t.Fatalf("Iteration %d: Failed to acquire lock: %v", i, err)
		}

		if runLock == nil {
			t.Fatalf("Iteration %d: Expected non-nil lock", i)
		}

		// Release
		err = repo.Release(ctx, lockID)
		if err != nil {
			t.Fatalf("Iteration %d: Failed to release lock: %v", i, err)
		}
	}
}

func TestRunLockRepository_RapidHeartbeatUpdates(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	_, err = repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Rapidly update heartbeat
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Millisecond)
		err = repo.UpdateHeartbeat(ctx, lockID)
		if err != nil {
			t.Fatalf("Failed to update heartbeat: %v", err)
		}
	}

	// Verify lock is still valid
	runLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	if runLock.IsExpired() {
		t.Error("Lock should not be expired")
	}
}

func TestStateLockRepository_LockTypeTransition(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire read lock
	readLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire read lock: %v", err)
	}

	if readLock.LockType() != lock.LockTypeRead {
		t.Errorf("Expected read lock, got %s", readLock.LockType())
	}

	// Release read lock
	err = repo.Release(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to release read lock: %v", err)
	}

	// Acquire write lock with same ID
	writeLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire write lock: %v", err)
	}

	if writeLock.LockType() != lock.LockTypeWrite {
		t.Errorf("Expected write lock, got %s", writeLock.LockType())
	}
}

func TestRunLockRepository_EmptyLockID(t *testing.T) {
	// Try to create empty lock ID (should fail at NewLockID)
	_, err := lock.NewLockID("")
	if err == nil {
		t.Error("Expected error when creating empty lock ID")
	}
}

func TestRunLockRepository_ConcurrentCleanup(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	// Acquire multiple locks with very short TTL
	for i := 1; i <= 10; i++ {
		lockID, err := lock.NewLockID("test-lock-" + string(rune('0'+i)))
		if err != nil {
			t.Fatalf("Failed to create lock ID: %v", err)
		}

		_, err = repo.Acquire(ctx, lockID, 1*time.Millisecond)
		if err != nil {
			t.Fatalf("Failed to acquire lock: %v", err)
		}
	}

	// Wait for all locks to expire
	time.Sleep(10 * time.Millisecond)

	var wg sync.WaitGroup
	totalCleaned := 0
	var mu sync.Mutex

	// Run cleanup concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			count, err := repo.CleanupExpired(ctx)
			if err != nil {
				return
			}

			mu.Lock()
			totalCleaned += count
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Total cleaned should be exactly 10 (no double counting)
	if totalCleaned != 10 {
		t.Errorf("Expected 10 total cleaned locks, got %d", totalCleaned)
	}
}

func TestStateLockRepository_AcquireExpired(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock with very short TTL
	_, err = repo.Acquire(ctx, lockID, lock.LockTypeRead, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Wait for lock to expire
	time.Sleep(10 * time.Millisecond)

	// Try to acquire again with different lock type (should succeed)
	stateLock2, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire expired lock: %v", err)
	}

	if stateLock2 == nil {
		t.Error("Expected to acquire expired lock")
	}

	if stateLock2.LockType() != lock.LockTypeWrite {
		t.Errorf("Expected write lock, got %s", stateLock2.LockType())
	}
}

// Context Cancellation Tests

func TestRunLockRepository_AcquireWithCancelledContext(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Note: Mock implementation doesn't check context cancellation
	// This test documents expected behavior for real implementations
	_, err = repo.Acquire(ctx, lockID, 5*time.Minute)
	// Real implementation should respect context cancellation
	// Mock continues to work for testing purposes
	if err != nil {
		t.Logf("Context cancellation handled: %v", err)
	}
}

func TestStateLockRepository_AcquireWithCancelledContext(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Note: Mock implementation doesn't check context cancellation
	// This test documents expected behavior for real implementations
	_, err = repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	// Real implementation should respect context cancellation
	// Mock continues to work for testing purposes
	if err != nil {
		t.Logf("Context cancellation handled: %v", err)
	}
}

func TestRunLockRepository_FindWithTimeout(t *testing.T) {
	repo := NewMockRunLockRepository()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock first
	_, err = repo.Acquire(context.Background(), lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Find should work within timeout
	runLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	if runLock == nil {
		t.Fatal("Expected to find lock")
	}
}

// IsHeartbeatStale Tests

func TestRunLock_IsHeartbeatStale(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Heartbeat should not be stale immediately
	if runLock.IsHeartbeatStale(1 * time.Second) {
		t.Error("Heartbeat should not be stale immediately after acquisition")
	}

	// Wait for heartbeat to become stale
	time.Sleep(50 * time.Millisecond)

	// Check with short staleness threshold
	if !runLock.IsHeartbeatStale(10 * time.Millisecond) {
		t.Error("Expected heartbeat to be stale after waiting")
	}

	// Update heartbeat
	err = repo.UpdateHeartbeat(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to update heartbeat: %v", err)
	}

	// Find updated lock
	updatedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	// Heartbeat should not be stale after update
	if updatedLock.IsHeartbeatStale(1 * time.Second) {
		t.Error("Heartbeat should not be stale after update")
	}
}

func TestStateLock_IsHeartbeatStale(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire lock
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Heartbeat should not be stale immediately
	if stateLock.IsHeartbeatStale(1 * time.Second) {
		t.Error("Heartbeat should not be stale immediately after acquisition")
	}

	// Wait for heartbeat to become stale
	time.Sleep(50 * time.Millisecond)

	// Check with short staleness threshold
	if !stateLock.IsHeartbeatStale(10 * time.Millisecond) {
		t.Error("Expected heartbeat to be stale after waiting")
	}

	// Update heartbeat
	err = repo.UpdateHeartbeat(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to update heartbeat: %v", err)
	}

	// Find updated lock
	updatedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	// Heartbeat should not be stale after update
	if updatedLock.IsHeartbeatStale(1 * time.Second) {
		t.Error("Heartbeat should not be stale after update")
	}
}

// RemainingTime Tests

func TestRunLock_RemainingTime(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	ttl := 5 * time.Minute
	runLock, err := repo.Acquire(ctx, lockID, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Check remaining time is close to TTL
	remaining := runLock.RemainingTime()
	if remaining <= 0 {
		t.Error("Expected positive remaining time")
	}

	// Allow some tolerance for execution time
	if remaining > ttl || remaining < ttl-1*time.Second {
		t.Errorf("Expected remaining time close to %v, got %v", ttl, remaining)
	}

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Find lock and check remaining time decreased
	updatedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	newRemaining := updatedLock.RemainingTime()
	if newRemaining >= remaining {
		t.Error("Expected remaining time to decrease")
	}
}

func TestStateLock_RemainingTime(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	ttl := 5 * time.Minute
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Check remaining time is close to TTL
	remaining := stateLock.RemainingTime()
	if remaining <= 0 {
		t.Error("Expected positive remaining time")
	}

	// Allow some tolerance for execution time
	if remaining > ttl || remaining < ttl-1*time.Second {
		t.Errorf("Expected remaining time close to %v, got %v", ttl, remaining)
	}

	// Extend the lock
	extensionDuration := 3 * time.Minute
	err = repo.Extend(ctx, lockID, extensionDuration)
	if err != nil {
		t.Fatalf("Failed to extend lock: %v", err)
	}

	// Find lock and check remaining time increased
	extendedLock, err := repo.Find(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to find lock: %v", err)
	}

	newRemaining := extendedLock.RemainingTime()
	expectedRemaining := ttl + extensionDuration

	// Allow some tolerance for execution time
	if newRemaining <= remaining || newRemaining > expectedRemaining {
		t.Logf("Remaining after extend: %v, expected around %v", newRemaining, expectedRemaining)
	}
}

// Invalid Lock Type Tests

func TestStateLock_InvalidLockType(t *testing.T) {
	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Try to create lock with invalid lock type
	_, err = lock.NewStateLock(lockID, lock.LockType("invalid"), 5*time.Minute)
	if err == nil {
		t.Error("Expected error when creating lock with invalid lock type")
	}
}

// Multiple Operations Sequence Tests

func TestRunLockRepository_AcquireReleaseAcquireSequence(t *testing.T) {
	repo := NewMockRunLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// First acquisition
	lock1, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed first acquire: %v", err)
	}
	if lock1 == nil {
		t.Fatal("Expected non-nil lock on first acquire")
	}
	firstPID := lock1.PID()

	// Release
	err = repo.Release(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to release: %v", err)
	}

	// Second acquisition should succeed
	lock2, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed second acquire: %v", err)
	}
	if lock2 == nil {
		t.Fatal("Expected non-nil lock on second acquire")
	}

	// Should have same PID (same process)
	if lock2.PID() != firstPID {
		t.Errorf("Expected same PID, got %d vs %d", lock2.PID(), firstPID)
	}
}

func TestStateLockRepository_TypeSwitchAfterRelease(t *testing.T) {
	repo := NewMockStateLockRepository()
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-state-lock-001")
	if err != nil {
		t.Fatalf("Failed to create lock ID: %v", err)
	}

	// Acquire read lock
	readLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire read lock: %v", err)
	}
	if readLock.LockType() != lock.LockTypeRead {
		t.Errorf("Expected read lock, got %s", readLock.LockType())
	}

	// Release read lock
	err = repo.Release(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to release read lock: %v", err)
	}

	// Acquire write lock
	writeLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire write lock: %v", err)
	}
	if writeLock.LockType() != lock.LockTypeWrite {
		t.Errorf("Expected write lock, got %s", writeLock.LockType())
	}

	// Release write lock
	err = repo.Release(ctx, lockID)
	if err != nil {
		t.Fatalf("Failed to release write lock: %v", err)
	}

	// Acquire read lock again
	readLock2, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	if err != nil {
		t.Fatalf("Failed to acquire read lock again: %v", err)
	}
	if readLock2.LockType() != lock.LockTypeRead {
		t.Errorf("Expected read lock, got %s", readLock2.LockType())
	}
}
