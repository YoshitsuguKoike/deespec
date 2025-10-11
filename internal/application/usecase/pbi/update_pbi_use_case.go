package pbi

import (
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
)

// UpdatePBIUseCase handles PBI update operations
type UpdatePBIUseCase struct {
	repo pbi.Repository
}

// NewUpdatePBIUseCase creates a new UpdatePBIUseCase
func NewUpdatePBIUseCase(repo pbi.Repository) *UpdatePBIUseCase {
	return &UpdatePBIUseCase{repo: repo}
}

// UpdateOptions represents the fields that can be updated
type UpdateOptions struct {
	Status               *pbi.Status
	EstimatedStoryPoints *int
	Priority             *pbi.Priority
}

// Execute updates a PBI's metadata
func (u *UpdatePBIUseCase) Execute(id string, opts UpdateOptions) error {
	// 1. Check if PBI exists
	exists, err := u.repo.Exists(id)
	if err != nil {
		return fmt.Errorf("failed to check PBI existence: %w", err)
	}
	if !exists {
		return fmt.Errorf("PBI not found: %s", id)
	}

	// 2. Load existing PBI
	p, err := u.repo.FindByID(id)
	if err != nil {
		return fmt.Errorf("failed to load PBI: %w", err)
	}

	// 3. Load existing body (we need to preserve it)
	body, err := u.repo.GetBody(id)
	if err != nil {
		return fmt.Errorf("failed to load PBI body: %w", err)
	}

	// 4. Apply updates
	if opts.Status != nil {
		p.Status = *opts.Status
	}
	if opts.EstimatedStoryPoints != nil {
		p.EstimatedStoryPoints = *opts.EstimatedStoryPoints
	}
	if opts.Priority != nil {
		p.Priority = *opts.Priority
	}

	// 5. Update timestamp
	p.UpdatedAt = time.Now()

	// 6. Validate updated PBI
	if err := p.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// 7. Save updated PBI
	if err := u.repo.Save(p, body); err != nil {
		return fmt.Errorf("failed to update PBI: %w", err)
	}

	return nil
}
