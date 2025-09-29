package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
)

func TestCleanupLocks(t *testing.T) {
	// Setup test directory
	tmpDir := t.TempDir()
	paths := app.Paths{
		Var:       filepath.Join(tmpDir, "var"),
		State:     filepath.Join(tmpDir, "var", "state.json"),
		StateLock: filepath.Join(tmpDir, "var", "state.lock"),
	}
	os.MkdirAll(paths.Var, 0755)

	t.Run("no locks exist", func(t *testing.T) {
		err := CleanupLocks(paths)
		if err != nil {
			t.Errorf("CleanupLocks failed when no locks exist: %v", err)
		}
	})

	t.Run("expired runlock cleanup", func(t *testing.T) {
		// Create expired runlock
		runlockPath := filepath.Join(paths.Var, "runlock")
		expiredLock := LockInfo{
			PID:        99999,
			Hostname:   "test-host",
			AcquiredAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			ExpiresAt:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		}
		lockData, _ := json.Marshal(expiredLock)
		os.WriteFile(runlockPath, lockData, 0644)

		err := CleanupLocks(paths)
		if err != nil {
			t.Errorf("CleanupLocks failed: %v", err)
		}

		// Check lock was removed
		if _, err := os.Stat(runlockPath); !os.IsNotExist(err) {
			t.Error("Expired runlock not removed")
		}
	})

	t.Run("valid runlock not removed", func(t *testing.T) {
		// Create valid runlock
		runlockPath := filepath.Join(paths.Var, "runlock")
		validLock := LockInfo{
			PID:        os.Getpid(),
			Hostname:   "test-host",
			AcquiredAt: time.Now().Format(time.RFC3339),
			ExpiresAt:  time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		lockData, _ := json.Marshal(validLock)
		os.WriteFile(runlockPath, lockData, 0644)

		err := CleanupLocks(paths)
		if err != nil {
			t.Errorf("CleanupLocks failed: %v", err)
		}

		// Check lock still exists
		if _, err := os.Stat(runlockPath); os.IsNotExist(err) {
			t.Error("Valid runlock was incorrectly removed")
		}

		// Cleanup
		os.Remove(runlockPath)
	})

	t.Run("stale state.lock cleanup", func(t *testing.T) {
		// Create old state.lock file
		os.WriteFile(paths.StateLock, []byte("lock"), 0644)

		// Set modification time to 2 hours ago
		oldTime := time.Now().Add(-2 * time.Hour)
		os.Chtimes(paths.StateLock, oldTime, oldTime)

		err := CleanupLocks(paths)
		if err != nil {
			t.Errorf("CleanupLocks failed: %v", err)
		}

		// Check lock was removed
		if _, err := os.Stat(paths.StateLock); !os.IsNotExist(err) {
			t.Error("Stale state.lock not removed")
		}
	})

	t.Run("recent state.lock not removed", func(t *testing.T) {
		// Create recent state.lock file
		os.WriteFile(paths.StateLock, []byte("lock"), 0644)

		err := CleanupLocks(paths)
		if err != nil {
			t.Errorf("CleanupLocks failed: %v", err)
		}

		// Check lock still exists
		if _, err := os.Stat(paths.StateLock); os.IsNotExist(err) {
			t.Error("Recent state.lock was incorrectly removed")
		}

		// Cleanup
		os.Remove(paths.StateLock)
	})

	t.Run("expired lease cleanup", func(t *testing.T) {
		// Create state with expired lease
		expiredState := &State{
			Version:        1,
			WIP:            "TEST-001",
			LeaseExpiresAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		}
		stateData, _ := json.Marshal(expiredState)
		os.WriteFile(paths.State, stateData, 0644)

		err := CleanupLocks(paths)
		if err != nil {
			t.Errorf("CleanupLocks failed: %v", err)
		}

		// Check lease was cleared
		newState, _ := loadState(paths.State)
		if newState.LeaseExpiresAt != "" {
			t.Error("Expired lease not cleared")
		}
		if newState.WIP != "" {
			t.Error("WIP not cleared when lease expired")
		}
	})

	t.Run("valid lease not removed", func(t *testing.T) {
		// Create state with valid lease
		validState := &State{
			Version:        1,
			WIP:            "TEST-002",
			LeaseExpiresAt: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		stateData, _ := json.Marshal(validState)
		os.WriteFile(paths.State, stateData, 0644)

		err := CleanupLocks(paths)
		if err != nil {
			t.Errorf("CleanupLocks failed: %v", err)
		}

		// Check lease still exists
		newState, _ := loadState(paths.State)
		if newState.LeaseExpiresAt == "" {
			t.Error("Valid lease was incorrectly removed")
		}
		if newState.WIP == "" {
			t.Error("WIP was incorrectly cleared")
		}
	})
}

func TestShowLocks(t *testing.T) {
	// Setup test directory
	tmpDir := t.TempDir()
	paths := app.Paths{
		Var:       filepath.Join(tmpDir, "var"),
		State:     filepath.Join(tmpDir, "var", "state.json"),
		StateLock: filepath.Join(tmpDir, "var", "state.lock"),
	}
	os.MkdirAll(paths.Var, 0755)

	// Create various locks
	// 1. Runlock
	runlockPath := filepath.Join(paths.Var, "runlock")
	lock := LockInfo{
		PID:        12345,
		Hostname:   "test-host",
		AcquiredAt: time.Now().Format(time.RFC3339),
		ExpiresAt:  time.Now().Add(1 * time.Hour).Format(time.RFC3339),
	}
	lockData, _ := json.Marshal(lock)
	os.WriteFile(runlockPath, lockData, 0644)

	// 2. State lock
	os.WriteFile(paths.StateLock, []byte("lock"), 0644)

	// 3. Lease
	state := &State{
		Version:        1,
		WIP:            "TEST-001",
		LeaseExpiresAt: time.Now().Add(30 * time.Minute).Format(time.RFC3339),
	}
	stateData, _ := json.Marshal(state)
	os.WriteFile(paths.State, stateData, 0644)

	// Capture output (ShowLocks prints to stdout)
	err := ShowLocks(paths)
	if err != nil {
		t.Errorf("ShowLocks failed: %v", err)
	}
}

func TestCleanupSingleLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockPath := filepath.Join(tmpDir, "test.lock")

	t.Run("lock does not exist", func(t *testing.T) {
		err := cleanupSingleLock(lockPath, "test")
		if err != errLockNotExist {
			t.Errorf("Expected errLockNotExist, got %v", err)
		}
	})

	t.Run("expired lock", func(t *testing.T) {
		expiredLock := LockInfo{
			PID:        99999,
			Hostname:   "test",
			AcquiredAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
			ExpiresAt:  time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		}
		lockData, _ := json.Marshal(expiredLock)
		os.WriteFile(lockPath, lockData, 0644)

		err := cleanupSingleLock(lockPath, "test")
		if err != nil {
			t.Errorf("Failed to cleanup expired lock: %v", err)
		}

		if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
			t.Error("Expired lock file not removed")
		}
	})

	t.Run("valid lock", func(t *testing.T) {
		validLock := LockInfo{
			PID:        os.Getpid(),
			Hostname:   "test",
			AcquiredAt: time.Now().Format(time.RFC3339),
			ExpiresAt:  time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		lockData, _ := json.Marshal(validLock)
		os.WriteFile(lockPath, lockData, 0644)

		err := cleanupSingleLock(lockPath, "test")
		if err != errLockValid {
			t.Errorf("Expected errLockValid, got %v", err)
		}

		// Lock should still exist
		if _, err := os.Stat(lockPath); os.IsNotExist(err) {
			t.Error("Valid lock was incorrectly removed")
		}

		os.Remove(lockPath)
	})
}

func TestCleanupLease(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	t.Run("no state file", func(t *testing.T) {
		err := cleanupLease(statePath)
		if err != errNoLease {
			t.Errorf("Expected errNoLease, got %v", err)
		}
	})

	t.Run("no lease in state", func(t *testing.T) {
		state := &State{
			Version: 1,
			WIP:     "TEST-001",
		}
		stateData, _ := json.Marshal(state)
		os.WriteFile(statePath, stateData, 0644)

		err := cleanupLease(statePath)
		if err != errNoLease {
			t.Errorf("Expected errNoLease, got %v", err)
		}
	})

	t.Run("valid lease", func(t *testing.T) {
		state := &State{
			Version:        1,
			WIP:            "TEST-001",
			LeaseExpiresAt: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		stateData, _ := json.Marshal(state)
		os.WriteFile(statePath, stateData, 0644)

		err := cleanupLease(statePath)
		if err != errLeaseValid {
			t.Errorf("Expected errLeaseValid, got %v", err)
		}

		// State should be unchanged
		newState, _ := loadState(statePath)
		if newState.LeaseExpiresAt == "" {
			t.Error("Valid lease was incorrectly cleared")
		}
	})

	t.Run("expired lease", func(t *testing.T) {
		state := &State{
			Version:        1,
			WIP:            "TEST-001",
			LeaseExpiresAt: time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		}
		stateData, _ := json.Marshal(state)
		os.WriteFile(statePath, stateData, 0644)

		err := cleanupLease(statePath)
		if err != nil {
			t.Errorf("Failed to cleanup expired lease: %v", err)
		}

		// Lease and WIP should be cleared
		newState, _ := loadState(statePath)
		if newState.LeaseExpiresAt != "" {
			t.Error("Expired lease not cleared")
		}
		if newState.WIP != "" {
			t.Error("WIP not cleared with expired lease")
		}
	})
}

func TestCheckNoWIP(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	t.Run("no state file", func(t *testing.T) {
		err := checkNoWIP(statePath)
		if err != nil {
			t.Errorf("Should succeed when no state file: %v", err)
		}
	})

	t.Run("state with WIP but no lease", func(t *testing.T) {
		state := &State{
			Version: 1,
			WIP:     "TASK-001",
		}
		stateData, _ := json.Marshal(state)
		os.WriteFile(statePath, stateData, 0644)

		// With the new logic, WIP without lease should succeed (with warning)
		err := checkNoWIP(statePath)
		if err != nil {
			t.Errorf("Should succeed with WIP but no lease: %v", err)
		}
	})

	t.Run("state with WIP and active lease", func(t *testing.T) {
		state := &State{
			Version:        1,
			WIP:            "TASK-001",
			LeaseExpiresAt: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		stateData, _ := json.Marshal(state)
		os.WriteFile(statePath, stateData, 0644)

		err := checkNoWIP(statePath)
		if err == nil {
			t.Error("Expected error with WIP and active lease, got nil")
		}
		if !strings.Contains(err.Error(), "active lease") {
			t.Errorf("Expected 'active lease' error, got: %v", err)
		}
	})

	t.Run("state without WIP", func(t *testing.T) {
		state := &State{
			Version: 1,
			WIP:     "",
		}
		stateData, _ := json.Marshal(state)
		os.WriteFile(statePath, stateData, 0644)

		err := checkNoWIP(statePath)
		if err != nil {
			t.Errorf("Should succeed without WIP: %v", err)
		}
	})

	t.Run("state with active lease", func(t *testing.T) {
		state := &State{
			Version:        1,
			WIP:            "",
			LeaseExpiresAt: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
		}
		stateData, _ := json.Marshal(state)
		os.WriteFile(statePath, stateData, 0644)

		err := checkNoWIP(statePath)
		if err == nil {
			t.Error("Expected error with active lease, got nil")
		}
		if !strings.Contains(err.Error(), "active lease") {
			t.Errorf("Expected 'active lease' error, got: %v", err)
		}
	})
}
