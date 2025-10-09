package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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

// Acquire attempts to acquire a run lock
func (r *RunLockRepositoryImpl) Acquire(ctx context.Context, lockID lock.LockID, ttl time.Duration) (*lock.RunLock, error) {
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
	runLock, err := lock.NewRunLock(lockID, ttl)
	if err != nil {
		return nil, fmt.Errorf("create run lock: %w", err)
	}

	// Marshal metadata
	metadataJSON, err := json.Marshal(runLock.Metadata())
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}

	// Insert lock into database
	query := `
		INSERT INTO run_locks (lock_id, pid, hostname, acquired_at, expires_at, heartbeat_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	db := r.getDB(ctx)
	_, err = db.ExecContext(ctx, query,
		runLock.LockID().String(),
		runLock.PID(),
		runLock.Hostname(),
		runLock.AcquiredAt().Format(time.RFC3339),
		runLock.ExpiresAt().Format(time.RFC3339),
		runLock.HeartbeatAt().Format(time.RFC3339),
		string(metadataJSON),
	)
	if err != nil {
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
