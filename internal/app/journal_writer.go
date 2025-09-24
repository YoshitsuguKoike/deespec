package app

import (
	"bufio"
	"encoding/json"
	"os"
	"time"
)

// JournalWriter provides a unified interface for writing normalized journal entries
type JournalWriter struct {
	path string
}

// NewJournalWriter creates a new JournalWriter instance
func NewJournalWriter(path string) *JournalWriter {
	return &JournalWriter{path: path}
}

// AppendEntry writes a normalized journal entry to the journal file
// This method ensures all required fields are present with proper types
func (w *JournalWriter) AppendEntry(entry map[string]interface{}) error {
	// Normalize the entry to ensure all required fields
	normalized := NormalizeJournalEntry(entry)

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
	b, err := json.Marshal(normalized)
	if err != nil {
		return err
	}

	// Write JSON line
	if _, err := bw.Write(append(b, '\n')); err != nil {
		return err
	}

	return nil
}

// QuickAppend is a convenience method for simple journal entries
func (w *JournalWriter) QuickAppend(turn int, step string, decision string, elapsedMs int, errMsg string, artifacts []string) error {
	entry := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       turn,
		"step":       step,
		"decision":   decision,
		"elapsed_ms": elapsedMs,
		"error":      errMsg,
		"artifacts":  artifacts,
	}
	return w.AppendEntry(entry)
}