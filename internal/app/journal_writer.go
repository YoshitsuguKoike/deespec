package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"log"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/app/config"
)

// JournalWriter provides a unified interface for writing normalized journal entries
type JournalWriter struct {
	path string
	cfg  config.Config
}

// NewJournalWriter creates a new JournalWriter instance
func NewJournalWriter(path string) *JournalWriter {
	return &JournalWriter{path: path}
}

// NewJournalWriterWithConfig creates a new JournalWriter with config
func NewJournalWriterWithConfig(path string, cfg config.Config) *JournalWriter {
	return &JournalWriter{path: path, cfg: cfg}
}

// Append writes a normalized journal entry to the journal file
// This method ensures all required fields are present with proper types
func (w *JournalWriter) Append(entry *JournalEntry) error {
	// Normalize the entry to ensure all required fields
	e := NormalizeJournalEntry(entry)

	// Optional validation based on config
	if w.cfg != nil && w.cfg.Validate() {
		if err := validateJournal(e); err != nil {
			// MVP: Don't stop, just warn
			log.Printf("WARN: journal schema validation: %v", err)
		}
	}

	// Open file for appending
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Use buffered writer for better performance
	bw := bufio.NewWriter(f)
	defer bw.Flush()

	// Marshal to JSON
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}

	// Write JSON line
	if _, err = bw.Write(append(b, '\n')); err != nil {
		return err
	}

	// Flush buffer to file
	if err := bw.Flush(); err != nil {
		return err
	}

	// Sync file to disk for durability (ARCHITECTURE.md Section 3.3)
	// Using direct Sync() call instead of fs utilities to avoid circular dependency
	if err := f.Sync(); err != nil {
		// Log warning but don't fail - journal append is still successful
		log.Printf("WARN: failed to fsync journal: %v", err)
	}

	// Note: Parent directory sync would require opening the dir separately
	// This is deferred to future steps to avoid breaking existing tests

	return nil
}

// AppendMap handles map[string]interface{} for backward compatibility
func (w *JournalWriter) AppendMap(entry map[string]interface{}) error {
	// Convert map to struct
	normalized := NormalizeJournalEntryMap(entry)

	// Create JournalEntry from normalized map
	je := &JournalEntry{
		TS:        getStringOrDefault(normalized, "ts", ""),
		Turn:      getIntOrDefault(normalized, "turn", 0),
		Step:      getStringOrDefault(normalized, "step", "unknown"),
		Decision:  getStringOrDefault(normalized, "decision", ""),
		ElapsedMs: int64(getIntOrDefault(normalized, "elapsed_ms", 0)),
		Error:     getStringOrDefault(normalized, "error", ""),
	}

	// Handle artifacts specially to ensure []string type
	if artifacts, ok := normalized["artifacts"].([]string); ok {
		je.Artifacts = artifacts
	} else {
		je.Artifacts = []string{}
	}

	return w.Append(je)
}

// AppendEntry is an alias for AppendMap for backward compatibility
func (w *JournalWriter) AppendEntry(entry map[string]interface{}) error {
	return w.AppendMap(entry)
}

// QuickAppend is a convenience method for simple journal entries
func (w *JournalWriter) QuickAppend(turn int, step string, decision string, elapsedMs int, errMsg string, artifacts []string) error {
	entry := &JournalEntry{
		Turn:      turn,
		Step:      step,
		Decision:  decision,
		ElapsedMs: int64(elapsedMs),
		Error:     errMsg,
		Artifacts: artifacts,
	}
	return w.Append(entry)
}

// validateJournal checks if the journal entry has all required fields with correct types
func validateJournal(e JournalEntry) error {
	if e.TS == "" {
		return errors.New("ts is empty")
	}
	// Artifacts must not be nil (but can be empty slice)
	if e.Artifacts == nil {
		return errors.New("artifacts is nil")
	}
	// Step should not be empty
	if e.Step == "" {
		return errors.New("step is empty")
	}
	// Validate decision enum
	switch e.Decision {
	case "PENDING", "NEEDS_CHANGES", "OK":
		// valid enum value
	default:
		return errors.New("invalid decision: must be PENDING, NEEDS_CHANGES, or OK")
	}
	return nil
}

// Helper functions for type conversion
func getStringOrDefault(m map[string]interface{}, key string, defaultValue string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultValue
}

func getIntOrDefault(m map[string]interface{}, key string, defaultValue int) int {
	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return defaultValue
	}
}
