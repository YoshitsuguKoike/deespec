package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/transaction"
)

// RunLockRepositoryImpl implements repository.RunLockRepository with SQLite
type RunLockRepositoryImpl struct {
	db *sql.DB
}

// getDB returns the appropriate database executor from context
func (r *RunLockRepositoryImpl) getDB(ctx context.Context) dbExecutor {
	if tx, ok := transaction.GetTxFromContext(ctx); ok {
		return tx
	}
	return r.db
}

// NewRunLockRepository creates a new SQLite-based run lock repository
func NewRunLockRepository(db *sql.DB) repository.RunLockRepository {
	return &RunLockRepositoryImpl{db: db}
}

// Acquire attempts to acquire a run lock with atomic stale lock cleanup
func (r *RunLockRepositoryImpl) Acquire(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error) {
	db := r.getDB(ctx)
	now := time.Now().UTC()

	// Step 1: Check for existing lock and determine if it's stale
	existing, err := r.Find(ctx, lockID)
	if err == nil {
		// Lock exists - check if it's stale
		isStale := existing.IsExpired() || !isProcessRunning(existing.PID())

		if !isStale {
			// Lock is held by an active process
			return nil, nil
		}

		// Atomically delete stale lock
		// Use a simple DELETE - if another process deleted it first, that's fine
		result, _ := db.ExecContext(ctx,
			`DELETE FROM run_locks WHERE lock_id = ? AND (expires_at < ? OR pid = ?)`,
			lockID.String(),
			now.Format(time.RFC3339),
			existing.PID(),
		)

		// Check if we deleted it (1 row) or someone else did (0 rows)
		// Either way, we can proceed to insert
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
	runLock, err := lock.NewRunLock(lockID, ttl)
	if err != nil {
		return nil, fmt.Errorf("create run lock: %w", err)
	}

	// Marshal metadata
	metadataJSON, err := json.Marshal(runLock.Metadata())
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	// Step 3: Insert new lock
	// If UNIQUE constraint fails, another process acquired the lock
	insertQuery := `
		INSERT INTO run_locks (lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	_, err = db.ExecContext(ctx, insertQuery,
		runLock.LockID().String(),
		runLock.PID(),
		runLock.Hostname(),
		runLock.AcquiredAt().Format(time.RFC3339),
		runLock.ExpiresAt().Format(time.RFC3339),
		runLock.HeartbeatAt().Format(time.RFC3339),
		string(metadataJSON),
	)

	if err != nil {
		// Check if it's a UNIQUE constraint violation
		if isUniqueConstraintError(err) {
			// Another process acquired the lock first
			return nil, nil
		}
		return nil, fmt.Errorf("insert run lock: %w", err)
	}

	return runLock, nil
}

// Release releases a run lock
func (r *RunLockRepositoryImpl) Release(ctx context.Context, lockID lock.LockID) error {
	query := `DELETE FROM run_locks WHERE lock_id = ?`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query, lockID.String())
	if err != nil {
		return fmt.Errorf("delete run lock: %w", err)
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

// Find retrieves a run lock by ID
func (r *RunLockRepositoryImpl) Find(ctx context.Context, lockID lock.LockID) (*lock.RunLock, error) {
	query := `
		SELECT lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, metadata
		FROM run_locks
		WHERE lock_id = ?
	`

	db := r.getDB(ctx)
	row := db.QueryRowContext(ctx, query, lockID.String())

	var (
		lockIDStr    string
		pid          int
		hostname     string
		acquiredAt   string
		expiresAt    string
		heartbeatAt  string
		metadataJSON sql.NullString
	)

	err := row.Scan(&lockIDStr, &pid, &hostname, &acquiredAt, &expiresAt, &heartbeatAt, &metadataJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("run lock not found: %s", lockID.String())
		}
		return nil, fmt.Errorf("scan run lock: %w", err)
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

	// Unmarshal metadata
	var metadata map[string]string
	if metadataJSON.Valid && metadataJSON.String != "" {
		if err := json.Unmarshal([]byte(metadataJSON.String), &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}

	// Reconstruct lock ID
	lid, err := lock.NewLockID(lockIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid lock ID: %w", err)
	}

	return lock.ReconstructRunLock(lid, pid, hostname, acquiredAtTime, expiresAtTime, heartbeatAtTime, metadata), nil
}

// UpdateHeartbeat updates the heartbeat timestamp for a lock
func (r *RunLockRepositoryImpl) UpdateHeartbeat(ctx context.Context, lockID lock.LockID) error {
	query := `UPDATE run_locks SET heartbeat_at = ? WHERE lock_id = ?`

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
func (r *RunLockRepositoryImpl) Extend(ctx context.Context, lockID lock.LockID, duration time.Duration) error {
	// Load current lock
	runLock, err := r.Find(ctx, lockID)
	if err != nil {
		return err
	}

	// Extend expiration
	newExpiresAt := runLock.ExpiresAt().Add(duration)

	query := `UPDATE run_locks SET expires_at = ? WHERE lock_id = ?`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query, newExpiresAt.Format(time.RFC3339), lockID.String())
	if err != nil {
		return fmt.Errorf("extend lock: %w", err)
	}

	return nil
}

// CleanupExpired removes expired locks
func (r *RunLockRepositoryImpl) CleanupExpired(ctx context.Context) (int, error) {
	query := `DELETE FROM run_locks WHERE expires_at < ?`

	db := r.getDB(ctx)
	result, err := db.ExecContext(ctx, query, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("cleanup expired locks: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}

	return int(rows), nil
}

// List lists all active run locks
func (r *RunLockRepositoryImpl) List(ctx context.Context) ([]*lock.RunLock, error) {
	query := `
		SELECT lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, metadata
		FROM run_locks
		ORDER BY acquired_at DESC
	`

	db := r.getDB(ctx)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query run locks: %w", err)
	}
	defer rows.Close()

	var locks []*lock.RunLock
	for rows.Next() {
		var (
			lockIDStr    string
			pid          int
			hostname     string
			acquiredAt   string
			expiresAt    string
			heartbeatAt  string
			metadataJSON sql.NullString
		)

		if err := rows.Scan(&lockIDStr, &pid, &hostname, &acquiredAt, &expiresAt, &heartbeatAt, &metadataJSON); err != nil {
			return nil, fmt.Errorf("scan run lock: %w", err)
		}

		// Parse timestamps
		acquiredAtTime, _ := time.Parse(time.RFC3339, acquiredAt)
		expiresAtTime, _ := time.Parse(time.RFC3339, expiresAt)
		heartbeatAtTime, _ := time.Parse(time.RFC3339, heartbeatAt)

		// Unmarshal metadata
		var metadata map[string]string
		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &metadata)
		}

		// Reconstruct lock ID
		lid, _ := lock.NewLockID(lockIDStr)

		locks = append(locks, lock.ReconstructRunLock(lid, pid, hostname, acquiredAtTime, expiresAtTime, heartbeatAtTime, metadata))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate run locks: %w", err)
	}

	return locks, nil
}

// isProcessRunning checks if a process with the given PID is running
// Returns true if the process exists and is running, false otherwise
func isProcessRunning(pid int) bool {
	// Use ps command to check if process exists
	// This works on Unix-like systems (Linux, macOS)
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid))
	err := cmd.Run()
	return err == nil
}

// isUniqueConstraintError checks if the error is a UNIQUE constraint violation
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	// SQLite UNIQUE constraint error messages contain "UNIQUE constraint failed"
	return strings.Contains(err.Error(), "UNIQUE constraint failed") ||
		strings.Contains(err.Error(), "constraint failed")
}
