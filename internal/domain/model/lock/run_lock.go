package lock

import (
	"fmt"
	"os"
	"time"
)

// RunLock represents an execution lock for SBI tasks
// It ensures only one process can execute a specific SBI at a time
type RunLock struct {
	lockID      LockID
	pid         int
	hostname    string
	acquiredAt  time.Time
	expiresAt   time.Time
	heartbeatAt time.Time
	metadata    map[string]string
}

// NewRunLock creates a new run lock
func NewRunLock(lockID LockID, ttl time.Duration) (*RunLock, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	now := time.Now().UTC()

	return &RunLock{
		lockID:      lockID,
		pid:         os.Getpid(),
		hostname:    hostname,
		acquiredAt:  now,
		expiresAt:   now.Add(ttl),
		heartbeatAt: now,
		metadata:    make(map[string]string),
	}, nil
}

// ReconstructRunLock reconstructs a RunLock from persisted data
// Used by repository when loading from database
func ReconstructRunLock(
	lockID LockID,
	pid int,
	hostname string,
	acquiredAt, expiresAt, heartbeatAt time.Time,
	metadata map[string]string,
) *RunLock {
	if metadata == nil {
		metadata = make(map[string]string)
	}
	return &RunLock{
		lockID:      lockID,
		pid:         pid,
		hostname:    hostname,
		acquiredAt:  acquiredAt,
		expiresAt:   expiresAt,
		heartbeatAt: heartbeatAt,
		metadata:    metadata,
	}
}

// IsExpired checks if the lock has expired
func (l *RunLock) IsExpired() bool {
	return time.Now().UTC().After(l.expiresAt)
}

// IsHeartbeatStale checks if the heartbeat is stale (no update for TTL duration)
func (l *RunLock) IsHeartbeatStale(maxStaleness time.Duration) bool {
	return time.Now().UTC().Sub(l.heartbeatAt) > maxStaleness
}

// UpdateHeartbeat updates the heartbeat timestamp
func (l *RunLock) UpdateHeartbeat() {
	l.heartbeatAt = time.Now().UTC()
}

// Extend extends the lock expiration time
func (l *RunLock) Extend(duration time.Duration) {
	l.expiresAt = l.expiresAt.Add(duration)
}

// SetMetadata sets a metadata value
func (l *RunLock) SetMetadata(key, value string) {
	l.metadata[key] = value
}

// GetMetadata retrieves a metadata value
func (l *RunLock) GetMetadata(key string) (string, bool) {
	value, exists := l.metadata[key]
	return value, exists
}

// Getters
func (l *RunLock) LockID() LockID               { return l.lockID }
func (l *RunLock) PID() int                     { return l.pid }
func (l *RunLock) Hostname() string             { return l.hostname }
func (l *RunLock) AcquiredAt() time.Time        { return l.acquiredAt }
func (l *RunLock) ExpiresAt() time.Time         { return l.expiresAt }
func (l *RunLock) HeartbeatAt() time.Time       { return l.heartbeatAt }
func (l *RunLock) Metadata() map[string]string  { return l.metadata }
func (l *RunLock) RemainingTime() time.Duration { return time.Until(l.expiresAt) }
