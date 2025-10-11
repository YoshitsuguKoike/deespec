package pbi

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"
)

// PBI represents a Product Backlog Item
// Note: Body content is stored in .deespec/specs/pbi/{id}/pbi.md, not in the database
type PBI struct {
	ID                   string
	Title                string // Extracted from H1 in pbi.md
	Status               Status
	EstimatedStoryPoints int
	Priority             Priority
	ParentEpicID         string // Optional parent EPIC ID
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Status represents the PBI status (5 stages)
type Status string

const (
	StatusPending    Status = "pending"     // 未着手
	StatusPlanning   Status = "planning"    // 計画中
	StatusPlaned     Status = "planed"      // 計画完了
	StatusInProgress Status = "in_progress" // 実行中
	StatusDone       Status = "done"        // 完了
)

// Priority represents the PBI priority
type Priority int

const (
	PriorityNormal Priority = 0 // 通常
	PriorityHigh   Priority = 1 // 高
	PriorityUrgent Priority = 2 // 緊急
)

// NewPBI creates a new PBI with default values
func NewPBI(title string) *PBI {
	now := time.Now()
	return &PBI{
		Title:     title,
		Status:    StatusPending,
		Priority:  PriorityNormal,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate validates the PBI
func (p *PBI) Validate() error {
	if p.Title == "" {
		return errors.New("title is required")
	}

	if p.EstimatedStoryPoints < 0 || p.EstimatedStoryPoints > 13 {
		return errors.New("story points must be between 0 and 13")
	}

	if p.Priority < PriorityNormal || p.Priority > PriorityUrgent {
		return errors.New("priority must be between 0 and 2")
	}

	if !p.Status.IsValid() {
		return fmt.Errorf("invalid status: %s", p.Status)
	}

	return nil
}

// GetMarkdownPath returns the path to the Markdown file
func (p *PBI) GetMarkdownPath() string {
	return filepath.Join(".deespec", "specs", "pbi", p.ID, "pbi.md")
}

// IsValid checks if the status is valid
func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusPlanning, StatusPlaned, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}

// String returns the string representation of Status
func (s Status) String() string {
	return string(s)
}

// String returns the string representation of Priority
func (p Priority) String() string {
	switch p {
	case PriorityNormal:
		return "通常"
	case PriorityHigh:
		return "高"
	case PriorityUrgent:
		return "緊急"
	default:
		return "不明"
	}
}

// UpdateStatus updates the PBI status
func (p *PBI) UpdateStatus(newStatus Status) error {
	if !newStatus.IsValid() {
		return fmt.Errorf("invalid status: %s", newStatus)
	}
	p.Status = newStatus
	p.UpdatedAt = time.Now()
	return nil
}

// UpdateTitle updates the PBI title
func (p *PBI) UpdateTitle(newTitle string) error {
	if newTitle == "" {
		return errors.New("title cannot be empty")
	}
	p.Title = newTitle
	p.UpdatedAt = time.Now()
	return nil
}

// IsCompleted checks if the PBI is completed
func (p *PBI) IsCompleted() bool {
	return p.Status == StatusDone
}

// IsPending checks if the PBI is pending
func (p *PBI) IsPending() bool {
	return p.Status == StatusPending
}

// IsInProgress checks if the PBI is in progress
func (p *PBI) IsInProgress() bool {
	return p.Status == StatusInProgress
}
