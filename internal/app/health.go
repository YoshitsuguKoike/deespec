package app

import (
	"encoding/json"
	"os"
	"time"
)

// Health represents the health status of the workflow
type Health struct {
	TS    string `json:"ts"`
	Turn  int    `json:"turn"`
	Step  string `json:"step"`
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// WriteHealth writes the health status to a JSON file
func WriteHealth(path string, turn int, step string, ok bool, errMsg string) error {
	h := Health{
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Turn:  turn,
		Step:  step,
		OK:    ok,
		Error: errMsg,
	}
	b, err := json.Marshal(h)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}