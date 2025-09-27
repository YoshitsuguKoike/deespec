//go:build !windows
// +build !windows

package txn

import (
	"os"
	"syscall"
)

// checkSameDevice checks if two paths are on the same filesystem device
func checkSameDevice(s1, s2 os.FileInfo) (bool, error) {
	// Unix系: Stat_t.Dev を比較
	if st1, ok1 := s1.Sys().(*syscall.Stat_t); ok1 {
		if st2, ok2 := s2.Sys().(*syscall.Stat_t); ok2 {
			return st1.Dev == st2.Dev, nil
		}
	}
	// Fallback if type assertion fails
	return true, nil
}
