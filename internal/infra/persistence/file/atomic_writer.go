package file

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/afero"
)

// WriteFileAtomic writes data to a file atomically using temp file + rename
// This ensures that the file is either fully written or not written at all
func WriteFileAtomic(fs afero.Fs, path string, data []byte) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create temp file in the same directory to ensure atomic rename
	tmpFile, err := afero.TempFile(fs, dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on any error
	defer func() {
		// Always try to remove temp file if it still exists
		fs.Remove(tmpPath)
	}()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is flushed to disk
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close temp file before rename
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := fs.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", path, err)
	}

	return nil
}
