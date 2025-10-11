package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"gopkg.in/yaml.v3"
)

// SBIApprovalRepositoryImpl implements SBIApprovalRepository for file-based storage
type SBIApprovalRepositoryImpl struct{}

// NewSBIApprovalRepositoryImpl creates a new file-based SBI approval repository
func NewSBIApprovalRepositoryImpl() repository.SBIApprovalRepository {
	return &SBIApprovalRepositoryImpl{}
}

// LoadManifest loads the approval manifest from approval.yaml
func (r *SBIApprovalRepositoryImpl) LoadManifest(ctx context.Context, pbiID repository.PBIID) (*pbi.SBIApprovalManifest, error) {
	manifestPath := r.getManifestPath(pbiID)

	// Read file content
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("approval manifest not found for PBI %s: %w", pbiID, err)
		}
		return nil, fmt.Errorf("failed to read approval manifest for PBI %s: %w", pbiID, err)
	}

	// Parse YAML
	var manifest pbi.SBIApprovalManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse approval manifest for PBI %s: %w", pbiID, err)
	}

	return &manifest, nil
}

// SaveManifest persists the approval manifest to approval.yaml
func (r *SBIApprovalRepositoryImpl) SaveManifest(ctx context.Context, manifest *pbi.SBIApprovalManifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest cannot be nil")
	}

	manifestPath := r.getManifestPath(repository.PBIID(manifest.PBIID))

	// Create directory if it doesn't exist
	dir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal approval manifest for PBI %s: %w", manifest.PBIID, err)
	}

	// Write file atomically
	if err := r.atomicWrite(manifestPath, data); err != nil {
		return fmt.Errorf("failed to write approval manifest for PBI %s: %w", manifest.PBIID, err)
	}

	return nil
}

// ManifestExists checks if an approval manifest exists for the given PBI ID
func (r *SBIApprovalRepositoryImpl) ManifestExists(ctx context.Context, pbiID repository.PBIID) (bool, error) {
	manifestPath := r.getManifestPath(pbiID)

	_, err := os.Stat(manifestPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("failed to check if approval manifest exists for PBI %s: %w", pbiID, err)
}

// DeleteManifest removes the approval manifest file
func (r *SBIApprovalRepositoryImpl) DeleteManifest(ctx context.Context, pbiID repository.PBIID) error {
	manifestPath := r.getManifestPath(pbiID)

	// Check if file exists first
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("approval manifest not found for PBI %s: %w", pbiID, err)
		}
		return fmt.Errorf("failed to check approval manifest for PBI %s: %w", pbiID, err)
	}

	// Remove the file
	if err := os.Remove(manifestPath); err != nil {
		return fmt.Errorf("failed to delete approval manifest for PBI %s: %w", pbiID, err)
	}

	return nil
}

// getManifestPath generates the file path for the approval manifest
func (r *SBIApprovalRepositoryImpl) getManifestPath(pbiID repository.PBIID) string {
	return filepath.Join(".deespec", "specs", "pbi", string(pbiID), "approval.yaml")
}

// atomicWrite writes data to a file atomically using a temp file and rename
func (r *SBIApprovalRepositoryImpl) atomicWrite(path string, data []byte) error {
	// Create temp file in the same directory as the target
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-approval-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup in case of error
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	// Write data to temp file
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is written to disk
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	// Close the temp file
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Atomic rename (file permissions are set to 0644 by default with CreateTemp)
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", path, err)
	}

	return nil
}
