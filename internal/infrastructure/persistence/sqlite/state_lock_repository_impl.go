package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

// StateLockRepositoryImpl implements repository.StateLockRepository with SQLite
type StateLockRepositoryImpl struct {
	db *sql.DB
}

// getDB returns the appropriate database executor from context
func (r *StateLockRepositoryImpl) getDB(ctx context.Context) dbExecutor {
	if tx, ok := transaction.GetTxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// NewStateLockRepository creates a new SQLite-based state lock repository
func NewStateLockRepository(db *sql.DB) repository.StateLockRepository {
	return &StateLockRepositoryImpl{db: db}
}

// Acquire attempts to acquire a state lock
func (r *StateLockRepositoryImpl) Acquire(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error) {
	// Check if lock already exists and is valid
	existing, err := r.Find(ctx, lockID)
	if err == nil {
		// Lock exists - check if expired
		if !existing.IsExpired() {
			return nil, nil // Lock is held by another process
		}
		// Lock expired, remove it first
		if err := r.Release(ctx, lockID); err != nil {
			return nil, fmt.Errorf("release expired lock: %w", err)
		}
	}

	// Create new lock
	stateLock, err := lock.NewStateLock(lockID, lockType, ttl)
	if err != nil {
		return nil, fmt.Errorf("create state lock: %w", err)
	}

	// Insert lock into database
	query := `
		INSERT INTO state_locks (lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, lock_type)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query,
		stateLock.LockID().String(),
		stateLock.PID(),
		stateLock.Hostname(),
		stateLock.AcquiredAt().Format(time.RFC3339),
		stateLock.ExpiresAt().Format(time.RFC3339),
		stateLock.HeartbeatAt().Format(time.RFC3339),
		string(stateLock.LockType()),
	)
	if err != nil {
		return nil, fmt.Errorf("insert state lock: %w", err)
	}

	return stateLock, nil
}

// Release releases a state lock
func (r *StateLockRepositoryImpl) Release(ctx context.Context, lockID lock.LockID) error {
	query := `DELETE FROM state_locks WHERE lock_id = ?`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query, lockID.String())
	if err != nil {
		return fmt.Errorf("delete state lock: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lock not found: %s", lockID.String())
	}

	return nil
}

// Find retrieves a state lock by ID
func (r *StateLockRepositoryImpl) Find(ctx context.Context, lockID lock.LockID) (*lock.StateLock, error) {
	query := `
		SELECT lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, lock_type
		FROM state_locks
		WHERE lock_id = ?
	`

	db := r.getDB(ctx)
	row := db.QueryRowContext(ctx, query, lockID.String())

	var (
		lockIDStr   string
		pid         int
		hostname    string
		acquiredAt  string
		expiresAt   string
		heartbeatAt string
		lockType    string
	)

	err := row.Scan(&lockIDStr, &pid, &hostname, &acquiredAt, &expiresAt, &heartbeatAt, &lockType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("state lock not found: %s", lockID.String())
		}
		return nil, fmt.Errorf("scan state lock: %w", err)
	}

	// Parse timestamps
	acquiredAtTime, err := time.Parse(time.RFC3339, acquiredAt)
	if err != nil {
		return nil, fmt.Errorf("parse acquired_at: %w", err)
	}
	expiresAtTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("parse expires_at: %w", err)
	}
	heartbeatAtTime, err := time.Parse(time.RFC3339, heartbeatAt)
	if err != nil {
		return nil, fmt.Errorf("parse heartbeat_at: %w", err)
	}

	// Reconstruct lock ID
	lid, err := lock.NewLockID(lockIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid lock ID: %w", err)
	}

	return lock.ReconstructStateLock(lid, pid, hostname, acquiredAtTime, expiresAtTime, heartbeatAtTime, lock.LockType(lockType)), nil
}

// UpdateHeartbeat updates the heartbeat timestamp for a lock
func (r *StateLockRepositoryImpl) UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error {
	query := `UPDATE state_locks SET heartbeat_at = ? WHERE lock_id = ?`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query, time.Now().Format(time.RFC3339), lockID.String())
	if err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("lock not found: %s", lockID.String())
	}

	return nil
}

// Extend extends the expiration time of a lock
func (r *StateLockRepositoryImpl) Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	// Load current lock
	stateLock, err := r.Find(ctx, lockID)
	if err != nil {
		return err
	}

	// Extend expiration
	newExpiresAt := stateLock.ExpiresAt().Add(duration)

	query := `UPDATE state_locks SET expires_at = ? WHERE lock_id = ?`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query, newExpiresAt.Format(time.RFC3339), lockID.String())
	if err != nil {
		return fmt.Errorf("extend lock: %w", err)
	}

	return nil
}

// CleanupExpired removes expired locks
func (r *StateLockRepositoryImpl) CleanupExpired(ctx context.Context) (int, error) {
	query := `DELETE FROM state_locks WHERE expires_at < ?`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query, time.Now().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("cleanup expired locks: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return int(rows), nil
}

// List lists all active state locks
func (r *StateLockRepositoryImpl) List(ctx context.Context) ([]*lock.StateLock, error) {
	query := `
		SELECT lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, lock_type
		FROM state_locks
		ORDER BY acquired_at DESC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query state locks: %w", err)
	}
	defer rows.Close()

	var locks []*lock.StateLock
	for rows.Next() {
		var (
			lockIDStr   string
			pid         int
			hostname    string
			acquiredAt  string
			expiresAt   string
			heartbeatAt string
			lockType    string
		)

		if err := rows.Scan(&lockIDStr, &pid, &hostname, &acquiredAt, &expiresAt, &heartbeatAt, &lockType); err != nil {
			return nil, fmt.Errorf("scan state lock: %w", err)
		}

		// Parse timestamps
		acquiredAtTime, _ := time.Parse(time.RFC3339, acquiredAt)
		expiresAtTime, _ := time.Parse(time.RFC3339, expiresAt)
		heartbeatAtTime, _ := time.Parse(time.RFC3339, heartbeatAt)

		// Reconstruct lock ID
		lid, _ := lock.NewLockID(lockIDStr)

		locks = append(locks, lock.ReconstructStateLock(lid, pid, hostname, acquiredAtTime, expiresAtTime, heartbeatAtTime, lock.LockType(lockType)))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate state locks: %w", err)
	}

	return locks, nil
}
