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
		os.MkdirAll(filepath.Dir(journalPath), 0755)

		f, err := os.OpenFile(journalPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		defer f.Close()

		f.WriteString(`{"action":"test","txn_id":"` + string(tx.Manifest.ID) + `"}` + "\n")
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
	os.MkdirAll(".deespec/var/txn", 0755)
	os.MkdirAll(".deespec/specs", 0755)

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
		f.WriteString(entry + "\n")

		// Critical: fsync journal and its directory
		fs.FsyncFile(f)
		fs.FsyncDir(".deespec/var")

		return nil
	})

	// Generate report
	fmt.Fprintf(os.Stderr, "\n=== REGISTER OPERATION FSYNC AUDIT ===\n")
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
