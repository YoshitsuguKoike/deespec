//go:build !windows
// +build !windows

package fs

import (
	"os"
	"syscall"
)

// flockExclusive acquires an exclusive lock on the file
func flockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// flockUnlock releases the lock on the file
func flockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
