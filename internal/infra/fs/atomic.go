package fs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// AtomicWriteJSON writes JSON atomically with fsync(file) and fsync(parent dir),
// using a unique temporary file in the same directory to avoid collisions.
func AtomicWriteJSON(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	// Create unique temp file in same directory
	tf, err := os.CreateTemp(dir, ".tmp.*")
	if err != nil {
		return fmt.Errorf("atomic json: create temp: %w", err)
	}
	tmp := tf.Name()
	// Ensure cleanup in error cases (harmless if renamed)
	defer os.Remove(tmp)

	// Set desired permission (CreateTemp defaults 0600)
	if err := os.Chmod(tmp, 0o644); err != nil {
		tf.Close()
		return fmt.Errorf("atomic json: chmod temp: %w", err)
	}

	if _, err := tf.Write(data); err != nil {
		tf.Close()
		return fmt.Errorf("atomic json: write temp: %w", err)
	}
	if err := FsyncFile(tf); err != nil {
		tf.Close()
		return fmt.Errorf("atomic json: fsync temp: %w", err)
	}
	if err := tf.Close(); err != nil {
		return fmt.Errorf("atomic json: close temp: %w", err)
	}

	// Atomic rename to destination and fsync parent dir
	if err := AtomicRename(tmp, path); err != nil {
		return err
	}
	return nil
}

// すでに存在したら失敗する簡易ロック（多重実行防止）
func AcquireLock(lockPath string) (release func() error, err error) {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("another process is running (lock): %w", err)
	}
	if _, _ = f.Write([]byte("locked")); f != nil {
		_ = f.Close()
	}
	return func() error { return os.Remove(lockPath) }, nil
}

// AppendNDJSONLine appends a single JSON line to an NDJSON file with file locking.
// This function ensures safe concurrent writes by using flock(2) for exclusive locking.
// The JSON object is marshaled to a single line (no indentation) and appended with a newline.
//
// Key features:
// - File locking (flock LOCK_EX) prevents concurrent write corruption
// - Atomic append operation (O_APPEND ensures atomic writes on POSIX systems)
// - fsync guarantees durability before returning
// - Automatic directory creation if needed
//
// Error handling:
// - Returns error if JSON marshaling fails
// - Returns error if file operations fail
// - Lock is always released even if write fails
func AppendNDJSONLine(path string, record interface{}) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("append ndjson: failed to create directory: %w", err)
	}

	// Open file in append mode (create if not exists)
	// O_APPEND ensures atomic writes at end of file on POSIX systems
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("append ndjson: failed to open file: %w", err)
	}
	defer f.Close()

	// Acquire exclusive file lock using flock
	// This prevents concurrent writes from corrupting the file
	// LOCK_EX = exclusive lock (no other process can hold any lock)
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("append ndjson: failed to acquire file lock: %w", err)
	}
	// Release lock when function returns (defer ensures this happens)
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)

	// Marshal record to JSON (single line, no indentation for NDJSON)
	jsonBytes, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("append ndjson: failed to marshal record: %w", err)
	}

	// Append newline to create NDJSON format (one JSON object per line)
	line := append(jsonBytes, '\n')

	// Write the complete line atomically
	// O_APPEND flag ensures this write goes to end of file atomically
	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("append ndjson: failed to write line: %w", err)
	}

	// fsync ensures data is written to persistent storage
	// This guarantees durability even if system crashes after this call
	if err := FsyncFile(f); err != nil {
		return fmt.Errorf("append ndjson: failed to sync file: %w", err)
	}

	return nil
}
