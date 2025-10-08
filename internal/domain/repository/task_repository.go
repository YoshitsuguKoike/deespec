package repository

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// TaskRepository is a unified repository interface for Task entities (EPIC/PBI/SBI)
// This interface allows polymorphic handling of different task types
type TaskRepository interface {
	// FindByID retrieves a task by its ID (works for EPIC/PBI/SBI)
	FindByID(ctx context.Context, id TaskID) (task.Task, error)

	// Save persists a task entity
	Save(ctx context.Context, t task.Task) error

	// Delete removes a task
	Delete(ctx context.Context, id TaskID) error

	// List retrieves tasks by filter criteria
	List(ctx context.Context, filter TaskFilter) ([]task.Task, error)
}

// TaskID is a type-safe task identifier
type TaskID string

// TaskType represents the type of task
type TaskType string

const (
	TaskTypeEPIC TaskType = "EPIC"
	TaskTypePBI  TaskType = "PBI"
	TaskTypeSBI  TaskType = "SBI"
)

// Step represents workflow steps
type Step string

const (
	StepPick      Step = "pick"
	StepImplement Step = "implement"
	StepReview    Step = "review"
	StepDone      Step = "done"
)

// Status represents task status
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
)

// TaskFilter defines criteria for filtering tasks
type TaskFilter struct {
	Types     []TaskType // Filter by task types
	Statuses  []Status   // Filter by statuses
	Steps     []Step     // Filter by current steps
	ParentID  *TaskID    // Filter by parent task
	HasParent *bool      // Filter tasks with/without parent
	Limit     int        // Limit number of results
	Offset    int        // Offset for pagination
}
