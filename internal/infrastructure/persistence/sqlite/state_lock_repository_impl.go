package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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

// Acquire attempts to acquire a state lock with atomic stale lock cleanup
func (r *StateLockRepositoryImpl) Acquire(ctx context.Context, lockID lock.LockID, lockType lock.LockType, ttl time.Duration) (*lock.StateLock, error) {
	db := r.getDB(ctx)
	now := time.Now().UTC()

	// Step 1: Check for existing lock and determine if it's stale
	existing, err := r.Find(ctx, lockID)
	if err == nil {
		// Lock exists - check if it's stale
		isStale := existing.IsExpired() || !isStateLockProcessRunning(existing.PID())

		if !isStale {
			// Lock is held by an active process
			return nil, nil
		}

		// Atomically delete stale lock
		// Use a simple DELETE - if another process deleted it first, that's fine
		result, _ := db.ExecContext(ctx,
			`DELETE FROM state_locks WHERE lock_id = ? AND (expires_at < ? OR pid = ?)`,
			lockID.String(),
			now.Format(time.RFC3339Nano),
			existing.PID(),
		)

		// Check if we deleted it (1 row) or someone else did (0 rows)
		if result != nil {
			rows, _ := result.RowsAffected()
			if rows == 0 {
				// Another process deleted it - verify it's really gone before inserting
				if stillExists, _ := r.Find(ctx, lockID); stillExists != nil {
					// Lock was recreated by another process
					return nil, nil
				}
			}
		}
	}

	// Step 2: Create new lock
	stateLock, err := lock.NewStateLock(lockID, lockType, ttl)
	if err != nil {
		return nil, fmt.Errorf("create state lock: %w", err)
	}

	// Step 3: Insert new lock
	// If UNIQUE constraint fails, another process acquired the lock
	insertQuery := `
		INSERT INTO state_locks (lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, lock_type)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.ExecContext(ctx, insertQuery,
		stateLock.LockID().String(),
		stateLock.PID(),
		stateLock.Hostname(),
		stateLock.AcquiredAt().Format(time.RFC3339Nano),
		stateLock.ExpiresAt().Format(time.RFC3339Nano),
		stateLock.HeartbeatAt().Format(time.RFC3339Nano),
		string(stateLock.LockType()),
	)

	if err != nil {
		// Check if it's a UNIQUE constraint violation
		if isStateLockUniqueConstraintError(err) {
			// Another process acquired the lock first
			return nil, nil
		}
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

	// Parse timestamps - try RFC3339Nano first, fall back to RFC3339 for backward compatibility
	acquiredAtTime, err := time.Parse(time.RFC3339Nano, acquiredAt)
	if err != nil {
		acquiredAtTime, err = time.Parse(time.RFC3339, acquiredAt)
		if err != nil {
			return nil, fmt.Errorf("parse acquired_at: %w", err)
		}
	}
	expiresAtTime, err := time.Parse(time.RFC3339Nano, expiresAt)
	if err != nil {
		expiresAtTime, err = time.Parse(time.RFC3339, expiresAt)
		if err != nil {
			return nil, fmt.Errorf("parse expires_at: %w", err)
		}
	}
	heartbeatAtTime, err := time.Parse(time.RFC3339Nano, heartbeatAt)
	if err != nil {
		heartbeatAtTime, err = time.Parse(time.RFC3339, heartbeatAt)
		if err != nil {
			return nil, fmt.Errorf("parse heartbeat_at: %w", err)
		}
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
	result, err := db.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339Nano), lockID.String())
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
	_, err = db.ExecContext(ctx, query, newExpiresAt.Format(time.RFC3339Nano), lockID.String())
	if err != nil {
		return fmt.Errorf("extend lock: %w", err)
	}

	return nil
}

// CleanupExpired removes expired locks
func (r *StateLockRepositoryImpl) CleanupExpired(ctx context.Context) (int, error) {
	query := `DELETE FROM state_locks WHERE expires_at < ?`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339Nano))
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

		// Parse timestamps - try RFC3339Nano first, fall back to RFC3339 for backward compatibility
		acquiredAtTime, err := time.Parse(time.RFC3339Nano, acquiredAt)
		if err != nil {
			acquiredAtTime, _ = time.Parse(time.RFC3339, acquiredAt)
		}
		expiresAtTime, err := time.Parse(time.RFC3339Nano, expiresAt)
		if err != nil {
			expiresAtTime, _ = time.Parse(time.RFC3339, expiresAt)
		}
		heartbeatAtTime, err := time.Parse(time.RFC3339Nano, heartbeatAt)
		if err != nil {
			heartbeatAtTime, _ = time.Parse(time.RFC3339, heartbeatAt)
		}

		// Reconstruct lock ID
		lid, _ := lock.NewLockID(lockIDStr)

		locks = append(locks, lock.ReconstructStateLock(lid, pid, hostname, acquiredAtTime, expiresAtTime, heartbeatAtTime, lock.LockType(lockType)))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate state locks: %w", err)
	}

	return locks, nil
}

// isStateLockProcessRunning checks if a process with the given PID is running
// Returns true if the process exists and is running, false otherwise
func isStateLockProcessRunning(pid int) bool {
	// Use ps command to check if process exists
	// This works on Unix-like systems (Linux, macOS)
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid))
	err := cmd.Run()
	return err == nil
}

// isStateLockUniqueConstraintError checks if the error is a UNIQUE constraint violation
func isStateLockUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// SQLite UNIQUE constraint error messages contain "UNIQUE constraint failed"
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint failed")
}
