package repository

import (
	"context"

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
}

// SBIFilter defines criteria for filtering SBIs
type SBIFilter struct {
	PBIID    *PBIID   // Filter by parent PBI
	Labels   []string // Filter by labels
	Statuses []Status
	Limit    int
	Offset   int
}
