package common

import (
	"fmt"
	"path/filepath"
)

// State represents the legacy state.json structure (deprecated)
// This is kept as a stub for backward compatibility with old commands
// New code should use DB-based state management instead
type State struct {
	Version        string                 `json:"version"`
	WIP            string                 `json:"wip"`
	Status         string                 `json:"status"`
	Current        string                 `json:"current"`
	Turn           int                    `json:"turn"`
	Attempt        int                    `json:"attempt"`
	Decision       string                 `json:"decision"`
	LeaseExpiresAt string                 `json:"lease_expires_at"`
	Inputs         map[string]interface{} `json:"inputs,omitempty"`
	LastArtifacts  []string               `json:"last_artifacts,omitempty"`
	Meta           map[string]interface{} `json:"meta,omitempty"`
}

// GetFileName extracts the filename from a path for cleaner output
func GetFileName(filePath string) string {
	if filePath == ".deespec/var/state.json" {
		return "state.json"
	}
	if filePath == ".deespec/var/health.json" {
		return "health.json"
	}
	return filepath.Base(filePath)
}

// LoadState loads state from state.json (deprecated - returns empty state)
// This is kept as a stub for backward compatibility
// New code should use DB-based state management instead
func LoadState(path string) (*State, error) {
	Warn("LoadState is deprecated - DB-based state management should be used instead")
	return &State{
		Version: "0.1.14",
		Meta:    make(map[string]interface{}),
		Inputs:  make(map[string]interface{}),
	}, nil
}

// SaveStateCAS saves state to state.json with CAS (deprecated - no-op)
// This is kept as a stub for backward compatibility
// New code should use DB-based state management instead
func SaveStateCAS(path string, state *State, expectedSerial int) error {
	Warn("SaveStateCAS is deprecated - DB-based state management should be used instead")
	return fmt.Errorf("state.json is no longer supported - use DB-based state management")
}

// LeaseExpired checks if the lease has expired (deprecated - always returns false)
// This is kept as a stub for backward compatibility
// New code should use lock service for lease management
func LeaseExpired(state *State) bool {
	Warn("LeaseExpired is deprecated - use lock service instead")
	return false
}
