package pbi

import (
	"errors"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// PBI represents a Product Backlog Item (medium-sized task)
// PBI can optionally belong to an EPIC and can contain multiple SBIs
// PBI is an aggregate root in DDD terms
type PBI struct {
	base     *task.BaseTask
	sbiIDs   []model.TaskID // Child SBI IDs
	metadata PBIMetadata
}

// PBIMetadata contains PBI-specific metadata
type PBIMetadata struct {
	StoryPoints        int
	Priority           int
	Labels             []string
	AssignedAgent      string // e.g., "claude-code", "gemini-cli"
	AcceptanceCriteria []string
}

// NewPBI creates a new PBI
func NewPBI(title, description string, parentEPICID *model.TaskID, metadata PBIMetadata) (*PBI, error) {
	baseTask, err := task.NewBaseTask(
		model.TaskTypePBI,
		title,
		description,
		parentEPICID, // PBI can optionally have an EPIC as parent
	)
	if err != nil {
		return nil, err
	}

	return &PBI{
		base:     baseTask,
		sbiIDs:   []model.TaskID{},
		metadata: metadata,
	}, nil
}

// ReconstructPBI reconstructs a PBI from stored data
func ReconstructPBI(
	id model.TaskID,
	title string,
	description string,
	status model.Status,
	currentStep model.Step,
	parentEPICID *model.TaskID,
	sbiIDs []model.TaskID,
	metadata PBIMetadata,
	createdAt time.Time,
	updatedAt time.Time,
) *PBI {
	baseTask := task.ReconstructBaseTask(
		id,
		model.TaskTypePBI,
		title,
		description,
		status,
		currentStep,
		parentEPICID,
		createdAt,
		updatedAt,
	)

	return &PBI{
		base:     baseTask,
		sbiIDs:   sbiIDs,
		metadata: metadata,
	}
}

// Implement Task interface
func (p *PBI) ID() model.TaskID {
	return p.base.ID()
}

func (p *PBI) Type() model.TaskType {
	return p.base.Type()
}

func (p *PBI) Title() string {
	return p.base.Title()
}

func (p *PBI) Description() string {
	return p.base.Description()
}

func (p *PBI) Status() model.Status {
	return p.base.Status()
}

func (p *PBI) CurrentStep() model.Step {
	return p.base.CurrentStep()
}

func (p *PBI) ParentTaskID() *model.TaskID {
	return p.base.ParentTaskID()
}

func (p *PBI) CreatedAt() model.Timestamp {
	return p.base.CreatedAt()
}

func (p *PBI) UpdatedAt() model.Timestamp {
	return p.base.UpdatedAt()
}

func (p *PBI) UpdateStatus(newStatus model.Status) error {
	return p.base.UpdateStatus(newStatus)
}

func (p *PBI) UpdateStep(newStep model.Step) error {
	return p.base.UpdateStep(newStep)
}

// PBI-specific methods

// AddSBI adds a child SBI to this PBI
func (p *PBI) AddSBI(sbiID model.TaskID) error {
	// Check if SBI already exists
	for _, id := range p.sbiIDs {
		if id.Equals(sbiID) {
			return errors.New("SBI already exists in this PBI")
		}
	}

	p.sbiIDs = append(p.sbiIDs, sbiID)
	return nil
}

// RemoveSBI removes a child SBI from this PBI
func (p *PBI) RemoveSBI(sbiID model.TaskID) error {
	for i, id := range p.sbiIDs {
		if id.Equals(sbiID) {
			p.sbiIDs = append(p.sbiIDs[:i], p.sbiIDs[i+1:]...)
			return nil
		}
	}
	return errors.New("SBI not found in this PBI")
}

// SBIIDs returns the list of child SBI IDs
func (p *PBI) SBIIDs() []model.TaskID {
	// Return a copy to prevent external modification
	result := make([]model.TaskID, len(p.sbiIDs))
	copy(result, p.sbiIDs)
	return result
}

// HasSBIs checks if this PBI has any SBIs
func (p *PBI) HasSBIs() bool {
	return len(p.sbiIDs) > 0
}

// SBICount returns the number of child SBIs
func (p *PBI) SBICount() int {
	return len(p.sbiIDs)
}

// Metadata returns the PBI metadata
func (p *PBI) Metadata() PBIMetadata {
	return p.metadata
}

// UpdateMetadata updates the PBI metadata
func (p *PBI) UpdateMetadata(metadata PBIMetadata) {
	p.metadata = metadata
}

// UpdateTitle updates the PBI title
func (p *PBI) UpdateTitle(title string) error {
	return p.base.UpdateTitle(title)
}

// UpdateDescription updates the PBI description
func (p *PBI) UpdateDescription(description string) {
	p.base.UpdateDescription(description)
}

// HasParentEPIC checks if this PBI belongs to an EPIC
func (p *PBI) HasParentEPIC() bool {
	return p.base.ParentTaskID() != nil
}

// CanDelete checks if the PBI can be deleted
func (p *PBI) CanDelete() bool {
	// PBI can only be deleted if it has no child SBIs
	return !p.HasSBIs()
}

// IsCompleted checks if the PBI is completed
func (p *PBI) IsCompleted() bool {
	return p.base.Status() == model.StatusDone
}

// IsFailed checks if the PBI has failed
func (p *PBI) IsFailed() bool {
	return p.base.Status() == model.StatusFailed
}
