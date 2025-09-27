package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FsyncFile syncs file contents to disk.
// This ensures that all buffered data is written to persistent storage.
// According to ARCHITECTURE.md Section 3.3, this should be followed by FsyncDir.
func FsyncFile(f *os.File) error {
	if f == nil {
		return fmt.Errorf("FsyncFile: file is nil")
	}

	// Sync file contents and metadata to disk
	if err := f.Sync(); err != nil {
		return fmt.Errorf("FsyncFile: failed to sync file %s: %w", f.Name(), err)
	}

	return nil
}

// FsyncDir syncs directory metadata to disk.
// This is crucial after rename operations to ensure directory entries are persisted.
// According to ARCHITECTURE.md Section 3.3: fsync(file) → fsync(parent dir)
func FsyncDir(dirPath string) error {
	if dirPath == "" {
		return fmt.Errorf("FsyncDir: directory path is empty")
	}

	// Open directory for sync
	dir, err := os.Open(dirPath)
	if err != nil {
		return fmt.Errorf("FsyncDir: failed to open directory %s: %w", dirPath, err)
	}
	defer dir.Close()

	// Sync directory metadata
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("FsyncDir: failed to sync directory %s: %w", dirPath, err)
	}

	return nil
}

// AtomicRename performs an atomic rename operation within the same filesystem.
// This ensures that the rename either completes fully or not at all.
// After rename, the parent directory is synced to persist the directory entry.
//
// Note: src and dst must be on the same filesystem for atomicity guarantee.
// EXDEV errors are wrapped with clear messaging for cross-filesystem attempts.
// According to ARCHITECTURE.md Section 3.3: rename requires parent dir sync.
func AtomicRename(src, dst string) error {
	if src == "" {
		return fmt.Errorf("atomic rename: source path is empty")
	}
	if dst == "" {
		return fmt.Errorf("atomic rename: destination path is empty")
	}

	// Verify source exists
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("atomic rename %s -> %s: source does not exist: %w", src, dst, err)
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(dst)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("atomic rename %s -> %s: failed to create parent dir: %w", src, dst, err)
	}

	// Perform atomic rename
	if err := os.Rename(src, dst); err != nil {
		// Check for cross-filesystem error (EXDEV)
		if os.IsExist(err) || strings.Contains(err.Error(), "cross-device") || strings.Contains(err.Error(), "invalid cross-device") {
			return fmt.Errorf("atomic rename %s -> %s: cross-filesystem rename not supported (EXDEV). Source and destination must be on the same filesystem: %w", src, dst, err)
		}
		return fmt.Errorf("atomic rename %s -> %s: %w", src, dst, err)
	}

	// Sync parent directory to ensure rename is persisted
	// This is critical for crash recovery
	if err := FsyncDir(parentDir); err != nil {
		// Rename succeeded but directory sync failed
		// This is a critical error but rename is already done
		return fmt.Errorf("atomic rename %s -> %s: rename succeeded but parent sync failed: %w", src, dst, err)
	}

	return nil
}

// WriteFileSync writes data to a file and ensures it is synced to disk.
// This is a convenience function that combines write, fsync(file), and fsync(parent dir).
// The temporary file is created in the same directory as the destination for same-FS guarantee.
// Default permission is 0644 if not specified (subject to umask).
// According to ARCHITECTURE.md Section 3.3: both file and parent dir must be synced.
func WriteFileSync(path string, data []byte, perm os.FileMode) error {
	if path == "" {
		return fmt.Errorf("write file sync: path is empty")
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("write file sync %s: failed to create parent dir: %w", path, err)
	}

	// Write to temporary file first for atomicity
	// Keep temp file in same directory to ensure same filesystem
	tempFile := filepath.Join(dir, fmt.Sprintf(".tmp.%s.%d", filepath.Base(path), os.Getpid()))

	// Create and write to temp file
	// Use provided permission or default to 0644
	if perm == 0 {
		perm = 0644
	}
	f, err := os.OpenFile(tempFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("write file sync %s: failed to create temp file: %w", path, err)
	}
	defer func() {
		f.Close()
		// Clean up temp file if it still exists (error case)
		os.Remove(tempFile)
	}()

	// Write data
	if _, err := f.Write(data); err != nil {
		return fmt.Errorf("write file sync %s: failed to write data: %w", path, err)
	}

	// Sync file contents
	if err := FsyncFile(f); err != nil {
		return fmt.Errorf("write file sync %s: failed to sync file: %w", path, err)
	}

	// Close before rename
	if err := f.Close(); err != nil {
		return fmt.Errorf("write file sync %s: failed to close file: %w", path, err)
	}

	// Atomic rename to final destination
	if err := AtomicRename(tempFile, path); err != nil {
		return fmt.Errorf("write file sync %s: %w", path, err)
	}

	return nil
}
