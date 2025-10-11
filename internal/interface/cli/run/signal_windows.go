//go:build windows
// +build windows

package run

import (
	"os"
	"syscall"
)

// getSignalsToHandle returns the list of signals to handle on Windows
// Note: Windows doesn't support SIGTSTP (Ctrl+Z), so we only handle SIGINT and SIGTERM
func getSignalsToHandle() []os.Signal {
	return []os.Signal{
		os.Interrupt,    // Ctrl+C (SIGINT)
		syscall.SIGTERM, // kill command
	}
}

// isSIGTSTP checks if the signal is SIGTSTP (Ctrl+Z)
// Always returns false on Windows as SIGTSTP is not supported
func isSIGTSTP(sig os.Signal) bool {
	return false
}
