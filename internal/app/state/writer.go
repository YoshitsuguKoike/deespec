package state

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/util"
)

// SaveStateAtomic saves state atomically with updated timestamp
func SaveStateAtomic(state *State, path string) error {
	// Ensure meta exists
	if state.Meta == nil {
		state.Meta = make(map[string]interface{})
	}

	// Update timestamp
	state.Meta["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)

	// Marshal to JSON (compact form)
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically
	if err := util.WriteFileAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state: %w", err)
	}

	return nil
}