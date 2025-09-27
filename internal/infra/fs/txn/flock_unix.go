//go:build !windows
// +build !windows

package txn

import (
	"os"
	"syscall"
)

// flockExclusive acquires an exclusive lock on the file
func flockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// flockShared acquires a shared lock on the file
func flockShared(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH)
}

// flockUnlock releases the lock on the file
func flockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}