package fs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func AtomicWriteJSON(path string, v any) error {
	tmp := path + ".tmp"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
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
