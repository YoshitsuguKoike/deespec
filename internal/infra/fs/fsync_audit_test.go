//go:build fsync_audit
// +build fsync_audit

package fs

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBasicFsyncAudit verifies fsync audit with basic file operations
func TestBasicFsyncAudit(t *testing.T) {
	// Enable audit via environment
	os.Setenv("DEESPEC_FSYNC_AUDIT", "1")
	defer os.Unsetenv("DEESPEC_FSYNC_AUDIT")

	// Setup test environment
	tempDir, err := os.MkdirTemp("", "fsync_audit_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Reset audit stats
	ResetFsyncStats()

	// Test WriteFileSync with audit
	testFile := filepath.Join(tempDir, "test.txt")
	err = WriteFileSync(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("WriteFileSync failed: %v", err)
	}

	// Test AtomicRename with audit
	srcFile := filepath.Join(tempDir, "src.txt")
	dstFile := filepath.Join(tempDir, "dst.txt")
	os.WriteFile(srcFile, []byte("rename test"), 0644)

	err = AtomicRename(srcFile, dstFile)
	if err != nil {
		t.Fatalf("AtomicRename failed: %v", err)
	}

	// Print audit report
	PrintFsyncReport()

	// Verify fsync counts
	fileCount, dirCount, _, _ := GetFsyncStats()

	t.Logf("File fsyncs: %d, Dir fsyncs: %d", fileCount, dirCount)

	// WriteFileSync should trigger at least 1 file fsync and 1 dir fsync
	// AtomicRename should trigger at least 1 dir fsync
	if fileCount < 1 {
		t.Errorf("Expected at least 1 file fsync, got %d", fileCount)
	}

	if dirCount < 2 {
		t.Errorf("Expected at least 2 directory fsyncs, got %d", dirCount)
	}
}

// TestFsyncAuditCounts verifies that audit counters work correctly
func TestFsyncAuditCounts(t *testing.T) {
	os.Setenv("DEESPEC_FSYNC_AUDIT", "1")
	defer os.Unsetenv("DEESPEC_FSYNC_AUDIT")

	// Reset counters
	ResetFsyncStats()

	// Create a test file
	tempFile, err := os.CreateTemp("", "fsync_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Perform fsyncs
	FsyncFile(tempFile)
	FsyncFile(tempFile)
	FsyncDir(os.TempDir())

	// Check counts
	fileCount, dirCount, _, _ := GetFsyncStats()

	if fileCount != 2 {
		t.Errorf("Expected 2 file fsyncs, got %d", fileCount)
	}

	if dirCount != 1 {
		t.Errorf("Expected 1 dir fsync, got %d", dirCount)
	}

	// Reset and verify
	ResetFsyncStats()
	fileCount, dirCount, _, _ = GetFsyncStats()

	if fileCount != 0 || dirCount != 0 {
		t.Errorf("Expected counts to be reset, got files=%d dirs=%d", fileCount, dirCount)
	}
}
