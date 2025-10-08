package repository

import (
	"context"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

// RunLockRepository manages RunLock persistence
type RunLockRepository interface {
	// Acquire attempts to acquire a run lock
	// Returns the lock if successful, nil if lock is held by another process
	Acquire(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error)

	// Release releases a run lock
	Release(ctx context.Context, lockID lock.LockID) error

	// Find retrieves a run lock by ID
	Find(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error)

	// UpdateHeartbeat updates the heartbeat timestamp for a lock
	UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error

	// Extend extends the expiration time of a lock
	Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error

	// CleanupExpired removes expired locks
	CleanupExpired(ctx context.Context) (int, error)

	// List lists all active run locks
	List(ctx context.Context) ([]*lock.RunLock, error)
}

// StateLockRepository manages StateLock persistence
type StateLockRepository interface {
	// Acquire attempts to acquire a state lock
	// Returns the lock if successful, nil if lock is held by another process
	Acquire(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error)

	// Release releases a state lock
	Release(ctx context.Context, lockID lock.LockID) error

	// Find retrieves a state lock by ID
	Find(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error)

	// UpdateHeartbeat updates the heartbeat timestamp for a lock
	UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error

	// Extend extends the expiration time of a lock
	Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error

	// CleanupExpired removes expired locks
	CleanupExpired(ctx context.Context) (int, error)

	// List lists all active state locks
	List(ctx context.Context) ([]*lock.StateLock, error)
}
