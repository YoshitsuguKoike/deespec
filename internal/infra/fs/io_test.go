package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFsyncFile(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test-fsync-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write data
	testData := []byte("test data for fsync")
	if _, err := tmpFile.Write(testData); err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	// Test FsyncFile
	if err := FsyncFile(tmpFile); err != nil {
		t.Errorf("FsyncFile failed: %v", err)
	}

	// Test with nil file
	if err := FsyncFile(nil); err == nil {
		t.Error("FsyncFile should fail with nil file")
	}
}

func TestFsyncDir(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-fsync-dir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test FsyncDir
	if err := FsyncDir(tmpDir); err != nil {
		t.Errorf("FsyncDir failed: %v", err)
	}

	// Test with empty path
	if err := FsyncDir(""); err == nil {
		t.Error("FsyncDir should fail with empty path")
	}

	// Test with non-existent directory
	nonExistentDir := filepath.Join(tmpDir, "non-existent")
	if err := FsyncDir(nonExistentDir); err == nil {
		t.Error("FsyncDir should fail with non-existent directory")
	}
}

func TestAtomicRename(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-atomic-rename-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	dstPath := filepath.Join(tmpDir, "destination.txt")
	testData := []byte("test content for rename")

	if err := os.WriteFile(srcPath, testData, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Test AtomicRename
	if err := AtomicRename(srcPath, dstPath); err != nil {
		t.Errorf("AtomicRename failed: %v", err)
	}

	// Verify source is gone
	if _, err := os.Stat(srcPath); !os.IsNotExist(err) {
		t.Error("Source file should not exist after rename")
	}

	// Verify destination exists with correct content
	if content, err := os.ReadFile(dstPath); err != nil {
		t.Errorf("Failed to read destination file: %v", err)
	} else if string(content) != string(testData) {
		t.Errorf("Destination content mismatch: got %s, want %s", content, testData)
	}

	// Test with empty paths
	if err := AtomicRename("", dstPath); err == nil {
		t.Error("AtomicRename should fail with empty source")
	}
	if err := AtomicRename(dstPath, ""); err == nil {
		t.Error("AtomicRename should fail with empty destination")
	}

	// Test with non-existent source
	if err := AtomicRename("/non/existent/file", dstPath); err == nil {
		t.Error("AtomicRename should fail with non-existent source")
	}
}

func TestWriteFileSync(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "test-write-sync-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test WriteFileSync
	testPath := filepath.Join(tmpDir, "test-file.txt")
	testData := []byte("synchronized write test data")

	if err := WriteFileSync(testPath, testData, 0644); err != nil {
		t.Errorf("WriteFileSync failed: %v", err)
	}

	// Verify file exists with correct content
	if content, err := os.ReadFile(testPath); err != nil {
		t.Errorf("Failed to read written file: %v", err)
	} else if string(content) != string(testData) {
		t.Errorf("Content mismatch: got %s, want %s", content, testData)
	}

	// Verify permissions
	if info, err := os.Stat(testPath); err != nil {
		t.Errorf("Failed to stat file: %v", err)
	} else if info.Mode().Perm() != 0644 {
		t.Errorf("Permission mismatch: got %v, want 0644", info.Mode().Perm())
	}

	// Test overwrite
	newData := []byte("overwritten data")
	if err := WriteFileSync(testPath, newData, 0644); err != nil {
		t.Errorf("WriteFileSync overwrite failed: %v", err)
	}

	// Verify overwritten content
	if content, err := os.ReadFile(testPath); err != nil {
		t.Errorf("Failed to read overwritten file: %v", err)
	} else if string(content) != string(newData) {
		t.Errorf("Overwrite content mismatch: got %s, want %s", content, newData)
	}

	// Test with empty path
	if err := WriteFileSync("", testData, 0644); err == nil {
		t.Error("WriteFileSync should fail with empty path")
	}

	// Verify no temp files left
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Errorf("Failed to read directory: %v", err)
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".tmp" {
			t.Errorf("Temp file left behind: %s", entry.Name())
		}
	}
}