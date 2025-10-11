package pbi

import (
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
)

// RegisterPBIUseCase handles PBI registration
type RegisterPBIUseCase struct {
	repo pbi.Repository
}

// NewRegisterPBIUseCase creates a new RegisterPBIUseCase
func NewRegisterPBIUseCase(repo pbi.Repository) *RegisterPBIUseCase {
	return &RegisterPBIUseCase{
		repo: repo,
	}
}

// Execute registers a new PBI
func (u *RegisterPBIUseCase) Execute(p *pbi.PBI, body string) (string, error) {
	// 1. Generate ID
	id, err := pbi.GenerateID(u.repo)
	if err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}
	p.ID = id

	// 2. Validate PBI
	if err := p.Validate(); err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 3. Save PBI (metadata + Markdown body)
	if err := u.repo.Save(p, body); err != nil {
		return "", fmt.Errorf("failed to save PBI: %w", err)
	}

	return p.ID, nil
}
