package file_test

import (
	"errors"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/infra/persistence/file"
	"github.com/spf13/afero"
)

func TestWriteFileAtomic(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		data        []byte
		setupFS     func(fs afero.Fs) error
		wantErr     bool
		checkResult func(t *testing.T, fs afero.Fs, path string)
	}{
		{
			name: "Write new file successfully",
			path: "test/dir/file.txt",
			data: []byte("test content"),
			setupFS: func(fs afero.Fs) error {
				return nil
			},
			wantErr: false,
			checkResult: func(t *testing.T, fs afero.Fs, path string) {
				// Check file exists and has correct content
				content, err := afero.ReadFile(fs, path)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if string(content) != "test content" {
					t.Errorf("File content mismatch: got %q, want %q", string(content), "test content")
				}

				// Check directory was created
				info, err := fs.Stat("test/dir")
				if err != nil {
					t.Errorf("Directory not created: %v", err)
					return
				}
				if !info.IsDir() {
					t.Error("Expected directory but got file")
				}
			},
		},
		{
			name: "Overwrite existing file",
			path: "existing/file.txt",
			data: []byte("new content"),
			setupFS: func(fs afero.Fs) error {
				// Create existing file with different content
				fs.MkdirAll("existing", 0o755)
				return afero.WriteFile(fs, "existing/file.txt", []byte("old content"), 0o644)
			},
			wantErr: false,
			checkResult: func(t *testing.T, fs afero.Fs, path string) {
				content, err := afero.ReadFile(fs, path)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if string(content) != "new content" {
					t.Errorf("File not overwritten: got %q, want %q", string(content), "new content")
				}
			},
		},
		{
			name: "Write to deeply nested directory",
			path: "a/b/c/d/e/file.txt",
			data: []byte("nested"),
			setupFS: func(fs afero.Fs) error {
				return nil
			},
			wantErr: false,
			checkResult: func(t *testing.T, fs afero.Fs, path string) {
				content, err := afero.ReadFile(fs, path)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if string(content) != "nested" {
					t.Errorf("File content mismatch: got %q, want %q", string(content), "nested")
				}
			},
		},
		{
			name: "Write empty file",
			path: "empty.txt",
			data: []byte{},
			setupFS: func(fs afero.Fs) error {
				return nil
			},
			wantErr: false,
			checkResult: func(t *testing.T, fs afero.Fs, path string) {
				content, err := afero.ReadFile(fs, path)
				if err != nil {
					t.Errorf("Failed to read file: %v", err)
					return
				}
				if len(content) != 0 {
					t.Errorf("Expected empty file, got %d bytes", len(content))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use memory filesystem for testing
			fs := afero.NewMemMapFs()

			// Setup initial filesystem state
			if tt.setupFS != nil {
				if err := tt.setupFS(fs); err != nil {
					t.Fatalf("Failed to setup filesystem: %v", err)
				}
			}

			// Execute the atomic write
			err := file.WriteFileAtomic(fs, tt.path, tt.data)

			// Check error expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteFileAtomic() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check result if no error expected
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, fs, tt.path)
			}

			// Ensure no temp files are left behind
			if !tt.wantErr {
				dir := "test/dir"
				if tt.path == "empty.txt" {
					dir = "."
				} else if tt.path == "existing/file.txt" {
					dir = "existing"
				} else if tt.path == "a/b/c/d/e/file.txt" {
					dir = "a/b/c/d/e"
				}

				files, _ := afero.ReadDir(fs, dir)
				for _, file := range files {
					if len(file.Name()) > 5 && file.Name()[:5] == ".tmp-" {
						t.Errorf("Temp file not cleaned up: %s", file.Name())
					}
				}
			}
		})
	}
}

// MockFailFS is a filesystem that fails on specific operations for testing
type MockFailFS struct {
	afero.Fs
	failOnRename bool
}

func (m *MockFailFS) Rename(oldname, newname string) error {
	if m.failOnRename {
		return errors.New("rename failed")
	}
	return m.Fs.Rename(oldname, newname)
}

func TestWriteFileAtomic_RenameFailure(t *testing.T) {
	// Test that temp file is cleaned up when rename fails
	fs := &MockFailFS{
		Fs:           afero.NewMemMapFs(),
		failOnRename: true,
	}

	err := file.WriteFileAtomic(fs, "test.txt", []byte("content"))
	if err == nil {
		t.Error("Expected error when rename fails")
	}

	// Check that temp files are cleaned up
	files, _ := afero.ReadDir(fs, ".")
	for _, file := range files {
		if len(file.Name()) > 5 && file.Name()[:5] == ".tmp-" {
			t.Errorf("Temp file not cleaned up after rename failure: %s", file.Name())
		}
	}
}
