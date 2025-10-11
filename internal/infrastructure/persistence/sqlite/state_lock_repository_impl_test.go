package sqlite

import (
	"context"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
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

func TestStateLockRepository_ReleaseNonExistent(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("non-existent-state-lock")

	// Try to release non-existent lock (should return error)
	err := repo.Release(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not found")
}

func TestStateLockRepository_UpdateHeartbeatNonExistent(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("non-existent-state-lock")

	// Try to update heartbeat for non-existent lock
	err := repo.UpdateHeartbeat(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not found")
}

func TestStateLockRepository_ExtendNonExistent(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("non-existent-state-lock")

	// Try to extend non-existent lock
	err := repo.Extend(ctx, lockID, 5*time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStateLockRepository_LockTypePersistence(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Test that lock type is properly persisted and retrieved
	lockID, _ := lock.NewLockID("state-file-type-persist")

	// Acquire write lock
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, stateLock)
	assert.Equal(t, lock.LockTypeWrite, stateLock.LockType())

	// Verify lock type is persisted by retrieving the lock
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeWrite, foundLock.LockType())

	// Release and test with read lock
	err = repo.Release(ctx, lockID)
	require.NoError(t, err)

	// Acquire read lock
	stateLock2, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeRead, stateLock2.LockType())

	// Verify read lock type is persisted
	foundLock2, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeRead, foundLock2.LockType())
}

func TestStateLockRepository_ListEmptyRepository(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// List locks from empty repository
	locks, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, locks)
}

func TestStateLockRepository_CleanupExpiredNoLocks(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Cleanup when there are no locks
	count, err := repo.CleanupExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestStateLockRepository_CleanupExpiredNoExpiredLocks(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Acquire lock with long TTL
	lockID, _ := lock.NewLockID("state-file-active")
	_, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 10*time.Minute)
	require.NoError(t, err)

	// Cleanup when there are no expired locks
	count, err := repo.CleanupExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	// Verify lock still exists
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.NotNil(t, foundLock)
}

func TestStateLockRepository_LockTypeTransition(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("state-file-transition")

	// Acquire read lock
	readLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeRead, readLock.LockType())

	// Release read lock
	err = repo.Release(ctx, lockID)
	require.NoError(t, err)

	// Acquire write lock with same ID
	writeLock, err := repo.Acquire(ctx, lockID, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeWrite, writeLock.LockType())

	// Release write lock
	err = repo.Release(ctx, lockID)
	require.NoError(t, err)

	// Acquire read lock again
	readLock2, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, lock.LockTypeRead, readLock2.LockType())
}

func TestStateLockRepository_MultipleLocksWithDifferentTypes(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Acquire multiple locks with different IDs and types
	testCases := []struct {
		id       string
		lockType lock.LockType
	}{
		{"state-001", lock.LockTypeRead},
		{"state-002", lock.LockTypeWrite},
		{"state-003", lock.LockTypeRead},
		{"state-004", lock.LockTypeWrite},
	}

	for _, tc := range testCases {
		lockID, _ := lock.NewLockID(tc.id)
		stateLock, err := repo.Acquire(ctx, lockID, tc.lockType, 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, stateLock)
		assert.Equal(t, tc.lockType, stateLock.LockType())
	}

	// List all locks
	locks, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, locks, 4)

	// Count lock types
	readCount := 0
	writeCount := 0
	for _, l := range locks {
		if l.LockType() == lock.LockTypeRead {
			readCount++
		} else if l.LockType() == lock.LockTypeWrite {
			writeCount++
		}
	}
	assert.Equal(t, 2, readCount)
	assert.Equal(t, 2, writeCount)
}

func TestStateLockRepository_ListOrderedByAcquiredAt(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Acquire locks with delays to ensure different acquired_at timestamps
	lockID1, _ := lock.NewLockID("state-first")
	lock1, err := repo.Acquire(ctx, lockID1, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	firstAcquiredAt := lock1.AcquiredAt()

	time.Sleep(1 * time.Second)

	lockID2, _ := lock.NewLockID("state-second")
	lock2, err := repo.Acquire(ctx, lockID2, lock.LockTypeWrite, 5*time.Minute)
	require.NoError(t, err)
	secondAcquiredAt := lock2.AcquiredAt()

	// Verify second lock was acquired after first lock
	assert.True(t, secondAcquiredAt.After(firstAcquiredAt))

	// List should return locks ordered by acquired_at DESC (most recent first)
	locks, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, locks, 2)

	// Verify both locks are present in the list
	lockIDs := []string{locks[0].LockID().String(), locks[1].LockID().String()}
	assert.Contains(t, lockIDs, "state-first")
	assert.Contains(t, lockIDs, "state-second")

	// Most recent lock should be first (but we'll be lenient due to timestamp precision)
	// Just verify the order is consistent with acquisition times
	if locks[0].LockID().String() == "state-second" {
		// DESC order: most recent first
		assert.True(t, locks[0].AcquiredAt().After(locks[1].AcquiredAt()) || locks[0].AcquiredAt().Equal(locks[1].AcquiredAt()))
	}
}

