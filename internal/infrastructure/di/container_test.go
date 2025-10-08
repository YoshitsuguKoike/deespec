package di

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

func TestContainer_LockServiceIntegration(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create container with test configuration
	config := Config{
		DBPath:                dbPath,
		StorageType:           "mock",
		LockHeartbeatInterval: 100 * time.Millisecond, // Fast for testing
		LockCleanupInterval:   200 * time.Millisecond, // Fast for testing
	}

	container, err := NewContainer(config)
	require.NoError(t, err)
	defer container.Close()

	// Verify Lock Service is initialized
	lockService := container.GetLockService()
	require.NotNil(t, lockService)

	// Start Lock Service
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	// Test RunLock acquisition
	lockID1, err := lock.NewLockID("test-runlock-001")
	require.NoError(t, err)

	runLock, err := lockService.AcquireRunLock(ctx, lockID1, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock)
	assert.Equal(t, lockID1, runLock.LockID())

	// Test StateLock acquisition
	lockID2, err := lock.NewLockID("test-statelock-001")
	require.NoError(t, err)

	stateLock, err := lockService.AcquireStateLock(ctx, lockID2, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, stateLock)
	assert.Equal(t, lockID2, stateLock.LockID())
	assert.Equal(t, lock.LockTypeWrite, stateLock.LockType())

	// Wait for at least 2 heartbeats
	time.Sleep(250 * time.Millisecond)

	// Verify locks are still active (heartbeats working)
	foundRunLock, err := lockService.FindRunLock(ctx, lockID1)
	require.NoError(t, err)
	assert.NotNil(t, foundRunLock)

	foundStateLock, err := lockService.FindStateLock(ctx, lockID2)
	require.NoError(t, err)
	assert.NotNil(t, foundStateLock)

	// Release locks
	err = lockService.ReleaseRunLock(ctx, lockID1)
	require.NoError(t, err)

	err = lockService.ReleaseStateLock(ctx, lockID2)
	require.NoError(t, err)

	// Verify locks are released
	_, err = lockService.FindRunLock(ctx, lockID1)
	assert.Error(t, err)

	_, err = lockService.FindStateLock(ctx, lockID2)
	assert.Error(t, err)
}

func TestContainer_LockServiceCleanup(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create container with fast cleanup interval
	config := Config{
		DBPath:                dbPath,
		StorageType:           "mock",
		LockHeartbeatInterval: 10 * time.Second, // Long interval to avoid heartbeat interference
		LockCleanupInterval:   500 * time.Millisecond,
	}

	container, err := NewContainer(config)
	require.NoError(t, err)
	defer container.Close()

	lockService := container.GetLockService()
	require.NotNil(t, lockService)

	// Start Lock Service
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	// Acquire lock with very short TTL
	lockID, err := lock.NewLockID("test-expired-lock")
	require.NoError(t, err)

	runLock, err := lockService.AcquireRunLock(ctx, lockID, 1*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, runLock)

	// Stop heartbeat by releasing the lock (this stops heartbeat goroutine)
	// but then manually re-insert an expired lock directly via repository
	err = lockService.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)

	// Get direct access to repository to insert an already-expired lock
	runLockRepo := container.runLockRepo
	_, err = runLockRepo.Acquire(ctx, lockID, 100*time.Millisecond) // Very short TTL
	require.NoError(t, err)

	// Wait for lock to expire and cleanup to run (multiple cleanup cycles)
	time.Sleep(1000 * time.Millisecond)

	// Verify lock is cleaned up
	_, err = lockService.FindRunLock(ctx, lockID)
	if err == nil {
		// Lock still exists, check if it's actually expired
		foundLock, _ := lockService.FindRunLock(ctx, lockID)
		if foundLock != nil {
			t.Logf("Lock still exists. Expired: %v, ExpiresAt: %v, Now: %v",
				foundLock.IsExpired(), foundLock.ExpiresAt(), time.Now())
		}
	}
	assert.Error(t, err, "Lock should be cleaned up after expiration")

	// Should be able to acquire the same lock again
	runLock2, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock2)

	// Clean up
	err = lockService.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)
}

