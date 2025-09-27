package state

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// State represents the state.json structure
type State struct {
	Version int                    `json:"version"`
	Step    string                 `json:"step"`
	Turn    int                    `json:"turn"`
	Meta    map[string]interface{} `json:"meta,omitempty"`
}

// ValidSteps defines allowed values for the step field
var ValidSteps = map[string]bool{
	"plan":      true,
	"implement": true,
	"test":      true,
	"review":    true,
	"done":      true,
}

// LoadState loads and normalizes state from the given path
func LoadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	// Parse into raw map first for normalization
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}

	// Initialize state with defaults
	state := &State{
		Version: 1,
		Step:    "plan",
		Turn:    0,
	}

	// Extract and normalize fields
	if v, ok := raw["version"].(float64); ok {
		state.Version = int(v)
	}

	if t, ok := raw["turn"].(float64); ok {
		state.Turn = int(t)
	}

	// Handle step field (prefer step over current)
	if s, ok := raw["step"].(string); ok {
		state.Step = s
	} else if c, ok := raw["current"].(string); ok {
		// Legacy support: migrate current to step
		state.Step = c
		log.Printf("INFO: Migrating 'current' field to 'step' in state.json")
	}

	// Validate step value
	if !ValidSteps[state.Step] {
		log.Printf("WARN: Invalid step value '%s' in state.json (expected: plan, implement, test, review, or done)", state.Step)
	}

	// Preserve meta if exists
	if m, ok := raw["meta"].(map[string]interface{}); ok {
		state.Meta = m
	}

	return state, nil
}
