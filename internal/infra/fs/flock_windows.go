//go:build windows
// +build windows

package fs

import (
	"os"
)

// flockExclusive acquires an exclusive lock on the file
// Note: Windows doesn't have direct flock support, so this is a no-op for now
// TODO: Implement Windows file locking using LockFileEx
func flockExclusive(f *os.File) error {
	// No-op on Windows for now
	// In production, this should use Windows API LockFileEx
	return nil
}

// flockUnlock releases the lock on the file
// Note: Windows doesn't have direct flock support, so this is a no-op for now
func flockUnlock(f *os.File) error {
	// No-op on Windows for now
	// In production, this should use Windows API UnlockFileEx
	return nil
}
