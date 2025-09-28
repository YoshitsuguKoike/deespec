package sbi

import (
	"errors"
)

// SBI represents a specification entity in the domain layer
type SBI struct {
	ID    string // Format: SBI-<ULID>
	Title string // Required, cannot be empty
	Body  string // The full content including guidelines and body text
}

// NewSBI creates a new SBI entity with validation
func NewSBI(id, title, body string) (*SBI, error) {
	if title == "" {
		return nil, errors.New("title cannot be empty")
	}

	return &SBI{
		ID:    id,
		Title: title,
		Body:  body,
	}, nil
}
