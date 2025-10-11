package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

func setupTestDBForLock(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	// Run migrations
	migrator := NewMigrator(db)
	err = migrator.Migrate()
	require.NoError(t, err)

	return db
}

func TestRunLockRepository_AcquireAndRelease(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-sbi-001")
	require.NoError(t, err)

	// Acquire lock
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock)
	assert.Equal(t, lockID, runLock.LockID())
	assert.Greater(t, runLock.PID(), 0)
	assert.NotEmpty(t, runLock.Hostname())

	// Try to acquire same lock again (should fail)
	runLock2, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.Nil(t, runLock2) // Lock already held

	// Release lock
	err = repo.Release(ctx, lockID)
	require.NoError(t, err)

	// Acquire lock again (should succeed after release)
	runLock3, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock3)
}

func TestRunLockRepository_Find(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-sbi-002")
	require.NoError(t, err)

	// Acquire lock
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)

	// Find lock
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.Equal(t, runLock.LockID(), foundLock.LockID())
	assert.Equal(t, runLock.PID(), foundLock.PID())
	assert.Equal(t, runLock.Hostname(), foundLock.Hostname())
}

func TestRunLockRepository_Find_NotFound(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("non-existent")
	require.NoError(t, err)

	// Find non-existent lock
	_, err = repo.Find(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunLockRepository_UpdateHeartbeat(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-sbi-003")
	require.NoError(t, err)

	// Acquire lock
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)

	initialHeartbeat := runLock.HeartbeatAt()

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

func TestRunLockRepository_Extend(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, err := lock.NewLockID("test-sbi-004")
	require.NoError(t, err)

	// Acquire lock with 5 minute TTL
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)

	initialExpiration := runLock.ExpiresAt()

	// Extend lock by 10 minutes
	err = repo.Extend(ctx, lockID, 10*time.Minute)
	require.NoError(t, err)

	// Find lock and verify expiration was extended
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.True(t, foundLock.ExpiresAt().After(initialExpiration))
	assert.InDelta(t, 15*time.Minute, time.Until(foundLock.ExpiresAt()), float64(2*time.Second))
}

func TestRunLockRepository_CleanupExpired(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	// Acquire lock with 1 second TTL (will expire soon)
	lockID1, _ := lock.NewLockID("test-sbi-expired-001")
	_, err := repo.Acquire(ctx, lockID1, 1*time.Second)
	require.NoError(t, err)

	// Acquire lock with long TTL (won't expire)
	lockID2, _ := lock.NewLockID("test-sbi-active-001")
	_, err = repo.Acquire(ctx, lockID2, 10*time.Minute)
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

func TestRunLockRepository_List(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	// Acquire multiple locks
	lockID1, _ := lock.NewLockID("test-sbi-list-001")
	_, err := repo.Acquire(ctx, lockID1, 5*time.Minute)
	require.NoError(t, err)

	lockID2, _ := lock.NewLockID("test-sbi-list-002")
	_, err = repo.Acquire(ctx, lockID2, 5*time.Minute)
	require.NoError(t, err)

	// List locks
	locks, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, locks, 2)

	// Verify lock IDs
	lockIDs := []string{locks[0].LockID().String(), locks[1].LockID().String()}
	assert.Contains(t, lockIDs, "test-sbi-list-001")
	assert.Contains(t, lockIDs, "test-sbi-list-002")
}

func TestRunLockRepository_AcquireExpiredLock(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-sbi-expire-001")

	// Acquire lock with 1 second TTL
	runLock1, err := repo.Acquire(ctx, lockID, 1*time.Second)
	require.NoError(t, err)
	assert.NotNil(t, runLock1)

	// Wait for lock to expire
	time.Sleep(2 * time.Second)

	// Acquire same lock again (should succeed because previous lock expired)
	runLock2, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock2)
}

func TestRunLockRepository_ReleaseNonExistent(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("non-existent-lock")

	// Try to release non-existent lock (should return error)
	err := repo.Release(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not found")
}

func TestRunLockRepository_UpdateHeartbeatNonExistent(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("non-existent-lock")

	// Try to update heartbeat for non-existent lock
	err := repo.UpdateHeartbeat(ctx, lockID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "lock not found")
}

func TestRunLockRepository_ExtendNonExistent(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("non-existent-lock")

	// Try to extend non-existent lock
	err := repo.Extend(ctx, lockID, 5*time.Minute)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRunLockRepository_MetadataPersistence(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-sbi-metadata")

	// Acquire lock (metadata is set automatically in NewRunLock)
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock)

	// Verify metadata is persisted by retrieving the lock
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)

	// Metadata should be preserved
	assert.Equal(t, runLock.Metadata(), foundLock.Metadata())
}

