package sbi_test

import (
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/sbi"
)

func TestNewSBI(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		title   string
		body    string
		labels  []string
		wantErr bool
	}{
		{
			name:    "Valid SBI creation",
			id:      "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P78",
			title:   "Test Specification",
			body:    "This is the body content",
			labels:  []string{"test", "unit"},
			wantErr: false,
		},
		{
			name:    "Empty title should fail",
			id:      "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P79",
			title:   "",
			body:    "This is the body content",
			labels:  nil,
			wantErr: true,
		},
		{
			name:    "Empty body should be allowed",
			id:      "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P80",
			title:   "Test Specification",
			body:    "",
			labels:  []string{},
			wantErr: false,
		},
		{
			name:    "Nil labels should be allowed",
			id:      "SBI-01J8X5YNFZ4TQ5H5N5RQNT5P81",
			title:   "Test Specification",
			body:    "Content",
			labels:  nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sbi.NewSBI(tt.id, tt.title, tt.body, tt.labels)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				if result != nil {
					t.Errorf("Expected nil result when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result == nil {
					t.Fatal("Expected non-nil result")
				}
				if result.ID != tt.id {
					t.Errorf("ID mismatch: got %s, want %s", result.ID, tt.id)
				}
				if result.Title != tt.title {
					t.Errorf("Title mismatch: got %s, want %s", result.Title, tt.title)
				}
				if result.Body != tt.body {
					t.Errorf("Body mismatch: got %s, want %s", result.Body, tt.body)
				}
				// Check labels (should be non-nil even if empty)
				if result.Labels == nil {
					t.Error("Labels should not be nil")
				}
				if tt.labels != nil && len(result.Labels) != len(tt.labels) {
					t.Errorf("Labels count mismatch: got %d, want %d", len(result.Labels), len(tt.labels))
				}
			}
		})
	}
}
