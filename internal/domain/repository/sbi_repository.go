package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

// SBIRepository manages SBI entities
type SBIRepository interface {
	// Find retrieves an SBI by its ID
	Find(ctx context.Context, id SBIID) (*sbi.SBI, error)

	// Save persists an SBI entity
	Save(ctx context.Context, s *sbi.SBI) error

	// Delete removes an SBI
	Delete(ctx context.Context, id SBIID) error

	// List retrieves SBIs by filter
	List(ctx context.Context, filter SBIFilter) ([]*sbi.SBI, error)

	// FindByPBIID retrieves all SBIs belonging to a PBI
	FindByPBIID(ctx context.Context, pbiID PBIID) ([]*sbi.SBI, error)

	// GetNextSequence retrieves the next available sequence number
	GetNextSequence(ctx context.Context) (int, error)

	// ResetSBIState resets an SBI to a specific status (for testing/maintenance)
	ResetSBIState(ctx context.Context, id SBIID, toStatus string) error

	// GetDependencies retrieves the list of SBI IDs that the given SBI depends on
	GetDependencies(ctx context.Context, sbiID SBIID) ([]string, error)

	// GetDependents retrieves the list of SBI IDs that depend on the given SBI
	GetDependents(ctx context.Context, sbiID SBIID) ([]string, error)

	// SaveDependencies persists the dependencies for an SBI
	// This replaces all existing dependencies with the provided list
	SaveDependencies(ctx context.Context, sbiID SBIID, dependsOn []string) error
}

// SBIFilter defines criteria for filtering SBIs
type SBIFilter struct {
	PBIID    *PBIID         // Filter by parent PBI
	Labels   []string       // Filter by labels
	Statuses []model.Status // Filter by status (uses domain model Status)
	Limit    int
	Offset   int
}
