package sqlite

import (
	"context"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

func TestStateLockRepository_AcquireAndRelease(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("state-file-001")
	require.NoError(t, err)

	// Acquire write lock
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, stateLock)
	assert.Equal(t, lockID, stateLock.LockID())
	assert.Equal(t, lock.LockTypeWrite, stateLock.LockType())
	assert.Greater(t, stateLock.PID(), 0)
	assert.NotEmpty(t, stateLock.Hostname())

	// Try to acquire same lock again (should fail)
	stateLock2, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	assert.Nil(t, stateLock2) // Lock already held

	// Release lock
	err = repo.Release(ctx, lockID)
	require.NoError(t, err)

	// Acquire lock again (should succeed after release)
	stateLock3, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, stateLock3)
	assert.Equal(t, lock.LockTypeRead, stateLock3.LockType())
}

func TestStateLockRepository_Find(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("state-file-002")
	require.NoError(t, err)

	// Acquire lock
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)

	// Find lock
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.Equal(t, stateLock.LockID(), foundLock.LockID())
	assert.Equal(t, stateLock.PID(), foundLock.PID())
	assert.Equal(t, stateLock.Hostname(), foundLock.Hostname())
	assert.Equal(t, stateLock.LockType(), foundLock.LockType())
}

func TestStateLockRepository_Find_NotFound(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("non-existent-state")
	require.NoError(t, err)

	// Find non-existent lock
	_, err = repo.Find(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStateLockRepository_UpdateHeartbeat(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("state-file-003")
	require.NoError(t, err)

	// Acquire lock
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)

	initialHeartbeat := stateLock.HeartbeatAt()

	// Wait a bit to ensure timestamp difference
	time.Sleep(1 * time.Second)

	// Update heartbeat
	err = repo.UpdateHeartbeat(ctx, lockID)
	require.NoError(t, err)

	// Find lock and verify heartbeat was updated
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.True(t, foundLock.HeartbeatAt().After(initialHeartbeat))
}

func TestStateLockRepository_Extend(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("state-file-004")
	require.NoError(t, err)

	// Acquire lock with 5 minute TTL
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)

	initialExpiration := stateLock.ExpiresAt()

	// Extend lock by 10 minutes
	err = repo.Extend(ctx, lockID, 10*time.Minute)
	require.NoError(t, err)

	// Find lock and verify expiration was extended
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.True(t, foundLock.ExpiresAt().After(initialExpiration))
	assert.InDelta(t, 15*time.Minute, time.Until(foundLock.ExpiresAt()), float64(2*time.Second))
}

func TestStateLockRepository_CleanupExpired(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Acquire lock with 1 second TTL (will expire soon)
	lockID1, _ := lock.NewLockID("state-file-expired-001")
	_, err := repo.Acquire(ctx, lockID1, lock.LockTypeWrite, 1*time.Second)
	require.NoError(t, err)

	// Acquire lock with long TTL (won't expire)
	lockID2, _ := lock.NewLockID("state-file-active-001")
	_, err = repo.Acquire(ctx, lockID2, lock.LockTypeRead, 10*time.Minute)
	require.NoError(t, err)

	// Wait for first lock to expire
	time.Sleep(2 * time.Second)

	// Cleanup expired locks
	count, err := repo.CleanupExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count) // One lock should be cleaned up

	// Verify first lock is gone
	_, err = repo.Find(ctx, lockID1)
	assert.Error(t, err)

	// Verify second lock still exists
	foundLock, err := repo.Find(ctx, lockID2)
	require.NoError(t, err)
	assert.Equal(t, lockID2, foundLock.LockID())
}

func TestStateLockRepository_List(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Acquire multiple locks
	lockID1, _ := lock.NewLockID("state-file-list-001")
	_, err := repo.Acquire(ctx, lockID1, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)

	lockID2, _ := lock.NewLockID("state-file-list-002")
	_, err = repo.Acquire(ctx, lockID2, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)

	// List locks
	locks, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, locks, 2)

	// Verify lock IDs
	lockIDs := []string{locks[0].LockID().String(), locks[1].LockID().String()}
	assert.Contains(t, lockIDs, "state-file-list-001")
	assert.Contains(t, lockIDs, "state-file-list-002")

	// Verify lock types
	lockTypes := []lock.LockType{locks[0].LockType(), locks[1].LockType()}
	assert.Contains(t, lockTypes, lock.LockTypeWrite)
	assert.Contains(t, lockTypes, lock.LockTypeRead)
}

func TestStateLockRepository_AcquireExpiredLock(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("state-file-expire-001")

	// Acquire lock with 1 second TTL
	stateLock1, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 1*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, stateLock1)

	// Wait for lock to expire
	time.Sleep(2 * time.Second)

	// Acquire same lock again (should succeed because previous lock expired)
	stateLock2, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, stateLock2)
	assert.Equal(t, lock.LockTypeRead, stateLock2.LockType())
}

func TestStateLockRepository_ReadWriteLockTypes(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Test read lock
	lockID1, _ := lock.NewLockID("state-file-read")
	readLock, err := repo.Acquire(ctx, lockID1, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeRead, readLock.LockType())

	// Test write lock
	lockID2, _ := lock.NewLockID("state-file-write")
	writeLock, err := repo.Acquire(ctx, lockID2, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeWrite, writeLock.LockType())
}
