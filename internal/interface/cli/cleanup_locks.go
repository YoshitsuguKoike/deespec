package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
)

// CleanupLocks removes expired lock files
func CleanupLocks(paths app.Paths) error {
	Info("Checking for expired locks...\n")

	cleanedCount := 0
	errors := []error{}

	// 1. Check and clean runlock
	runlockPath := filepath.Join(paths.Var, "runlock")
	if err := cleanupSingleLock(runlockPath, "runlock"); err == nil {
		cleanedCount++
	} else if err != errLockValid && err != errLockNotExist {
		errors = append(errors, fmt.Errorf("runlock: %w", err))
	}

	// 2. Check and clean state.lock (file system lock)
	// Note: state.lock is usually automatically released by the OS when process dies
	// But we'll check it anyway
	if info, err := os.Stat(paths.StateLock); err == nil {
		// Check if the lock file is stale (older than 1 hour)
		if time.Since(info.ModTime()) > time.Hour {
			if err := os.Remove(paths.StateLock); err == nil {
				Info("Removed stale state.lock (older than 1 hour)\n")
				cleanedCount++
			} else {
				errors = append(errors, fmt.Errorf("state.lock: %w", err))
			}
		} else {
			Info("state.lock exists but is recent (modified %s)\n",
				info.ModTime().Format(time.RFC3339))
		}
	}

	// 3. Check and clean lease information in state.json
	if err := cleanupLease(paths.State); err == nil {
		cleanedCount++
	} else if err != errLeaseValid && err != errNoLease {
		errors = append(errors, fmt.Errorf("lease: %w", err))
	}

	// Report results
	if cleanedCount == 0 {
		Info("No expired locks found\n")
	} else {
		Info("Cleaned up %d expired lock(s)\n", cleanedCount)
	}

	if len(errors) > 0 {
		for _, err := range errors {
			Warn("Error during cleanup: %v\n", err)
		}
		return fmt.Errorf("cleanup completed with %d error(s)", len(errors))
	}

	return nil
}

var (
	errLockValid    = fmt.Errorf("lock is still valid")
	errLockNotExist = fmt.Errorf("lock does not exist")
	errLeaseValid   = fmt.Errorf("lease is still valid")
	errNoLease      = fmt.Errorf("no lease exists")
)

// cleanupSingleLock checks and cleans a single lock file
func cleanupSingleLock(lockPath, lockName string) error {
	// Check if lock exists
	lockInfo, err := GetLockInfo(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			Info("%s: not found\n", lockName)
			return errLockNotExist
		}
		return fmt.Errorf("failed to read %s: %w", lockName, err)
	}

	// Check if expired
	if !isLockExpired(lockInfo) {
		Info("%s: valid until %s (PID: %d)\n",
			lockName, lockInfo.ExpiresAt, lockInfo.PID)
		return errLockValid
	}

	// Lock is expired, remove it
	if err := os.Remove(lockPath); err != nil {
		return fmt.Errorf("failed to remove %s: %w", lockName, err)
	}

	Info("%s: removed (was expired at %s, PID: %d)\n",
		lockName, lockInfo.ExpiresAt, lockInfo.PID)
	return nil
}

// cleanupLease checks and cleans expired lease in state.json
func cleanupLease(statePath string) error {
	// Load state
	st, err := loadState(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			Info("lease: no state.json found\n")
			return errNoLease
		}
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check if there's a lease
	if st.LeaseExpiresAt == "" {
		Info("lease: no active lease\n")
		return errNoLease
	}

	// Check if expired
	if !LeaseExpired(st) {
		Info("lease: valid until %s (WIP: %s)\n",
			st.LeaseExpiresAt, st.WIP)
		return errLeaseValid
	}

	// Lease is expired, clear it
	prevVersion := st.Version
	st.LeaseExpiresAt = ""

	// Also clear WIP if lease expired
	if st.WIP != "" {
		Info("lease: clearing expired WIP task %s\n", st.WIP)
		st.WIP = ""
	}

	// Save updated state
	if err := saveStateCAS(statePath, st, prevVersion); err != nil {
		return fmt.Errorf("failed to save state after lease cleanup: %w", err)
	}

	Info("lease: cleared (was expired at %s)\n", st.LeaseExpiresAt)
	return nil
}

// ShowLocks displays information about all current locks
func ShowLocks(paths app.Paths) error {
	fmt.Println("=== Lock Status ===")
	fmt.Println()

	// 1. Show runlock status
	runlockPath := filepath.Join(paths.Var, "runlock")
	if lockInfo, err := GetLockInfo(runlockPath); err == nil {
		fmt.Printf("runlock:\n")
		fmt.Printf("  PID:        %d\n", lockInfo.PID)
		fmt.Printf("  Host:       %s\n", lockInfo.Hostname)
		fmt.Printf("  Acquired:   %s\n", lockInfo.AcquiredAt)
		fmt.Printf("  Expires:    %s\n", lockInfo.ExpiresAt)

		if isLockExpired(lockInfo) {
			fmt.Printf("  Status:     EXPIRED (can be cleaned)\n")
		} else {
			fmt.Printf("  Status:     VALID\n")
		}
	} else {
		fmt.Printf("runlock: not found\n")
	}
	fmt.Println()

	// 2. Show state.lock status
	if info, err := os.Stat(paths.StateLock); err == nil {
		fmt.Printf("state.lock:\n")
		fmt.Printf("  Modified:   %s\n", info.ModTime().Format(time.RFC3339))
		fmt.Printf("  Size:       %d bytes\n", info.Size())

		age := time.Since(info.ModTime())
		if age > time.Hour {
			fmt.Printf("  Status:     STALE (older than 1 hour)\n")
		} else {
			fmt.Printf("  Status:     RECENT\n")
		}
	} else {
		fmt.Printf("state.lock: not found\n")
	}
	fmt.Println()

	// 3. Show lease status
	if st, err := loadState(paths.State); err == nil {
		if st.LeaseExpiresAt != "" {
			fmt.Printf("lease:\n")
			fmt.Printf("  WIP:        %s\n", st.WIP)
			fmt.Printf("  Expires:    %s\n", st.LeaseExpiresAt)

			if LeaseExpired(st) {
				fmt.Printf("  Status:     EXPIRED (can be cleaned)\n")
			} else {
				fmt.Printf("  Status:     VALID\n")
			}
		} else {
			fmt.Printf("lease: no active lease\n")
		}
	} else {
		fmt.Printf("lease: no state.json found\n")
	}

	return nil
}
