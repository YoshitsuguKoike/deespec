package pbi

// Repository defines the interface for PBI persistence
// It handles both Markdown files and database metadata
type Repository interface {
	// Save saves a PBI with its Markdown body
	// - Saves metadata to database
	// - Saves body to .deespec/specs/pbi/{id}/pbi.md
	Save(pbi *PBI, body string) error

	// FindByID retrieves a PBI by ID (metadata only)
	FindByID(id string) (*PBI, error)

	// GetBody retrieves the Markdown body from file
	GetBody(id string) (string, error)

	// FindAll retrieves all PBIs (metadata only)
	FindAll() ([]*PBI, error)

	// FindByStatus retrieves PBIs by status (metadata only)
	FindByStatus(status Status) ([]*PBI, error)

	// Delete deletes a PBI (both database and Markdown file)
	Delete(id string) error

	// Exists checks if a PBI exists
	Exists(id string) (bool, error)
}
