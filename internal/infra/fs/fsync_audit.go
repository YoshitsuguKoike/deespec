//go:build fsync_audit
// +build fsync_audit

package fs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// FsyncAudit tracks fsync calls for testing and verification
// Enabled with build tag: -tags fsync_audit
type FsyncAudit struct {
	fileCount int64
	dirCount  int64
	filePaths []string
	dirPaths  []string
	mu        sync.Mutex
	enabled   bool
}

var audit = &FsyncAudit{
	enabled: true,
}

func init() {
	// Also check environment variable
	if os.Getenv("DEESPEC_FSYNC_AUDIT") == "1" {
		audit.enabled = true
		fmt.Fprintf(os.Stderr, "INFO: Fsync audit enabled via environment\n")
	}
}

// FsyncFile synchronizes a file to disk (audit version)
func FsyncFile(file *os.File) error {
	if audit.enabled {
		atomic.AddInt64(&audit.fileCount, 1)

		audit.mu.Lock()
		audit.filePaths = append(audit.filePaths, file.Name())
		audit.mu.Unlock()

		fmt.Fprintf(os.Stderr, "AUDIT: fsync.file path=%s count=%d\n",
			file.Name(), atomic.LoadInt64(&audit.fileCount))
	}

	// Perform actual fsync
	return file.Sync()
}

// FsyncDir synchronizes a directory to disk (audit version)
func FsyncDir(path string) error {
	if audit.enabled {
		atomic.AddInt64(&audit.dirCount, 1)

		audit.mu.Lock()
		audit.dirPaths = append(audit.dirPaths, path)
		audit.mu.Unlock()

		fmt.Fprintf(os.Stderr, "AUDIT: fsync.dir path=%s count=%d\n",
			path, atomic.LoadInt64(&audit.dirCount))
	}

	// Open directory for fsync
	dir, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open dir for fsync: %w", err)
	}
	defer dir.Close()

	// Perform actual fsync
	if err := dir.Sync(); err != nil {
		// Some filesystems don't support directory fsync
		if !isNotSupportedError(err) {
			return fmt.Errorf("fsync dir: %w", err)
		}
	}

	return nil
}

// isNotSupportedError checks if error indicates unsupported operation
func isNotSupportedError(err error) bool {
	// Some filesystems (e.g., tmpfs) don't support directory fsync
	return os.IsPermission(err) || os.IsNotExist(err)
}

// GetFsyncStats returns current fsync statistics
func GetFsyncStats() (fileCount, dirCount int64, filePaths, dirPaths []string) {
	fileCount = atomic.LoadInt64(&audit.fileCount)
	dirCount = atomic.LoadInt64(&audit.dirCount)

	audit.mu.Lock()
	filePaths = append([]string{}, audit.filePaths...)
	dirPaths = append([]string{}, audit.dirPaths...)
	audit.mu.Unlock()

	return
}

// ResetFsyncStats resets the audit counters
func ResetFsyncStats() {
	atomic.StoreInt64(&audit.fileCount, 0)
	atomic.StoreInt64(&audit.dirCount, 0)

	audit.mu.Lock()
	audit.filePaths = []string{}
	audit.dirPaths = []string{}
	audit.mu.Unlock()

	fmt.Fprintf(os.Stderr, "AUDIT: fsync stats reset\n")
}

// AtomicRename performs atomic rename with audit logging
func AtomicRename(src, dst string) error {
	if audit.enabled {
		fmt.Fprintf(os.Stderr, "AUDIT: atomic.rename src=%s dst=%s\n", src, dst)
	}

	// Same implementation as non-audit version
	if err := os.Rename(src, dst); err != nil {
		return err
	}

	// Sync parent directory with audit
	return FsyncDir(filepath.Dir(dst))
}

// WriteFileSync writes file with sync and audit logging
func WriteFileSync(path string, data []byte, perm os.FileMode) error {
	if audit.enabled {
		fmt.Fprintf(os.Stderr, "AUDIT: write.file.sync path=%s size=%d\n", path, len(data))
	}

	// Create temp file
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// Write data
	if _, err := tempFile.Write(data); err != nil {
		tempFile.Close()
		return err
	}

	// Sync file
	if err := FsyncFile(tempFile); err != nil {
		tempFile.Close()
		return err
	}
	tempFile.Close()

	// Set permissions
	if err := os.Chmod(tempPath, perm); err != nil {
		return err
	}

	// Atomic rename
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}

	// Sync directory
	return FsyncDir(dir)
}

// PrintFsyncReport prints a summary of fsync calls
func PrintFsyncReport() {
	fileCount, dirCount, filePaths, dirPaths := GetFsyncStats()

	fmt.Fprintf(os.Stderr, "\n=== FSYNC AUDIT REPORT ===\n")
	fmt.Fprintf(os.Stderr, "Total file fsyncs: %d\n", fileCount)
	fmt.Fprintf(os.Stderr, "Total dir fsyncs: %d\n", dirCount)

	if len(filePaths) > 0 {
		fmt.Fprintf(os.Stderr, "\nFile fsync paths:\n")
		for i, path := range filePaths {
			fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, path)
		}
	}

	if len(dirPaths) > 0 {
		fmt.Fprintf(os.Stderr, "\nDirectory fsync paths:\n")
		for i, path := range dirPaths {
			fmt.Fprintf(os.Stderr, "  %d. %s\n", i+1, path)
		}
	}
	fmt.Fprintf(os.Stderr, "========================\n\n")
}
