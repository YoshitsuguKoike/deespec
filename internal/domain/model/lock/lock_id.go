package lock

import (
	"errors"
	"fmt"
)

// Common lock errors
var (
	ErrLockNotFound = errors.New("lock not found")
)

// LockID is a value object representing a unique lock identifier
// It identifies the resource being locked (e.g., SBI ID, state file path)
type LockID struct {
	value string
}

// NewLockID creates a new lock ID
func NewLockID(value string) (LockID, error) {
	if value == "" {
		return LockID{}, fmt.Errorf("lock ID cannot be empty")
	}
	return LockID{value: value}, nil
}

// String returns the string representation of the lock ID
func (id LockID) String() string {
	return id.value
}

// Equals checks if two lock IDs are equal
func (id LockID) Equals(other LockID) bool {
	return id.value == other.value
}