func TestContainer_DefaultConfiguration(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create container with minimal configuration
	config := Config{
		DBPath:      dbPath,
		StorageType: "mock",
		// Lock intervals not set - should use defaults
	}

	container, err := NewContainer(config)
	require.NoError(t, err)
	defer container.Close()

	// Verify Lock Service is initialized with defaults
	lockService := container.GetLockService()
	require.NotNil(t, lockService)

	// Start Lock Service
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	// Should work normally
	lockID, err := lock.NewLockID("test-default-config")
	require.NoError(t, err)

	runLock, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock)

	// Clean up
	err = lockService.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)
}

func TestContainer_CloseStopsLockService(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create container
	config := Config{
		DBPath:                dbPath,
		StorageType:           "mock",
		LockHeartbeatInterval: 100 * time.Millisecond,
		LockCleanupInterval:   100 * time.Millisecond,
	}

	container, err := NewContainer(config)
	require.NoError(t, err)

	lockService := container.GetLockService()
	require.NotNil(t, lockService)

	// Start Lock Service
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	// Acquire a lock
	lockID, err := lock.NewLockID("test-close")
	require.NoError(t, err)

	runLock, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock)

	// Close container (should stop Lock Service)
	err = container.Close()
	require.NoError(t, err)

	// After close, database should be closed
	// Trying to acquire new lock should fail
	lockID2, _ := lock.NewLockID("test-after-close")
	_, err = lockService.AcquireRunLock(ctx, lockID2, 5*time.Minute)
	assert.Error(t, err) // Should fail because DB is closed
}

func TestContainer_MultipleLocksSimultaneously(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := Config{
		DBPath:                dbPath,
		StorageType:           "mock",
		LockHeartbeatInterval: 100 * time.Millisecond,
		LockCleanupInterval:   200 * time.Millisecond,
	}

	container, err := NewContainer(config)
	require.NoError(t, err)
	defer container.Close()

	lockService := container.GetLockService()
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	// Acquire multiple locks
	locks := []lock.LockID{}
	for i := 0; i < 5; i++ {
		lockID, _ := lock.NewLockID(filepath.Join("test-multi", string(rune('A'+i))))
		runLock, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, runLock)
		locks = append(locks, lockID)
	}

	// List all locks
	allLocks, err := lockService.ListRunLocks(ctx)
	require.NoError(t, err)
	assert.Len(t, allLocks, 5)

	// Release all locks
	for _, lockID := range locks {
		err := lockService.ReleaseRunLock(ctx, lockID)
		require.NoError(t, err)
	}

	// Verify all released
	allLocks, err = lockService.ListRunLocks(ctx)
	require.NoError(t, err)
	assert.Len(t, allLocks, 0)
}

func TestContainer_LockConflict(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := Config{
		DBPath:      dbPath,
		StorageType: "mock",
	}

	container, err := NewContainer(config)
	require.NoError(t, err)
	defer container.Close()

	lockService := container.GetLockService()
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	// First process acquires lock
	lockID, _ := lock.NewLockID("test-conflict")
	runLock1, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock1)

	// Second process tries to acquire same lock (should fail)
	runLock2, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.Nil(t, runLock2) // Lock already held

	// Release lock
	err = lockService.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)

	// Now second process can acquire
	runLock3, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock3)

	// Clean up
	err = lockService.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)
}

// Benchmark tests
func BenchmarkLockAcquireRelease(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	config := Config{
		DBPath:      dbPath,
		StorageType: "mock",
	}

	container, err := NewContainer(config)
	require.NoError(b, err)
	defer container.Close()

	lockService := container.GetLockService()
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lockID, _ := lock.NewLockID(filepath.Join("bench", string(rune(i))))
		runLock, _ := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
		if runLock != nil {
			_ = lockService.ReleaseRunLock(ctx, lockID)
		}
	}
}
