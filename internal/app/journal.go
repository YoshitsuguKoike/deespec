package app

import (
	"encoding/json"
	"os"
	"time"
)

// JournalEntry represents a normalized journal entry with all required fields
type JournalEntry struct {
	Ts         string      `json:"ts"`
	Turn       int         `json:"turn"`
	Step       string      `json:"step"`
	Decision   string      `json:"decision"`
	ElapsedMs  int         `json:"elapsed_ms"`
	Error      string      `json:"error"`
	Artifacts  interface{} `json:"artifacts"`
}

// NormalizeJournalEntry ensures all required fields are present in the journal entry
// It fills missing fields with zero values to maintain consistent schema
func NormalizeJournalEntry(entry map[string]interface{}) map[string]interface{} {
	// Ensure all required keys exist with proper defaults
	normalized := make(map[string]interface{})

	// ts - timestamp (RFC3339Nano format)
	if ts, ok := entry["ts"].(string); ok && ts != "" {
		normalized["ts"] = ts
	} else {
		normalized["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	}

	// turn - numeric turn value
	if turn, ok := entry["turn"].(int); ok {
		normalized["turn"] = turn
	} else if turn, ok := entry["turn"].(float64); ok {
		normalized["turn"] = int(turn)
	} else {
		normalized["turn"] = 0
	}

	// step - current step name
	if step, ok := entry["step"].(string); ok && step != "" {
		normalized["step"] = step
	} else {
		normalized["step"] = "unknown"
	}

	// decision - decision value (OK, NEEDS_CHANGES, etc.)
	if decision, ok := entry["decision"].(string); ok {
		normalized["decision"] = decision
	} else {
		normalized["decision"] = ""
	}

	// elapsed_ms - elapsed time in milliseconds
	if elapsed, ok := entry["elapsed_ms"].(int); ok {
		normalized["elapsed_ms"] = elapsed
	} else if elapsed, ok := entry["elapsed_ms"].(float64); ok {
		normalized["elapsed_ms"] = int(elapsed)
	} else {
		normalized["elapsed_ms"] = 0
	}

	// error - error message if any
	if err, ok := entry["error"].(string); ok {
		normalized["error"] = err
	} else {
		normalized["error"] = ""
	}

	// artifacts - normalize to array format
	if artifacts, ok := entry["artifacts"]; ok {
		switch v := artifacts.(type) {
		case []interface{}:
			normalized["artifacts"] = v
		case []string:
			// Convert []string to []interface{} for consistency
			arr := make([]interface{}, len(v))
			for i, s := range v {
				arr[i] = s
			}
			normalized["artifacts"] = arr
		case string:
			// Convert single string to array
			if v != "" {
				normalized["artifacts"] = []interface{}{v}
			} else {
				normalized["artifacts"] = []interface{}{}
			}
		default:
			normalized["artifacts"] = []interface{}{}
		}
	} else {
		normalized["artifacts"] = []interface{}{}
	}

	return normalized
}

// AppendNormalizedJournal writes a normalized journal entry to the journal file
func AppendNormalizedJournal(entry map[string]interface{}) error {
	normalized := NormalizeJournalEntry(entry)

	f, err := os.OpenFile("journal.ndjson", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	b, err := json.Marshal(normalized)
	if err != nil {
		return err
	}

	_, err = f.Write(append(b, '\n'))
	return err
}