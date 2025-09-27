package fs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