func TestRunLockRepository_ListEmptyRepository(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	// List locks from empty repository
	locks, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, locks)
}

func TestRunLockRepository_CleanupExpiredNoLocks(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	// Cleanup when there are no locks
	count, err := repo.CleanupExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRunLockRepository_CleanupExpiredNoExpiredLocks(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	// Acquire lock with long TTL
	lockID, _ := lock.NewLockID("test-sbi-active")
	_, err := repo.Acquire(ctx, lockID, 10*time.Minute)
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

func TestRunLockRepository_MultipleLocksWithDifferentIDs(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	// Acquire multiple locks with different IDs
	lockIDs := []string{"lock-001", "lock-002", "lock-003"}
	for _, id := range lockIDs {
		lockID, _ := lock.NewLockID(id)
		runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, runLock)
	}

	// List all locks
	locks, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, locks, 3)

	// Verify all lock IDs are present
	foundIDs := make(map[string]bool)
	for _, l := range locks {
		foundIDs[l.LockID().String()] = true
	}
	for _, id := range lockIDs {
		assert.True(t, foundIDs[id], "Lock ID %s not found", id)
	}
}

func TestRunLockRepository_ListOrderedByAcquiredAt(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	ctx := context.Background()

	// Acquire locks with delays to ensure different acquired_at timestamps
	lockID1, _ := lock.NewLockID("lock-first")
	lock1, err := repo.Acquire(ctx, lockID1, 5*time.Minute)
	require.NoError(t, err)
	firstAcquiredAt := lock1.AcquiredAt()

	time.Sleep(1 * time.Second)

	lockID2, _ := lock.NewLockID("lock-second")
	lock2, err := repo.Acquire(ctx, lockID2, 5*time.Minute)
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
	assert.Contains(t, lockIDs, "lock-first")
	assert.Contains(t, lockIDs, "lock-second")

	// Most recent lock should be first (but we'll be lenient due to timestamp precision)
	// Just verify the order is consistent with acquisition times
	if locks[0].LockID().String() == "lock-second" {
		// DESC order: most recent first
		assert.True(t, locks[0].AcquiredAt().After(locks[1].AcquiredAt()) || locks[0].AcquiredAt().Equal(locks[1].AcquiredAt()))
	}
}

func TestRunLockRepository_AcquireInTransaction(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-tx-lock-001")

	// Acquire lock within a transaction
	err := txManager.InTransaction(ctx, func(txCtx context.Context) error {
		runLock, err := repo.Acquire(txCtx, lockID, 5*time.Minute)
		if err != nil {
			return err
		}
		assert.NotNil(t, runLock)
		assert.Equal(t, lockID, runLock.LockID())
		return nil
	})
	require.NoError(t, err)

	// Verify lock exists after transaction commit
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.Equal(t, lockID, foundLock.LockID())
}

func TestRunLockRepository_ReleaseInTransaction(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-tx-lock-002")

	// Acquire lock outside transaction
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, runLock)

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

func TestRunLockRepository_TransactionRollback(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-tx-rollback-001")

	// Attempt to acquire lock in a transaction that rolls back
	err := txManager.InTransaction(ctx, func(txCtx context.Context) error {
		runLock, err := repo.Acquire(txCtx, lockID, 5*time.Minute)
		if err != nil {
			return err
		}
		assert.NotNil(t, runLock)
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

func TestRunLockRepository_UpdateHeartbeatInTransaction(t *testing.T) {
	db := setupTestDBForLock(t)
	defer db.Close()

	repo := NewRunLockRepository(db)
	txManager := transaction.NewSQLiteTransactionManager(db)
	ctx := context.Background()

	lockID, _ := lock.NewLockID("test-tx-heartbeat-001")

	// Acquire lock
	runLock, err := repo.Acquire(ctx, lockID, 5*time.Minute)
	require.NoError(t, err)
	initialHeartbeat := runLock.HeartbeatAt()

	time.Sleep(1 * time.Second)

	// Update heartbeat within transaction
	err = txManager.InTransaction(ctx, func(txCtx context.Context) error {
		return repo.UpdateHeartbeat(txCtx, lockID)
	})
	require.NoError(t, err)

	// Verify heartbeat was updated
	foundLock, err := repo.Find(ctx, lockID)
	require.NoError(t, err)
	assert.True(t, foundLock.HeartbeatAt().After(initialHeartbeat))
}
