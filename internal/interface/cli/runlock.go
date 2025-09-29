package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"
)

// LockInfo represents the information stored in the lock file
type LockInfo struct {
	PID        int    `json:"pid"`
	AcquiredAt string `json:"acquired_at"` // UTC RFC3339
	ExpiresAt  string `json:"expires_at"`  // UTC RFC3339
	Hostname   string `json:"hostname"`
}

// AcquireLock attempts to acquire an exclusive lock for the run
// Returns release function, whether lock was acquired, and any error
func AcquireLock(lockPath string, ttl time.Duration) (func() error, bool, error) {
	if lockPath == "" {
		lockPath = ".deespec/var/lock"
	}

	// Default TTL if not specified
	if ttl == 0 {
		ttl = 10 * time.Minute // Conservative default
	}

	now := time.Now().UTC()
	expires := now.Add(ttl)

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	lockInfo := LockInfo{
		PID:        os.Getpid(),
		AcquiredAt: now.Format(time.RFC3339),
		ExpiresAt:  expires.Format(time.RFC3339),
		Hostname:   hostname,
	}

	// Check if lock file exists and is still valid
	if existingLock, err := readLockFile(lockPath); err == nil {
		// Lock exists - check if it's expired
		if !isLockExpired(existingLock) {
			// Lock is still valid, cannot acquire
			return nil, false, nil
		}
		// Lock is expired, remove it first so we can acquire with O_EXCL
		os.Remove(lockPath)
	}

	// Try to create lock file atomically
	lockData, err := json.Marshal(lockInfo)
	if err != nil {
		return nil, false, fmt.Errorf("failed to serialize lock info: %w", err)
	}

	// Try atomic creation with O_EXCL (fails if file exists)
	// If this fails due to race condition, that's expected
	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		if os.IsExist(err) {
			// Someone else got the lock first
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to create lock file: %w", err)
	}

	// Write lock data and close
	_, writeErr := f.Write(lockData)
	closeErr := f.Close()

	if writeErr != nil {
		os.Remove(lockPath) // Cleanup on write failure
		return nil, false, fmt.Errorf("failed to write lock data: %w", writeErr)
	}
	if closeErr != nil {
		os.Remove(lockPath) // Cleanup on close failure
		return nil, false, fmt.Errorf("failed to close lock file: %w", closeErr)
	}

	// Lock acquired successfully
	releaseFunc := func() error {
		return ReleaseLock(lockPath)
	}

	return releaseFunc, true, nil
}

// ReleaseLock removes the lock file
func ReleaseLock(lockPath string) error {
	err := os.Remove(lockPath)
	if os.IsNotExist(err) {
		// Lock file doesn't exist, already released
		return nil
	}
	return err
}

// readLockFile reads and parses a lock file
func readLockFile(lockPath string) (*LockInfo, error) {
	data, err := os.ReadFile(lockPath)
	if err != nil {
		return nil, err
	}

	var lockInfo LockInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return nil, err
	}

	return &lockInfo, nil
}

// isLockExpired checks if a lock has expired
func isLockExpired(lockInfo *LockInfo) bool {
	expires, err := time.Parse(time.RFC3339, lockInfo.ExpiresAt)
	if err != nil {
		// If we can't parse the expiration time, consider it expired
		return true
	}

	// Check if the process is still running (Unix-style)
	if !isProcessRunning(lockInfo.PID) {
		return true
	}

	return time.Now().UTC().After(expires)
}

// isProcessRunning checks if a process with the given PID is still running
func isProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// On Unix systems, send signal 0 to check if process exists
	// Signal 0 doesn't actually send a signal, just checks if process exists
	err = process.Signal(syscall.Signal(0))

	// If the signal succeeded or we get EPERM, the process exists
	// EPERM means we don't have permission but the process is there
	if err == nil {
		return true
	}

	// Check for permission denied error (process exists but we can't signal it)
	if err.Error() == "operation not permitted" {
		return true
	}

	// Process doesn't exist
	return false
}

// CleanupExpiredLocks removes expired lock files (maintenance function)
func CleanupExpiredLocks(lockPath string) error {
	lockInfo, err := readLockFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No lock to clean up
		}
		return err
	}

	if isLockExpired(lockInfo) {
		return os.Remove(lockPath)
	}

	return nil // Lock is still valid
}

// GetLockInfo returns information about the current lock (for debugging)
func GetLockInfo(lockPath string) (*LockInfo, error) {
	return readLockFile(lockPath)
}
