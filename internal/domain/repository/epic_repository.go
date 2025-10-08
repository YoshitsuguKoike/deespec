package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
)

// EPICRepository manages EPIC entities
type EPICRepository interface {
	// Find retrieves an EPIC by its ID
	Find(ctx context.Context, id EPICID) (*epic.EPIC, error)

	// Save persists an EPIC entity
	Save(ctx context.Context, e *epic.EPIC) error

	// Delete removes an EPIC
	Delete(ctx context.Context, id EPICID) error

	// List retrieves EPICs by filter
	List(ctx context.Context, filter EPICFilter) ([]*epic.EPIC, error)

	// FindByPBIID retrieves the parent EPIC of a PBI
	FindByPBIID(ctx context.Context, pbiID PBIID) (*epic.EPIC, error)
}

// EPICID is a type-safe EPIC identifier
type EPICID string

// EPICFilter defines criteria for filtering EPICs
type EPICFilter struct {
	Statuses []Status
	Limit    int
	Offset   int
}

// PBIID is imported from PBI repository
type PBIID string