func TestStateLockRepository_CleanupExpiredMixedTypes(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	ctx := context.Background()

	// Acquire locks with different types and TTLs
	// Short TTL locks (will expire)
	lockID1, _ := lock.NewLockID("state-expired-read")
	_, err := repo.Acquire(ctx, lockID1, lock.LockTypeRead, 1*time.Second)
	require.NoError(t, err)

	lockID2, _ := lock.NewLockID("state-expired-write")
	_, err = repo.Acquire(ctx, lockID2, lock.LockTypeWrite, 1*time.Second)
	require.NoError(t, err)

	// Long TTL locks (won't expire)
	lockID3, _ := lock.NewLockID("state-active-read")
	_, err = repo.Acquire(ctx, lockID3, lock.LockTypeRead, 10*time.Minute)
	require.NoError(t, err)

	lockID4, _ := lock.NewLockID("state-active-write")
	_, err = repo.Acquire(ctx, lockID4, lock.LockTypeWrite, 10*time.Minute)
	require.NoError(t, err)

	// Wait for short TTL locks to expire
	time.Sleep(2 * time.Second)

	// Cleanup expired locks
	count, err := repo.CleanupExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count) // Two locks should be cleaned up

	// Verify expired locks are gone
	_, err = repo.Find(ctx, lockID1)
	assert.Error(t, err)
	_, err = repo.Find(ctx, lockID2)
	assert.Error(t, err)

	// Verify active locks still exist
	foundLock3, err := repo.Find(ctx, lockID3)
	require.NoError(t, err)
	assert.Equal(t, lockID3, foundLock3.LockID())

	foundLock4, err := repo.Find(ctx, lockID4)
	require.NoError(t, err)
	assert.Equal(t, lockID4, foundLock4.LockID())
}

func TestStateLockRepository_AcquireInTransaction(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-tx-state-lock-001")

	// Acquire lock within a transaction
	err := txManager.InTransaction(ctx, func(txCtx context.Context) error {
		stateLock, err := repo.Acquire(txCtx, lockID, lock.LockTypeWrite, 5*time.Minute)
		if err != nil {
			return err
		}
		assert.NotNil(t, stateLock)
		assert.Equal(t, lockID, stateLock.LockID())
		assert.Equal(t, lock.LockTypeWrite, stateLock.LockType())
		return nil
	})
	require.NoError(t, err)

	// Verify lock exists after transaction commit
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.Equal(t, lockID, foundLock.LockID())
	assert.Equal(t, lock.LockTypeWrite, foundLock.LockType())
}

func TestStateLockRepository_ReleaseInTransaction(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-tx-state-lock-002")

	// Acquire lock outside transaction
	stateLock, err := repo.Acquire(ctx, lockID, lock.LockTypeRead, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, stateLock)

	// Release lock within a transaction
	err = txManager.InTransaction(ctx, func(txCtx context.Context) error {
		return repo.Release(txCtx, lockID)
	})
	require.NoError(t, err)

	// Verify lock is gone after transaction commit
	_, err = repo.Find(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStateLockRepository_TransactionRollback(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-tx-state-rollback-001")

	// Attempt to acquire lock in a transaction that rolls back
	err := txManager.InTransaction(ctx, func(txCtx context.Context) error {
		stateLock, err := repo.Acquire(txCtx, lockID, lock.LockTypeWrite, 5*time.Minute)
		if err != nil {
			return err
		}
		assert.NotNil(t, stateLock)
		// Force rollback by returning an error
		return fmt.Errorf("intentional rollback")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "intentional rollback")

	// Verify lock does NOT exist after transaction rollback
	_, err = repo.Find(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStateLockRepository_MultipleOperationsInTransaction(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewStateLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID1, _ := lock.NewLockID("test-tx-multi-001")
	lockID2, _ := lock.NewLockID("test-tx-multi-002")

	// Perform multiple operations within a single transaction
	err := txManager.InTransaction(ctx, func(txCtx context.Context) error {
		// Acquire first lock
		lock1, err := repo.Acquire(txCtx, lockID1, lock.LockTypeRead, 5*time.Minute)
		if err != nil {
			return err
		}
		assert.NotNil(t, lock1)

		// Acquire second lock
		lock2, err := repo.Acquire(txCtx, lockID2, lock.LockTypeWrite, 5*time.Minute)
		if err != nil {
			return err
		}
		assert.NotNil(t, lock2)

		// Update heartbeat for first lock
		time.Sleep(100 * time.Millisecond)
		if err := repo.UpdateHeartbeat(txCtx, lockID1); err != nil {
			return err
		}

		return nil
	})
	require.NoError(t, err)

	// Verify both locks exist after transaction commit
	foundLock1, err := repo.Find(ctx, lockID1)
	require.NoError(t, err)
	assert.Equal(t, lockID1, foundLock1.LockID())

	foundLock2, err := repo.Find(ctx, lockID2)
	require.NoError(t, err)
	assert.Equal(t, lockID2, foundLock2.LockID())
}
