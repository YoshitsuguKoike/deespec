package dto

import "time"

// NoteInput represents input for appending a note
type NoteInput struct {
	Kind     string    // "implement" or "review"
	Decision string    // "OK", "NEEDS_CHANGES", "PENDING", etc.
	Body     string    // The note content
	Turn     int       // The current turn number
	SBIID    string    // The SBI ID
	Now      time.Time // Timestamp for the note
}

// NoteMetadata represents metadata extracted from a note
type NoteMetadata struct {
	Summary  string
	Decision string
}
