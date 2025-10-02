package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
)

type State struct {
	Version        int               `json:"version"`
	Current        string            `json:"current"` // Legacy: plan/implement/test/review/done
	Status         string            `json:"status"`  // New: READY/WIP/REVIEW/REVIEW&WIP/DONE
	Turn           int               `json:"turn"`
	WIP            string            `json:"wip"`              // Work In Progress - current SBI ID (empty = no WIP)
	LeaseExpiresAt string            `json:"lease_expires_at"` // UTC RFC3339Nano (empty when no WIP)
	Inputs         map[string]string `json:"inputs"`
	LastArtifacts  map[string]string `json:"last_artifacts"`
	Decision       string            `json:"decision"` // PENDING/NEEDS_CHANGES/SUCCEEDED/FAILED
	Attempt        int               `json:"attempt"`  // Implementation attempt number (1-3)
	Meta           struct {
		UpdatedAt string `json:"updated_at"`
	} `json:"meta"`
}

func loadState(path string) (*State, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}
	return &st, nil
}

func saveStateCAS(path string, st *State, prevVersion int) error {
	if st.Version != prevVersion {
		return fmt.Errorf("version changed (expected %d, got %d)", prevVersion, st.Version)
	}
	st.Version++
	// Use local timezone with offset for better readability
	st.Meta.UpdatedAt = time.Now().Local().Format(time.RFC3339)
	return fs.AtomicWriteJSON(path, st)
}

// saveState saves the state without CAS check (simple save)
func saveState(st *State, path string) error {
	st.Version++
	st.Meta.UpdatedAt = time.Now().Local().Format(time.RFC3339)
	return fs.AtomicWriteJSON(path, st)
}

func nextStep(cur string, decision string) string {
	switch cur {
	case "plan":
		return "implement"
	case "implement":
		return "test"
	case "test":
		return "review"
	case "review":
		// Check for success decisions (SUCCEEDED or legacy OK)
		if decision == "SUCCEEDED" || decision == "OK" {
			return "done"
		}
		// For NEEDS_CHANGES or FAILED, go back to implement
		return "implement"
	case "done":
		return "done"
	default:
		return "plan"
	}
}

// nextStatusTransition determines the next status based on current status, decision, and attempt
func nextStatusTransition(currentStatus string, decision string, attempt int) string {
	// Get max attempts from config if available
	maxAttempts := 3 // Default
	if globalConfig != nil {
		maxAttempts = globalConfig.MaxAttempts()
	}

	switch currentStatus {
	case "", "READY":
		// READY -> WIP (start implementation)
		return "WIP"

	case "WIP":
		// WIP -> REVIEW (after implementation)
		return "REVIEW"

	case "REVIEW":
		if decision == "SUCCEEDED" {
			// REVIEW -> DONE (success)
			return "DONE"
		} else if attempt >= maxAttempts {
			// REVIEW -> REVIEW&WIP (force termination after max attempts)
			// This prevents infinite loops when AI keeps returning NEEDS_CHANGES or FAILED
			return "REVIEW&WIP"
		} else {
			// REVIEW -> WIP (needs changes, retry)
			return "WIP"
		}

	case "REVIEW&WIP":
		// REVIEW&WIP -> DONE (after force termination)
		return "DONE"

	case "DONE":
		// DONE -> DONE (terminal state)
		return "DONE"

	default:
		// Unknown status, default to READY
		return "READY"
	}
}
