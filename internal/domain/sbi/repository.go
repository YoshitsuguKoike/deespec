package sbi

import (
	"context"
)

// Repository defines the interface for SBI persistence
// This interface belongs to the domain layer and is implemented by the infrastructure layer
type Repository interface {
	// Save persists an SBI entity and returns the path where it was saved
	Save(ctx context.Context, s *SBI) (specPath string, err error)
}