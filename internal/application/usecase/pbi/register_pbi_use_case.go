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

	// 2. Use PBI-ID as title if title is empty, and update body with H1 header
	if p.Title == "" {
		p.Title = id
		// Prepend H1 header to body if not present
		if body != "" {
			body = fmt.Sprintf("# %s\n\n%s", id, body)
		} else {
			body = fmt.Sprintf("# %s\n", id)
		}
	}

	// 3. Validate PBI
	if err := p.Validate(); err != nil {
		return "", fmt.Errorf("validation failed: %w", err)
	}

	// 4. Save PBI (metadata + Markdown body)
	if err := u.repo.Save(p, body); err != nil {
		return "", fmt.Errorf("failed to save PBI: %w", err)
	}

	return p.ID, nil
}
