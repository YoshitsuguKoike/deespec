package di

import (
	"context"
	"os"
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
	// Need at least 2 cleanup cycles: 100ms (TTL) + 500ms (first cleanup) + 500ms (second cleanup) = 1100ms minimum
	time.Sleep(1500 * time.Millisecond)

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

func TestContainer_WALModeEnabled(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create container with test configuration
	config := Config{
		DBPath:      dbPath,
		StorageType: "mock",
	}

	container, err := NewContainer(config)
	require.NoError(t, err)
	defer container.Close()

	// Start Lock Service to perform database operations
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	// Perform a database operation to trigger WAL file creation
	lockService := container.GetLockService()
	lockID, err := lock.NewLockID("test-wal-mode")
	require.NoError(t, err)

	runLock, err := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, runLock)

	// Verify WAL files exist
	walPath := dbPath + "-wal"
	shmPath := dbPath + "-shm"

	// Check if WAL file exists
	_, err = os.Stat(walPath)
	assert.NoError(t, err, "WAL file (-wal) should exist when WAL mode is enabled")

	// Check if SHM file exists
	_, err = os.Stat(shmPath)
	assert.NoError(t, err, "Shared memory file (-shm) should exist when WAL mode is enabled")

	// Clean up
	err = lockService.ReleaseRunLock(ctx, lockID)
	require.NoError(t, err)
}

func TestContainer_ConcurrentAccess(t *testing.T) {
	// This test verifies that WAL mode allows concurrent read/write operations
	// simulating the scenario where `deespec run` and `deespec register` run simultaneously

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create first container (simulates `deespec run`)
	config1 := Config{
		DBPath:                dbPath,
		StorageType:           "mock",
		LockHeartbeatInterval: 100 * time.Millisecond,
		LockCleanupInterval:   200 * time.Millisecond,
	}

	container1, err := NewContainer(config1)
	require.NoError(t, err)
	defer container1.Close()

	ctx := context.Background()
	err = container1.Start(ctx)
	require.NoError(t, err)

	// Acquire a lock with container1 (simulates ongoing `run` command)
	lockService1 := container1.GetLockService()
	lockID1, err := lock.NewLockID("concurrent-test-run")
	require.NoError(t, err)

	runLock1, err := lockService1.AcquireRunLock(ctx, lockID1, 5*time.Minute)
	require.NoError(t, err)
	require.NotNil(t, runLock1)

	// Create second container (simulates `deespec register`)
	// This should succeed because WAL mode allows concurrent access
	config2 := Config{
		DBPath:                dbPath,
		StorageType:           "mock",
		LockHeartbeatInterval: 100 * time.Millisecond,
		LockCleanupInterval:   200 * time.Millisecond,
	}

	container2, err := NewContainer(config2)
	require.NoError(t, err, "Second container should initialize successfully with WAL mode")
	defer container2.Close()

	err = container2.Start(ctx)
	require.NoError(t, err, "Second container should start successfully")

	// Perform operations with container2 while container1 holds a lock
	lockService2 := container2.GetLockService()
	lockID2, err := lock.NewLockID("concurrent-test-register")
	require.NoError(t, err)

	// This should succeed because WAL mode allows multiple readers and one writer
	runLock2, err := lockService2.AcquireRunLock(ctx, lockID2, 5*time.Minute)
	require.NoError(t, err, "Second container should acquire lock successfully")
	require.NotNil(t, runLock2)

	// List locks from both containers to verify concurrent access works
	locks1, err := lockService1.ListRunLocks(ctx)
	require.NoError(t, err)
	assert.Len(t, locks1, 2, "Should see both locks from container1")

	locks2, err := lockService2.ListRunLocks(ctx)
	require.NoError(t, err)
	assert.Len(t, locks2, 2, "Should see both locks from container2")

	// Clean up locks
	err = lockService1.ReleaseRunLock(ctx, lockID1)
	require.NoError(t, err)

	err = lockService2.ReleaseRunLock(ctx, lockID2)
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

func BenchmarkConcurrentLockOperations(b *testing.B) {
	// Benchmark to measure concurrent lock operations performance with WAL mode
	tmpDir := b.TempDir()
	dbPath := filepath.Join(tmpDir, "bench.db")

	config := Config{
		DBPath:                dbPath,
		StorageType:           "mock",
		LockHeartbeatInterval: 1 * time.Second,
		LockCleanupInterval:   2 * time.Second,
	}

	container, err := NewContainer(config)
	require.NoError(b, err)
	defer container.Close()

	lockService := container.GetLockService()
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(b, err)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			lockID, _ := lock.NewLockID(filepath.Join("bench-concurrent", string(rune(i))))
			runLock, _ := lockService.AcquireRunLock(ctx, lockID, 5*time.Minute)
			if runLock != nil {
				_ = lockService.ReleaseRunLock(ctx, lockID)
			}
			i++
		}
	})
}
