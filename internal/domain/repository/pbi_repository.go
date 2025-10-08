package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
)

// PBIRepository manages PBI entities
type PBIRepository interface {
	// Find retrieves a PBI by its ID
	Find(ctx context.Context, id PBIID) (*pbi.PBI, error)

	// Save persists a PBI entity
	Save(ctx context.Context, p *pbi.PBI) error

	// Delete removes a PBI
	Delete(ctx context.Context, id PBIID) error

	// List retrieves PBIs by filter
	List(ctx context.Context, filter PBIFilter) ([]*pbi.PBI, error)

	// FindByEPICID retrieves all PBIs belonging to an EPIC
	FindByEPICID(ctx context.Context, epicID EPICID) ([]*pbi.PBI, error)

	// FindBySBIID retrieves the parent PBI of an SBI
	FindBySBIID(ctx context.Context, sbiID SBIID) (*pbi.PBI, error)
}

// PBIFilter defines criteria for filtering PBIs
type PBIFilter struct {
	EPICID   *EPICID // Filter by parent EPIC
	Statuses []Status
	Limit    int
	Offset   int
}

// SBIID is imported from SBI repository
type SBIID string
