package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
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
