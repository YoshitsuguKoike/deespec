package repository

import "context"

// NotesRepository defines operations for note persistence
type NotesRepository interface {
	// AppendNote appends a note section to the appropriate note file
	AppendNote(ctx context.Context, sbiID string, kind string, section string) error
}
