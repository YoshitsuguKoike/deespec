package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// NotesRepositoryImpl implements NotesRepository for file-based storage
type NotesRepositoryImpl struct{}

// NewNotesRepositoryImpl creates a new file-based notes repository
func NewNotesRepositoryImpl() repository.NotesRepository {
	return &NotesRepositoryImpl{}
}

// AppendNote appends a note section to the appropriate note file
func (r *NotesRepositoryImpl) AppendNote(ctx context.Context, sbiID string, kind string, section string) error {
	// Determine file path based on kind
	sbiDir := filepath.Join(".deespec", "specs", "sbi", sbiID)
	var path string
	switch kind {
	case "implement":
		path = filepath.Join(sbiDir, "impl_notes.md")
	case "review":
		path = filepath.Join(sbiDir, "review_notes.md")
	default:
		return fmt.Errorf("unknown note kind: %s", kind)
	}

	// Read existing content if file exists
	oldContent := ""
	if existingBytes, err := os.ReadFile(path); err == nil {
		oldContent = string(existingBytes)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing note file: %w", err)
	}

	// Ensure old content ends with LF if not empty
	if oldContent != "" && !strings.HasSuffix(oldContent, "\n") {
		oldContent += "\n"
	}

	// Combine old and new content
	newContent := oldContent + section

	// Ensure content ends with newline
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Atomic write
	return r.atomicWrite(path, []byte(newContent))
}

// atomicWrite writes data to a file atomically
func (r *NotesRepositoryImpl) atomicWrite(path string, data []byte) error {
	// Create temp file in the same directory as the target
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup in case of error
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close the temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", path, err)
	}

	return nil
}
