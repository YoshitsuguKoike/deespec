package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
)

// SBIApprovalRepository manages SBI approval manifests stored in approval.yaml files
type SBIApprovalRepository interface {
	// LoadManifest loads the approval manifest for a given PBI ID
	// Returns an error if the file does not exist or cannot be parsed
	LoadManifest(ctx context.Context, pbiID PBIID) (*pbi.SBIApprovalManifest, error)

	// SaveManifest persists the approval manifest to approval.yaml
	// Creates the directory structure if it doesn't exist
	SaveManifest(ctx context.Context, manifest *pbi.SBIApprovalManifest) error

	// ManifestExists checks if an approval manifest exists for the given PBI ID
	ManifestExists(ctx context.Context, pbiID PBIID) (bool, error)

	// DeleteManifest removes the approval manifest file for the given PBI ID
	// Returns an error if the file does not exist
	DeleteManifest(ctx context.Context, pbiID PBIID) error
}
