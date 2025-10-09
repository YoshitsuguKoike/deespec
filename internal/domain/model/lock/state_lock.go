package lock

import (
	"fmt"
	"os"
	"time"
)

// LockType represents the type of state lock
type LockType string

const (
	LockTypeRead  LockType = "read"
	LockTypeWrite LockType = "write"
)

// StateLock represents a lock for state file access
// It supports both read and write locks for state file operations
type StateLock struct {
	lockID      LockID
	pid         int
	hostname    string
	acquiredAt  time.Time
	expiresAt   time.Time
	heartbeatAt time.Time
	lockType    LockType
}

// NewStateLock creates a new state lock
func NewStateLock(lockID LockID, lockType LockType, ttl time.Duration) (*StateLock, error) {
	if lockType != LockTypeRead && lockType != LockTypeWrite {
		return nil, fmt.Errorf("invalid lock type: %s", lockType)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	now := time.Now().UTC()

	return &StateLock{
		lockID:      lockID,
		pid:         os.Getpid(),
		hostname:    hostname,
		acquiredAt:  now,
		expiresAt:   now.Add(ttl),
		heartbeatAt: now,
		lockType:    lockType,
	}, nil
}

// ReconstructStateLock reconstructs a StateLock from persisted data
// Used by repository when loading from database
func ReconstructStateLock(
	lockID LockID,
	pid int,
	hostname string,
	acquiredAt, expiresAt, heartbeatAt time.Time,
	lockType LockType,
) *StateLock {
	return &StateLock{
		lockID:      lockID,
		pid:         pid,
		hostname:    hostname,
		acquiredAt:  acquiredAt,
		expiresAt:   expiresAt,
		heartbeatAt: heartbeatAt,
		lockType:    lockType,
	}
}

// IsExpired checks if the lock has expired
func (l *StateLock) IsExpired() bool {
	return time.Now().UTC().After(l.expiresAt)
}

// IsHeartbeatStale checks if the heartbeat is stale
func (l *StateLock) IsHeartbeatStale(maxStaleness time.Duration) bool {
	return time.Now().UTC().Sub(l.heartbeatAt) > maxStaleness
}

// UpdateHeartbeat updates the heartbeat timestamp
func (l *StateLock) UpdateHeartbeat() {
	l.heartbeatAt = time.Now().UTC()
}

// Extend extends the lock expiration time
func (l *StateLock) Extend(duration time.Duration) {
	l.expiresAt = l.expiresAt.Add(duration)
}

// Getters
func (l *StateLock) LockID() LockID               { return l.lockID }
func (l *StateLock) PID() int                     { return l.pid }
func (l *StateLock) Hostname() string             { return l.hostname }
func (l *StateLock) AcquiredAt() time.Time        { return l.acquiredAt }
func (l *StateLock) ExpiresAt() time.Time         { return l.expiresAt }
func (l *StateLock) HeartbeatAt() time.Time       { return l.heartbeatAt }
func (l *StateLock) LockType() LockType           { return l.lockType }
func (l *StateLock) RemainingTime() time.Duration { return time.Until(l.expiresAt) }
