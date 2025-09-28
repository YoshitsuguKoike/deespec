package sbi

import "time"

// Meta represents metadata for an SBI specification
type Meta struct {
	ID        string    `yaml:"id"`
	Title     string    `yaml:"title"`
	Labels    []string  `yaml:"labels"`
	CreatedAt time.Time `yaml:"created_at"`
	UpdatedAt time.Time `yaml:"updated_at"`
}

// NewMeta creates a new Meta instance
func NewMeta(id, title string, createdAt time.Time) *Meta {
	return &Meta{
		ID:        id,
		Title:     title,
		Labels:    []string{}, // Empty labels by default
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}
}
