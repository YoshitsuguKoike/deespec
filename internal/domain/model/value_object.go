package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TaskID represents a unique identifier for a task
type TaskID struct {
	value string
}

// NewTaskID creates a new TaskID
func NewTaskID() TaskID {
	return TaskID{value: uuid.New().String()}
}

// NewTaskIDFromString creates a TaskID from an existing string
func NewTaskIDFromString(id string) (TaskID, error) {
	if id == "" {
		return TaskID{}, errors.New("task ID cannot be empty")
	}
	return TaskID{value: id}, nil
}

// String returns the string representation
func (t TaskID) String() string {
	return t.value
}

// Equals checks if two TaskIDs are equal
func (t TaskID) Equals(other TaskID) bool {
	return t.value == other.value
}

// TaskType represents the type of task
type TaskType string

const (
	TaskTypeEPIC TaskType = "EPIC"
	TaskTypePBI  TaskType = "PBI"
	TaskTypeSBI  TaskType = "SBI"
)

// String returns the string representation
func (t TaskType) String() string {
	return string(t)
}

// IsValid validates the task type
func (t TaskType) IsValid() bool {
	switch t {
	case TaskTypeEPIC, TaskTypePBI, TaskTypeSBI:
		return true
	default:
		return false
	}
}

// Status represents the current status of a task
type Status string

const (
	StatusPending      Status = "PENDING"
	StatusPicked       Status = "PICKED"
	StatusImplementing Status = "IMPLEMENTING"
	StatusReviewing    Status = "REVIEWING"
	StatusDone         Status = "DONE"
	StatusFailed       Status = "FAILED"
)

// String returns the string representation
func (s Status) String() string {
	return string(s)
}

// IsValid validates the status
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusPicked, StatusImplementing, StatusReviewing, StatusDone, StatusFailed:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if a status transition is valid
func (s Status) CanTransitionTo(next Status) bool {
	validTransitions := map[Status][]Status{
		StatusPending:      {StatusPicked},
		StatusPicked:       {StatusImplementing, StatusPending},
		StatusImplementing: {StatusReviewing, StatusFailed, StatusPending},
		StatusReviewing:    {StatusDone, StatusImplementing, StatusFailed},
		StatusDone:         {},
		StatusFailed:       {StatusPending},
	}

	allowed, exists := validTransitions[s]
	if !exists {
		return false
	}

	for _, allowedStatus := range allowed {
		if allowedStatus == next {
			return true
		}
	}
	return false
}

// Step represents a workflow step
type Step string

const (
	StepPick      Step = "PICK"
	StepImplement Step = "IMPLEMENT"
	StepReview    Step = "REVIEW"
	StepDone      Step = "DONE"
)

// String returns the string representation
func (s Step) String() string {
	return string(s)
}

// IsValid validates the step
func (s Step) IsValid() bool {
	switch s {
	case StepPick, StepImplement, StepReview, StepDone:
		return true
	default:
		return false
	}
}

// Turn represents an execution turn counter
type Turn struct {
	value int
}

// NewTurn creates a new Turn starting from 1
func NewTurn() Turn {
	return Turn{value: 1}
}

// NewTurnFromInt creates a Turn from an integer value
func NewTurnFromInt(value int) (Turn, error) {
	if value < 1 {
		return Turn{}, errors.New("turn value must be at least 1")
	}
	return Turn{value: value}, nil
}

// Value returns the integer value
func (t Turn) Value() int {
	return t.value
}

// Increment returns a new Turn with incremented value
func (t Turn) Increment() Turn {
	return Turn{value: t.value + 1}
}

// Equals checks if two Turns are equal
func (t Turn) Equals(other Turn) bool {
	return t.value == other.value
}

// String returns the string representation
func (t Turn) String() string {
	return fmt.Sprintf("Turn %d", t.value)
}

// Attempt represents an execution attempt counter
type Attempt struct {
	value int
}

// NewAttempt creates a new Attempt starting from 1
func NewAttempt() Attempt {
	return Attempt{value: 1}
}

// NewAttemptFromInt creates an Attempt from an integer value
func NewAttemptFromInt(value int) (Attempt, error) {
	if value < 1 {
		return Attempt{}, errors.New("attempt value must be at least 1")
	}
	return Attempt{value: value}, nil
}

// Value returns the integer value
func (a Attempt) Value() int {
	return a.value
}

// Increment returns a new Attempt with incremented value
func (a Attempt) Increment() Attempt {
	return Attempt{value: a.value + 1}
}

// Equals checks if two Attempts are equal
func (a Attempt) Equals(other Attempt) bool {
	return a.value == other.value
}

// String returns the string representation
func (a Attempt) String() string {
	return fmt.Sprintf("Attempt %d", a.value)
}

// Timestamp represents a point in time
type Timestamp struct {
	value time.Time
}

// NewTimestamp creates a new Timestamp with current time
func NewTimestamp() Timestamp {
	return Timestamp{value: time.Now()}
}

// NewTimestampFromTime creates a Timestamp from a time.Time value
func NewTimestampFromTime(t time.Time) Timestamp {
	return Timestamp{value: t}
}

// Value returns the time.Time value
func (t Timestamp) Value() time.Time {
	return t.value
}

// Before checks if this timestamp is before another
func (t Timestamp) Before(other Timestamp) bool {
	return t.value.Before(other.value)
}

// After checks if this timestamp is after another
func (t Timestamp) After(other Timestamp) bool {
	return t.value.After(other.value)
}

// String returns the string representation
func (t Timestamp) String() string {
	return t.value.Format(time.RFC3339)
}
