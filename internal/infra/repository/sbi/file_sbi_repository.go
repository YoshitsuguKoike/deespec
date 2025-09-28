package sbi

import (
	"context"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/infra/persistence/file"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

// FileSBIRepository is a file-based implementation of the SBI repository
type FileSBIRepository struct {
	FS afero.Fs
}

// NewFileSBIRepository creates a new file-based SBI repository
func NewFileSBIRepository(fs afero.Fs) *FileSBIRepository {
	return &FileSBIRepository{FS: fs}
}

// baseSBI is the base directory for SBI specifications
const baseSBI = ".deespec/specs/sbi"

// Save persists an SBI entity to the filesystem
// The spec is saved to .deespec/specs/sbi/<SBI-ID>/spec.md
// The meta is saved to .deespec/specs/sbi/<SBI-ID>/meta.yml
func (r *FileSBIRepository) Save(ctx context.Context, s *sbi.SBI) (string, error) {
	// Construct the directory path for this SBI
	specDir := filepath.Join(baseSBI, s.ID)

	// Construct the full path to spec.md
	specPath := filepath.Join(specDir, "spec.md")

	// Write the spec file atomically
	if err := file.WriteFileAtomic(r.FS, specPath, []byte(s.Body)); err != nil {
		return "", err
	}

	// Create and save meta.yml with labels
	meta := sbi.NewMeta(s.ID, s.Title, time.Now())
	meta.Labels = s.Labels // Set labels from SBI entity
	metaPath := filepath.Join(specDir, "meta.yml")

	// Marshal meta to YAML
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return "", err
	}

	// Write the meta file atomically
	if err := file.WriteFileAtomic(r.FS, metaPath, metaData); err != nil {
		return "", err
	}

	return specPath, nil
}
