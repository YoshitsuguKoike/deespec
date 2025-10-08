package epic

import (
	"errors"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// EPIC represents a large feature group that can contain multiple PBIs
// EPIC is an aggregate root in DDD terms
type EPIC struct {
	base     *task.BaseTask
	pbiIDs   []model.TaskID // Child PBI IDs
	metadata EPICMetadata
}

// EPICMetadata contains EPIC-specific metadata
type EPICMetadata struct {
	EstimatedStoryPoints int
	Priority             int
	Labels               []string
	AssignedAgent        string // e.g., "claude-code", "gemini-cli"
}

// NewEPIC creates a new EPIC
func NewEPIC(title, description string, metadata EPICMetadata) (*EPIC, error) {
	baseTask, err := task.NewBaseTask(
		model.TaskTypeEPIC,
		title,
		description,
		nil, // EPIC has no parent
	)
	if err != nil {
		return nil, err
	}

	return &EPIC{
		base:     baseTask,
		pbiIDs:   []model.TaskID{},
		metadata: metadata,
	}, nil
}

// ReconstructEPIC reconstructs an EPIC from stored data
func ReconstructEPIC(
	id model.TaskID,
	title string,
	description string,
	status model.Status,
	currentStep model.Step,
	pbiIDs []model.TaskID,
	metadata EPICMetadata,
	createdAt time.Time,
	updatedAt time.Time,
) *EPIC {
	baseTask := task.ReconstructBaseTask(
		id,
		model.TaskTypeEPIC,
		title,
		description,
		status,
		currentStep,
		nil, // EPIC has no parent
		createdAt,
		updatedAt,
	)

	return &EPIC{
		base:     baseTask,
		pbiIDs:   pbiIDs,
		metadata: metadata,
	}
}

// Implement Task interface
func (e *EPIC) ID() model.TaskID {
	return e.base.ID()
}

func (e *EPIC) Type() model.TaskType {
	return e.base.Type()
}

func (e *EPIC) Title() string {
	return e.base.Title()
}

func (e *EPIC) Description() string {
	return e.base.Description()
}

func (e *EPIC) Status() model.Status {
	return e.base.Status()
}

func (e *EPIC) CurrentStep() model.Step {
	return e.base.CurrentStep()
}

func (e *EPIC) ParentTaskID() *model.TaskID {
	return e.base.ParentTaskID()
}

func (e *EPIC) CreatedAt() model.Timestamp {
	return e.base.CreatedAt()
}

func (e *EPIC) UpdatedAt() model.Timestamp {
	return e.base.UpdatedAt()
}

func (e *EPIC) UpdateStatus(newStatus model.Status) error {
	return e.base.UpdateStatus(newStatus)
}

func (e *EPIC) UpdateStep(newStep model.Step) error {
	return e.base.UpdateStep(newStep)
}

// EPIC-specific methods

// AddPBI adds a child PBI to this EPIC
func (e *EPIC) AddPBI(pbiID model.TaskID) error {
	// Check if PBI already exists
	for _, id := range e.pbiIDs {
		if id.Equals(pbiID) {
			return errors.New("PBI already exists in this EPIC")
		}
	}

	e.pbiIDs = append(e.pbiIDs, pbiID)
	return nil
}

// RemovePBI removes a child PBI from this EPIC
func (e *EPIC) RemovePBI(pbiID model.TaskID) error {
	for i, id := range e.pbiIDs {
		if id.Equals(pbiID) {
			e.pbiIDs = append(e.pbiIDs[:i], e.pbiIDs[i+1:]...)
			return nil
		}
	}
	return errors.New("PBI not found in this EPIC")
}

// PBIIDs returns the list of child PBI IDs
func (e *EPIC) PBIIDs() []model.TaskID {
	// Return a copy to prevent external modification
	result := make([]model.TaskID, len(e.pbiIDs))
	copy(result, e.pbiIDs)
	return result
}

// HasPBIs checks if this EPIC has any PBIs
func (e *EPIC) HasPBIs() bool {
	return len(e.pbiIDs) > 0
}

// PBICount returns the number of child PBIs
func (e *EPIC) PBICount() int {
	return len(e.pbiIDs)
}

// Metadata returns the EPIC metadata
func (e *EPIC) Metadata() EPICMetadata {
	return e.metadata
}

// UpdateMetadata updates the EPIC metadata
func (e *EPIC) UpdateMetadata(metadata EPICMetadata) {
	e.metadata = metadata
}

// UpdateTitle updates the EPIC title
func (e *EPIC) UpdateTitle(title string) error {
	return e.base.UpdateTitle(title)
}

// UpdateDescription updates the EPIC description
func (e *EPIC) UpdateDescription(description string) {
	e.base.UpdateDescription(description)
}

// CanDelete checks if the EPIC can be deleted
func (e *EPIC) CanDelete() bool {
	// EPIC can only be deleted if it has no child PBIs
	return !e.HasPBIs()
}

// IsCompleted checks if the EPIC is completed
func (e *EPIC) IsCompleted() bool {
	return e.base.Status() == model.StatusDone
}

// IsFailed checks if the EPIC has failed
func (e *EPIC) IsFailed() bool {
	return e.base.Status() == model.StatusFailed
}
