package app

import (
	"time"
)

// JournalEntry represents a normalized journal entry with all required fields
type JournalEntry struct {
	TS        string   `json:"ts"`
	Turn      int      `json:"turn"`
	Step      string   `json:"step"`
	Decision  string   `json:"decision"`
	ElapsedMs int64    `json:"elapsed_ms"`
	Error     string   `json:"error"`
	Artifacts []string `json:"artifacts"`
}

// NormalizeJournalEntry ensures all required fields are present in the journal entry
// It fills missing fields with zero values to maintain consistent schema
func NormalizeJournalEntry(in *JournalEntry) JournalEntry {
	e := JournalEntry{}
	if in != nil {
		e = *in
	}

	// ts - timestamp (RFC3339Nano format)
	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339Nano)
	}

	// step - default to "unknown" if empty
	if e.Step == "" {
		e.Step = "unknown"
	}

	// artifacts - ensure non-nil (empty array if nil)
	if e.Artifacts == nil {
		e.Artifacts = []string{}
	}

	// decision - normalize empty to "PENDING"
	if e.Decision == "" {
		e.Decision = "PENDING"
	}

	// Numbers and strings use zero values (0, "") which is acceptable
	// error, turn, and elapsed_ms can remain as zero values

	return e
}

// NormalizeJournalEntryMap handles normalization from a map for backward compatibility
func NormalizeJournalEntryMap(entry map[string]interface{}) map[string]interface{} {
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
	if decision, ok := entry["decision"].(string); ok && decision != "" {
		normalized["decision"] = decision
	} else {
		normalized["decision"] = "PENDING"
	}

	// elapsed_ms - elapsed time in milliseconds
	if elapsed, ok := entry["elapsed_ms"].(int); ok {
		normalized["elapsed_ms"] = elapsed
	} else if elapsed, ok := entry["elapsed_ms"].(int64); ok {
		normalized["elapsed_ms"] = int(elapsed)
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
	// IMPORTANT: Must always be an array, never null or string
	artifacts := []string{}
	if artifactsRaw, ok := entry["artifacts"]; ok {
		switch v := artifactsRaw.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					artifacts = append(artifacts, s)
				}
			}
		case []string:
			artifacts = v
		case string:
			// Convert single string to array
			if v != "" {
				artifacts = []string{v}
			}
		}
	}
	normalized["artifacts"] = artifacts

	return normalized
}

// AppendNormalizedJournal writes a normalized journal entry to the journal file
// Deprecated: Use JournalWriter.Append instead for better control and validation
func AppendNormalizedJournal(entry map[string]interface{}) error {
	writer := NewJournalWriter(".deespec/var/journal.ndjson")
	return writer.AppendMap(entry)
}