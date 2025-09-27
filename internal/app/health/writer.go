package health

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/util"
)

// Health represents the health.json structure
type Health struct {
	Ts    string `json:"ts"`
	Turn  int    `json:"turn"`
	Step  string `json:"step"`
	Ok    bool   `json:"ok"`
	Error string `json:"error"`
}

// WriteHealthAtomic writes health data atomically with current timestamp
func WriteHealthAtomic(health *Health, path string) error {
	// Update timestamp to current time with RFC3339Nano precision
	health.Ts = time.Now().UTC().Format(time.RFC3339Nano)

	// Marshal to JSON (compact form)
	data, err := json.Marshal(health)
	if err != nil {
		return fmt.Errorf("failed to marshal health: %w", err)
	}

	// Write atomically
	if err := util.WriteFileAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write health: %w", err)
	}

	return nil
}
