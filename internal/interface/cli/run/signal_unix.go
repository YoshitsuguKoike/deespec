//go:build !windows
// +build !windows

package run

import (
	"os"
	"syscall"
)

// getSignalsToHandle returns the list of signals to handle on Unix systems
func getSignalsToHandle() []os.Signal {
	return []os.Signal{
		os.Interrupt,    // Ctrl+C (SIGINT)
		syscall.SIGTERM, // kill command
		syscall.SIGTSTP, // Ctrl+Z
	}
}

// isSIGTSTP checks if the signal is SIGTSTP (Ctrl+Z)
func isSIGTSTP(sig os.Signal) bool {
	return sig == syscall.SIGTSTP
}
