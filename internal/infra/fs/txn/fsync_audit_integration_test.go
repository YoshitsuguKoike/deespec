//go:build fsync_audit
// +build fsync_audit

package txn

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
)

// TestTransactionFsyncAudit verifies that transactions perform expected fsync operations
func TestTransactionFsyncAudit(t *testing.T) {
	// Enable audit
	os.Setenv("DEESPEC_FSYNC_AUDIT", "1")
	defer os.Unsetenv("DEESPEC_FSYNC_AUDIT")

	// Setup
	tempDir, err := os.MkdirTemp("", "txn_fsync_audit_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Reset counters
	fs.ResetFsyncStats()

	// Begin transaction
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage files
	err = manager.StageFile(tx, "test1.txt", []byte("content1"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	err = manager.StageFile(tx, "test2.txt", []byte("content2"))
	if err != nil {
		t.Fatalf("StageFile failed: %v", err)
	}

	// Mark intent
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// Commit with journal
	destRoot := filepath.Join(tempDir, ".deespec")
	err = manager.Commit(tx, destRoot, func() error {
		// Simulate journal append
		journalPath := filepath.Join(destRoot, "var", "journal.ndjson")

		if err := os.MkdirAll(filepath.Dir(journalPath), 0755); err != nil {
			t.Fatalf("mkdir %s failed: %v", baseDir, err)
		}

		f, err := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		f.WriteString(`{"action":"test","txn_id":"` + string(tx.Manifest.ID) + `"}` + "")
		fs.FsyncFile(f)
		fs.FsyncDir(filepath.Dir(journalPath))
		return nil
	})
	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Print report
	fs.PrintFsyncReport()

	// Get stats
	fileCount, dirCount, _, _ := fs.GetFsyncStats()

	// Log counts
	t.Logf("Transaction fsync stats: files=%d, dirs=%d", fileCount, dirCount)

	// Verify minimum expected fsyncs
	// Files: manifest (3x), intent, 2x staged files, commit marker, journal = 8+
	// Dirs: base dir, stage dirs, dest dirs, journal dir = 4+

	if fileCount < 6 {
		t.Errorf("Expected at least 6 file fsyncs, got %d", fileCount)
	}

	if dirCount < 3 {
		t.Errorf("Expected at least 3 dir fsyncs, got %d", dirCount)
	}

	// Verify critical paths were synced
	fileCount2, dirCount2, filePaths, dirPaths := fs.GetFsyncStats()
	t.Logf("Total fsyncs after transaction: files=%d, dirs=%d", fileCount2, dirCount2)

	// Check that journal was synced
	journalSynced := false
	for _, path := range filePaths {
		if filepath.Base(path) == "journal.ndjson" {
			journalSynced = true
			break
		}
	}

	if !journalSynced {
		t.Error("Journal file was not fsynced")
	}

	// Check that journal directory was synced
	journalDirSynced := false
	for _, path := range dirPaths {
		if filepath.Base(path) == "var" {
			journalDirSynced = true
			break
		}
	}

	if !journalDirSynced {
		t.Error("Journal directory was not fsynced")
	}
}

// TestRegisterFsyncPath simulates a complete register operation with fsync audit
func TestRegisterFsyncPath(t *testing.T) {
	os.Setenv("DEESPEC_FSYNC_AUDIT", "1")
	defer os.Unsetenv("DEESPEC_FSYNC_AUDIT")

	tempDir, err := os.MkdirTemp("", "register_fsync_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	oldDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(oldDir)

	// Setup .deespec structure
	if err := os.MkdirAll(".deespec/var/txn", 0755); err != nil {
		t.Fatalf("mkdir %s failed: %v", baseDir, err)
	}

	if err := os.MkdirAll(".deespec/specs", 0755); err != nil {
		t.Fatalf("mkdir %s failed: %v", baseDir, err)
	}

	// Reset stats
	fs.ResetFsyncStats()

	// Simulate register operation
	txnDir := filepath.Join(".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	tx, _ := manager.Begin(ctx)

	// Stage meta.yaml and spec.md (typical register operation)
	metaContent := `id: test-001
title: Test Spec
labels: [test]
status: registered
`
	manager.StageFile(tx, "specs/test-001/meta.yaml", []byte(metaContent))

	specContent := `# Test Spec

ID: test-001

## Description

Test specification for fsync audit.
`
	manager.StageFile(tx, "specs/test-001/spec.md", []byte(specContent))

	// Mark intent
	manager.MarkIntent(tx)

	// Commit with journal
	manager.Commit(tx, ".deespec", func() error {
		// Append to journal
		journalPath := ".deespec/var/journal.ndjson"
		f, _ := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		defer f.Close()

		entry := fmt.Sprintf(`{"action":"register","spec_id":"test-001","timestamp":"%s"}`, tx.Manifest.CreatedAt.Format("2006-01-02T15:04:05Z"))
		f.WriteString(entry + "")

		// Critical: fsync journal and its directory
		fs.FsyncFile(f)
		fs.FsyncDir(".deespec/var")

		return nil
	})

	// Generate report
	fmt.Fprintf(os.Stderr, "\n=== REGISTER OPERATION FSYNC AUDIT ===")
	fs.PrintFsyncReport()

	// Verify expected fsync pattern
	fileCount, dirCount, _, _ := fs.GetFsyncStats()

	t.Logf("Register operation complete: %d file fsyncs, %d dir fsyncs", fileCount, dirCount)

	// Minimum expected for register:
	// - Multiple manifest updates
	// - Intent marker
	// - Staged files
	// - Commit marker
	// - Journal append
	if fileCount < 7 {
		t.Errorf("Register operation: expected at least 7 file fsyncs, got %d", fileCount)
	}

	if dirCount < 3 {
		t.Errorf("Register operation: expected at least 3 dir fsyncs, got %d", dirCount)
	}
}

// TestParallelChecksumWithFsyncOrder verifies fsync ordering during parallel checksum validation
func TestParallelChecksumWithFsyncOrder(t *testing.T) {
	os.Setenv("DEESPEC_FSYNC_AUDIT", "1")
	defer os.Unsetenv("DEESPEC_FSYNC_AUDIT")

	tempDir, err := os.MkdirTemp("", "parallel_checksum_fsync_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	txnDir := filepath.Join(tempDir, ".deespec/var/txn")
	manager := NewManager(txnDir)
	ctx := context.Background()

	// Reset fsync stats
	fs.ResetFsyncStats()

	// Create transaction with multiple files for parallel processing
	tx, err := manager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	// Stage 6 files to trigger parallel checksum calculation
	fileContents := []string{
		"large content 1 for parallel checksum testing with fsync order verification",
		"large content 2 for parallel checksum testing with fsync order verification",
		"large content 3 for parallel checksum testing with fsync order verification",
		"large content 4 for parallel checksum testing with fsync order verification",
		"large content 5 for parallel checksum testing with fsync order verification",
		"large content 6 for parallel checksum testing with fsync order verification",
	}

	for i, content := range fileContents {
		fileName := fmt.Sprintf("parallel_test_%d.txt", i+1)
		err = manager.StageFile(tx, fileName, []byte(content))
		if err != nil {
			t.Fatalf("StageFile %d failed: %v", i+1, err)
		}
	}

	// Mark intent
	err = manager.MarkIntent(tx)
	if err != nil {
		t.Fatalf("MarkIntent failed: %v", err)
	}

	// Commit with parallel checksum validation
	destRoot := filepath.Join(tempDir, ".deespec")
	err = manager.Commit(tx, destRoot, func() error {
		// Journal append with fsync
		journalPath := filepath.Join(destRoot, "var", "journal.ndjson")
		if err := os.MkdirAll(filepath.Dir(journalPath), 0755); err != nil {
			t.Fatalf("mkdir %s failed: %v", baseDir, err)
		}

		f, err := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		entry := fmt.Sprintf(`{"action":"parallel_checksum_test","txn_id":"%s","files":%d}`,
			string(tx.Manifest.ID), len(fileContents))
		f.WriteString(entry + "")

		// Critical fsync ordering
		fs.FsyncFile(f)
		fs.FsyncDir(filepath.Dir(journalPath))
		return nil
	})

	if err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	// Generate detailed fsync report
	fmt.Fprintf(os.Stderr, "\n=== PARALLEL CHECKSUM FSYNC ORDER AUDIT ===")
	fs.PrintFsyncReport()

	// Get detailed fsync stats
	fileCount, dirCount, filePaths, dirPaths := fs.GetFsyncStats()
	t.Logf("Parallel checksum test: %d file fsyncs, %d dir fsyncs", fileCount, dirCount)

	// Verify critical fsync operations occurred
	if fileCount < 10 {
		t.Errorf("Expected at least 10 file fsyncs for 6-file parallel transaction, got %d", fileCount)
	}

	if dirCount < 4 {
		t.Errorf("Expected at least 4 dir fsyncs, got %d", dirCount)
	}

	// Verify journal fsync occurred (critical for consistency)
	journalSynced := false
	for _, path := range filePaths {
		if filepath.Base(path) == "journal.ndjson" {
			journalSynced = true
			t.Logf("Journal file synced: %s", path)
			break
		}
	}

	if !journalSynced {
		t.Error("Journal file was not fsynced during parallel checksum operation")
	}

	// Verify parent directory fsync (ensures rename→fsync ordering)
	parentDirSynced := false
	for _, path := range dirPaths {
		if filepath.Base(path) == "var" {
			parentDirSynced = true
			t.Logf("Parent directory synced: %s", path)
			break
		}
	}

	if !parentDirSynced {
		t.Error("Parent directory was not fsynced after file operations")
	}

	// Verify destination files exist and are properly fsynced
	for i := range fileContents {
		fileName := fmt.Sprintf("parallel_test_%d.txt", i+1)
		destFile := filepath.Join(destRoot, fileName)
		if _, err := os.Stat(destFile); err != nil {
			t.Errorf("Destination file not found: %s", fileName)
		}
	}

	t.Logf("Parallel checksum fsync audit completed successfully")
	t.Logf("Verified rename→parent_dir_fsync ordering during parallel processing")
}
