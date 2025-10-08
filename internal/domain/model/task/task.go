package task

import (
	"errors"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
)

// Task is the common interface for all task types (EPIC, PBI, SBI)
type Task interface {
	// ID returns the unique identifier
	ID() model.TaskID

	// Type returns the task type
	Type() model.TaskType

	// Title returns the task title
	Title() string

	// Description returns the task description
	Description() string

	// Status returns the current status
	Status() model.Status

	// CurrentStep returns the current workflow step
	CurrentStep() model.Step

	// ParentTaskID returns the parent task ID (nil if no parent)
	ParentTaskID() *model.TaskID

	// CreatedAt returns the creation timestamp
	CreatedAt() model.Timestamp

	// UpdatedAt returns the last update timestamp
	UpdatedAt() model.Timestamp

	// UpdateStatus transitions to a new status
	UpdateStatus(newStatus model.Status) error

	// UpdateStep moves to a new workflow step
	UpdateStep(newStep model.Step) error
}

// BaseTask contains common fields and behavior for all task types
type BaseTask struct {
	id          model.TaskID
	taskType    model.TaskType
	title       string
	description string
	status      model.Status
	currentStep model.Step
	parentID    *model.TaskID
	createdAt   model.Timestamp
	updatedAt   model.Timestamp
}

// NewBaseTask creates a new base task
func NewBaseTask(
	taskType model.TaskType,
	title string,
	description string,
	parentID *model.TaskID,
) (*BaseTask, error) {
	if !taskType.IsValid() {
		return nil, errors.New("invalid task type")
	}
	if title == "" {
		return nil, errors.New("title cannot be empty")
	}

	now := model.NewTimestamp()
	return &BaseTask{
		id:          model.NewTaskID(),
		taskType:    taskType,
		title:       title,
		description: description,
		status:      model.StatusPending,
		currentStep: model.StepPick,
		parentID:    parentID,
		createdAt:   now,
		updatedAt:   now,
	}, nil
}

// ReconstructBaseTask reconstructs a base task from stored data
func ReconstructBaseTask(
	id model.TaskID,
	taskType model.TaskType,
	title string,
	description string,
	status model.Status,
	currentStep model.Step,
	parentID *model.TaskID,
	createdAt time.Time,
	updatedAt time.Time,
) *BaseTask {
	return &BaseTask{
		id:          id,
		taskType:    taskType,
		title:       title,
		description: description,
		status:      status,
		currentStep: currentStep,
		parentID:    parentID,
		createdAt:   model.NewTimestampFromTime(createdAt),
		updatedAt:   model.NewTimestampFromTime(updatedAt),
	}
}

// ID returns the task ID
func (b *BaseTask) ID() model.TaskID {
	return b.id
}

// Type returns the task type
func (b *BaseTask) Type() model.TaskType {
	return b.taskType
}

// Title returns the title
func (b *BaseTask) Title() string {
	return b.title
}

// Description returns the description
func (b *BaseTask) Description() string {
	return b.description
}

// Status returns the current status
func (b *BaseTask) Status() model.Status {
	return b.status
}

// CurrentStep returns the current workflow step
func (b *BaseTask) CurrentStep() model.Step {
	return b.currentStep
}

// ParentTaskID returns the parent task ID
func (b *BaseTask) ParentTaskID() *model.TaskID {
	return b.parentID
}

// CreatedAt returns the creation timestamp
func (b *BaseTask) CreatedAt() model.Timestamp {
	return b.createdAt
}

// UpdatedAt returns the last update timestamp
func (b *BaseTask) UpdatedAt() model.Timestamp {
	return b.updatedAt
}

// UpdateStatus transitions to a new status
func (b *BaseTask) UpdateStatus(newStatus model.Status) error {
	if !newStatus.IsValid() {
		return errors.New("invalid status")
	}

	if !b.status.CanTransitionTo(newStatus) {
		return errors.New("invalid status transition from " + b.status.String() + " to " + newStatus.String())
	}

	b.status = newStatus
	b.updatedAt = model.NewTimestamp()
	return nil
}

// UpdateStep moves to a new workflow step
func (b *BaseTask) UpdateStep(newStep model.Step) error {
	if !newStep.IsValid() {
		return errors.New("invalid step")
	}

	b.currentStep = newStep
	b.updatedAt = model.NewTimestamp()
	return nil
}

// UpdateTitle updates the task title
func (b *BaseTask) UpdateTitle(title string) error {
	if title == "" {
		return errors.New("title cannot be empty")
	}
	b.title = title
	b.updatedAt = model.NewTimestamp()
	return nil
}

// UpdateDescription updates the task description
func (b *BaseTask) UpdateDescription(description string) {
	b.description = description
	b.updatedAt = model.NewTimestamp()
}
