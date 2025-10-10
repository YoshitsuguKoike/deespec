package service

import (
	"sync"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

// ConflictDetector detects file conflicts between concurrent SBI executions
// It tracks which files are being modified by which SBIs to prevent
// concurrent modifications that could cause merge conflicts or data corruption
type ConflictDetector struct {
	// activeFiles maps file paths to the SBI ID that is currently modifying them
	activeFiles map[string]string // filepath -> sbiID
	mu          sync.RWMutex
}

// NewConflictDetector creates a new conflict detector
func NewConflictDetector() *ConflictDetector {
	return &ConflictDetector{
		activeFiles: make(map[string]string),
	}
}

// HasConflict checks if the specified SBI would conflict with any currently active SBIs
// Returns true if any of the SBI's file paths are already being modified by another SBI
func (d *ConflictDetector) HasConflict(s *sbi.SBI) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	filePaths := s.Metadata().FilePaths
	sbiID := s.ID().String()

	for _, filePath := range filePaths {
		if conflictingSBIID, exists := d.activeFiles[filePath]; exists {
			// If the file is being modified by a different SBI, there's a conflict
			if conflictingSBIID != sbiID {
				return true
			}
		}
	}

	return false
}

// Register registers an SBI's file paths as active
// Should be called when an SBI execution starts
func (d *ConflictDetector) Register(s *sbi.SBI) {
	d.mu.Lock()
	defer d.mu.Unlock()

	filePaths := s.Metadata().FilePaths
	sbiID := s.ID().String()

	for _, filePath := range filePaths {
		d.activeFiles[filePath] = sbiID
	}
}

// Unregister removes an SBI's file paths from active tracking
// Should be called when an SBI execution completes
func (d *ConflictDetector) Unregister(s *sbi.SBI) {
	d.mu.Lock()
	defer d.mu.Unlock()

	filePaths := s.Metadata().FilePaths
	sbiID := s.ID().String()

	for _, filePath := range filePaths {
		// Only remove if this SBI is the one that registered it
		if registeredSBIID, exists := d.activeFiles[filePath]; exists && registeredSBIID == sbiID {
			delete(d.activeFiles, filePath)
		}
	}
}

// GetActiveFiles returns a copy of currently active file registrations
// Useful for debugging and monitoring
func (d *ConflictDetector) GetActiveFiles() map[string]string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Return a copy to prevent external modifications
	activeFilesCopy := make(map[string]string, len(d.activeFiles))
	for filePath, sbiID := range d.activeFiles {
		activeFilesCopy[filePath] = sbiID
	}

	return activeFilesCopy
}

// GetConflictingSBIID returns the SBI ID that is modifying the given file path
// Returns empty string if no SBI is modifying the file
func (d *ConflictDetector) GetConflictingSBIID(filePath string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if sbiID, exists := d.activeFiles[filePath]; exists {
		return sbiID
	}
	return ""
}

// Clear removes all registered file paths
// Useful for testing and cleanup
func (d *ConflictDetector) Clear() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.activeFiles = make(map[string]string)
}

// GetActiveFileCount returns the number of files currently being tracked
func (d *ConflictDetector) GetActiveFileCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return len(d.activeFiles)
}
