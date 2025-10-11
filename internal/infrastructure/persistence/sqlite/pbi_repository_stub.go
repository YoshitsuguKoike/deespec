package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// PBIRepositoryStub is a stub implementation of PBIRepository
// The old PBI system is being replaced with a new Markdown-based system
// This stub exists only to maintain backwards compatibility during the transition
type PBIRepositoryStub struct{}

// NewPBIRepository creates a stub PBI repository
// This is a temporary stub - use the new 'deespec pbi' commands instead
func NewPBIRepository(db *sql.DB) repository.PBIRepository {
	return &PBIRepositoryStub{}
}

func (r *PBIRepositoryStub) Find(ctx context.Context, id repository.PBIID) (*pbi.PBI, error) {
	return nil, errors.New("PBI repository is deprecated - use 'deespec pbi show' command instead")
}

func (r *PBIRepositoryStub) Save(ctx context.Context, p *pbi.PBI) error {
	return errors.New("PBI repository is deprecated - use 'deespec pbi register' command instead")
}

func (r *PBIRepositoryStub) Delete(ctx context.Context, id repository.PBIID) error {
	return errors.New("PBI repository is deprecated - use new PBI commands instead")
}

func (r *PBIRepositoryStub) List(ctx context.Context, filter repository.PBIFilter) ([]*pbi.PBI, error) {
	return nil, errors.New("PBI repository is deprecated - use 'deespec pbi list' command instead")
}

func (r *PBIRepositoryStub) FindByEPICID(ctx context.Context, epicID repository.EPICID) ([]*pbi.PBI, error) {
	return nil, errors.New("PBI repository is deprecated - use new PBI commands instead")
}

func (r *PBIRepositoryStub) FindBySBIID(ctx context.Context, sbiID repository.SBIID) (*pbi.PBI, error) {
	return nil, errors.New("PBI repository is deprecated - use new PBI commands instead")
}
