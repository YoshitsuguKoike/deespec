package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/label"
)

// ValidationStatus represents the result of label integrity validation
type ValidationStatus string

const (
	ValidationOK       ValidationStatus = "OK"       // File matches DB hash
	ValidationModified ValidationStatus = "MODIFIED" // File has been modified
	ValidationMissing  ValidationStatus = "MISSING"  // File not found
)

// ValidationResult contains the result of label validation
type ValidationResult struct {
	LabelID      int
	LabelName    string
	Status       ValidationStatus
	ExpectedHash string // Hash stored in DB
	ActualHash   string // Current file hash
	FilePath     string
}

// LabelRepository defines the interface for label persistence
type LabelRepository interface {
	// Basic CRUD operations
	Save(ctx context.Context, lbl *label.Label) error
	FindByID(ctx context.Context, id int) (*label.Label, error)
	FindByName(ctx context.Context, name string) (*label.Label, error)
	FindAll(ctx context.Context) ([]*label.Label, error)
	FindActive(ctx context.Context) ([]*label.Label, error)
	Update(ctx context.Context, lbl *label.Label) error
	Delete(ctx context.Context, id int) error

	// Label-Task association
	AttachToTask(ctx context.Context, taskID string, labelID int, position int) error
	DetachFromTask(ctx context.Context, taskID string, labelID int) error
	FindLabelsByTaskID(ctx context.Context, taskID string) ([]*label.Label, error)
	FindTaskIDsByLabelID(ctx context.Context, labelID int) ([]string, error)

	// Integrity validation (Phase 9.1 - new methods)
	ValidateIntegrity(ctx context.Context, labelID int) (*ValidationResult, error)
	ValidateAllLabels(ctx context.Context) ([]*ValidationResult, error)
	SyncFromFile(ctx context.Context, labelID int) error

	// Hierarchical operations
	FindChildren(ctx context.Context, parentID int) ([]*label.Label, error)
	FindByParentID(ctx context.Context, parentID *int) ([]*label.Label, error)
}
