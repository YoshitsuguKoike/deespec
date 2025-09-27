package util

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to a file atomically using temp file + rename
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	// Normalize line endings: CRLF -> LF
	data = NormalizeCRLFToLF(data)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Add newline if missing (for proper POSIX text file)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	// Write to temp file first
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, perm); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up on failure
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// NormalizeCRLFToLF converts CRLF line endings to LF
func NormalizeCRLFToLF(data []byte) []byte {
	// Replace all CRLF with LF
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}
