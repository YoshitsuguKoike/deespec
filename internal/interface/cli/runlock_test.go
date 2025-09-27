package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireLock_Success(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "runlock")

	release, acquired, err := AcquireLock(lockPath, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if !acquired {
		t.Error("Expected to acquire lock")
	}

	if release == nil {
		t.Error("Expected non-nil release function")
	}

	// Verify lock file exists
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("Lock file should exist")
	}

	// Clean up
	if err := release(); err != nil {
		t.Error(err)
	}

	// Verify lock file is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Lock file should be removed")
	}
}

func TestAcquireLock_Contention(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "runlock")

	// Simulate first process by manually creating a valid lock file
	lockInfo := LockInfo{
		PID:        os.Getpid(), // Use current PID so it appears as valid running process
		AcquiredAt: time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:  time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
		Hostname:   "test-host",
	}

	data, err := json.Marshal(lockInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Second process tries to acquire same lock
	release2, acquired2, err := AcquireLock(lockPath, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	if acquired2 {
		t.Errorf("Second process should not acquire lock, got acquired=%v", acquired2)
	}

	if release2 != nil {
		t.Error("Second process should not get release function, but got non-nil")
		// Clean up if we got a release function unexpectedly
		release2()
	}
}

func TestAcquireLock_ExpiredLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "runlock")

	// Create an expired lock manually
	lockInfo := LockInfo{
		PID:        9999, // Non-existent PID
		AcquiredAt: time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		ExpiresAt:  time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		Hostname:   "test-host",
	}

	// Write expired lock
	data, err := json.Marshal(lockInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Try to acquire lock - should succeed because existing lock is expired
	release, acquired, err := AcquireLock(lockPath, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if release != nil {
			release()
		}
	}()

	if !acquired {
		t.Error("Should acquire lock when existing lock is expired")
	}
}

func TestLockInfo_ProcessRunningCheck(t *testing.T) {
	// Test with current process PID (should be running)
	if !isProcessRunning(os.Getpid()) {
		t.Error("Current process should be detected as running")
	}

	// Test with invalid PID
	if isProcessRunning(-1) {
		t.Error("Invalid PID should not be detected as running")
	}

	if isProcessRunning(0) {
		t.Error("PID 0 should not be detected as running")
	}
}

func TestCleanupExpiredLocks(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "runlock")

	// Create an expired lock
	lockInfo := LockInfo{
		PID:        9999,
		AcquiredAt: time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339),
		ExpiresAt:  time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339),
		Hostname:   "test-host",
	}

	data, err := json.Marshal(lockInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Clean up expired lock
	if err := CleanupExpiredLocks(lockPath); err != nil {
		t.Fatal(err)
	}

	// Verify lock is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("Expired lock should be removed")
	}
}

func TestCleanupExpiredLocks_ValidLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "runlock")

	// Create a valid lock
	lockInfo := LockInfo{
		PID:        os.Getpid(),
		AcquiredAt: time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:  time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339),
		Hostname:   "test-host",
	}

	data, err := json.Marshal(lockInfo)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Try to clean up valid lock - should do nothing
	if err := CleanupExpiredLocks(lockPath); err != nil {
		t.Fatal(err)
	}

	// Verify lock still exists
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("Valid lock should not be removed")
	}
}
