package sbi

import (
	"context"
	"io"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/sbi"
	"github.com/oklog/ulid/v2"
)

// RegisterSBIUseCase handles the registration of new SBI specifications
type RegisterSBIUseCase struct {
	Repo sbi.Repository   // Repository for persisting SBI entities
	Now  func() time.Time // Time provider (for testing)
	Rand io.Reader        // Random source for ULID generation (for testing)
}

// Execute performs the SBI registration use case
// It generates a new ULID-based ID, builds the spec markdown content,
// creates the SBI entity, and saves it through the repository
func (uc *RegisterSBIUseCase) Execute(ctx context.Context, in RegisterSBIInput) (*RegisterSBIOutput, error) {
	// Generate ULID-based ID with SBI- prefix
	timestamp := ulid.Timestamp(uc.Now())
	ulidValue := ulid.MustNew(timestamp, uc.Rand)
	id := "SBI-" + ulidValue.String()

	// Build the complete markdown content with guidelines and title
	content := BuildSpecMarkdown(in.Title, in.Body)

	// Create the SBI entity
	entity, err := sbi.NewSBI(id, in.Title, content)
	if err != nil {
		return nil, err
	}

	// Save the entity through the repository
	specPath, err := uc.Repo.Save(ctx, entity)
	if err != nil {
		return nil, err
	}

	return &RegisterSBIOutput{
		ID:       id,
		SpecPath: specPath,
	}, nil
}
