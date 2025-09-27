//go:build windows
// +build windows

package txn

import (
	"os"
)

// checkSameDevice checks if two paths are on the same filesystem device
func checkSameDevice(s1, s2 os.FileInfo) (bool, error) {
	// Windows: Always assume same device for now
	// In production, this should check volume serial numbers
	// For now, return true to allow operations to proceed
	return true, nil
}
