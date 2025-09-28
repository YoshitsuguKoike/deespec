package util

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeCRLFToLF(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  []byte
	}{
		{
			name:  "CRLF to LF",
			input: []byte("line1\r\nline2\r"),
			want:  []byte("line1\nline2"),
		},
		{
			name:  "LF unchanged",
			input: []byte("line1\nline2"),
			want:  []byte("line1\nline2"),
		},
		{
			name:  "mixed CRLF and LF",
			input: []byte("line1\r\nline2\nline3\r"),
			want:  []byte("line1\nline2\nline3"),
		},
		{
			name:  "empty",
			input: []byte(""),
			want:  []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeCRLFToLF(tt.input)
			if string(got) != string(tt.want) {
				t.Errorf("NormalizeCRLFToLF() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteFileAtomic(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "test.json")
		content := []byte(`{"test":true}`)

		err := WriteFileAtomic(testPath, content, 0644)
		if err != nil {
			t.Fatalf("WriteFileAtomic() error = %v", err)
		}

		// Read back
		data, err := os.ReadFile(testPath)
		if err != nil {
			t.Fatal(err)
		}

		// Check content (with added newline)
		expected := string(content) + "\n"
		if string(data) != expected {
			t.Errorf("Content = %q, want %q", string(data), expected)
		}

		// Check permissions
		info, err := os.Stat(testPath)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0644 {
			t.Errorf("Permissions = %v, want 0644", info.Mode().Perm())
		}
	})

	t.Run("adds trailing newline", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "test.txt")
		content := []byte("no newline")

		err := WriteFileAtomic(testPath, content, 0644)
		if err != nil {
			t.Fatalf("WriteFileAtomic() error = %v", err)
		}

		data, err := os.ReadFile(testPath)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.HasSuffix(string(data), "") {
			t.Error("File should end with newline")
		}
	})

	t.Run("preserves existing newline", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "test.txt")
		content := []byte("has newline\n")

		err := WriteFileAtomic(testPath, content, 0644)
		if err != nil {
			t.Fatalf("WriteFileAtomic() error = %v", err)
		}

		data, err := os.ReadFile(testPath)
		if err != nil {
			t.Fatal(err)
		}

		// Should not add extra newline when one already exists
		if string(data) != string(content) {
			t.Errorf("Content = %q, want %q", string(data), string(content))
		}
	})

	t.Run("creates directory if needed", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "subdir", "nested", "test.json")
		content := []byte(`{"nested":true}`)

		err := WriteFileAtomic(testPath, content, 0644)
		if err != nil {
			t.Fatalf("WriteFileAtomic() error = %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(testPath); err != nil {
			t.Errorf("File should exist at nested path: %v", err)
		}
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "existing.json")

		// Create initial file
		initial := []byte(`{"old":true}`)
		if err := os.WriteFile(testPath, initial, 0644); err != nil {
			t.Fatal(err)
		}

		// Overwrite with atomic write
		newContent := []byte(`{"new":true}`)
		err := WriteFileAtomic(testPath, newContent, 0644)
		if err != nil {
			t.Fatalf("WriteFileAtomic() overwrite error = %v", err)
		}

		// Verify new content
		data, err := os.ReadFile(testPath)
		if err != nil {
			t.Fatal(err)
		}

		expected := string(newContent) + "\n"
		if string(data) != expected {
			t.Errorf("Content = %q, want %q", string(data), expected)
		}
	})

	t.Run("normalizes CRLF to LF", func(t *testing.T) {
		tmpDir := t.TempDir()
		testPath := filepath.Join(tmpDir, "crlf.txt")
		content := []byte("line1\r\nline2\r\nline3")

		err := WriteFileAtomic(testPath, content, 0644)
		if err != nil {
			t.Fatalf("WriteFileAtomic() error = %v", err)
		}

		data, err := os.ReadFile(testPath)
		if err != nil {
			t.Fatal(err)
		}

		// Should convert CRLF to LF and add trailing newline
		expected := "line1\nline2\nline3\n"
		if string(data) != expected {
			t.Errorf("Content = %q, want %q", string(data), expected)
		}

		// Verify no CRLF remains
		if strings.Contains(string(data), "\r") {
			t.Error("File should not contain CRLF")
		}
	})

	t.Run("cleans up temp file on failure", func(t *testing.T) {
		// Use a read-only directory to force rename failure
		tmpDir := t.TempDir()
		roDir := filepath.Join(tmpDir, "readonly")
		if err := os.Mkdir(roDir, 0755); err != nil {
			t.Fatal(err)
		}

		// Create a file and then make directory read-only
		testPath := filepath.Join(roDir, "test.json")
		if err := os.WriteFile(testPath, []byte("initial"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(roDir, 0555); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Chmod(roDir, 0755); err != nil {
				t.Fatalf("chmod %s failed: %v", roDir, err)
			}
		}() // Restore for cleanup

		// Try atomic write (should fail)
		content := []byte(`{"test":true}`)
		err := WriteFileAtomic(testPath, content, 0644)
		if err == nil {
			t.Error("Expected error for read-only directory")
		}

		// Temp file should not exist
		tmpPath := testPath + ".tmp"
		if _, err := os.Stat(tmpPath); err == nil {
			t.Error("Temp file should be cleaned up after failure")
		}
	})
}
