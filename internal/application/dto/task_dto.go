package dto

import "time"

// TaskDTO represents a task in data transfer format
type TaskDTO struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // "EPIC", "PBI", "SBI"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	CurrentStep string    `json:"current_step"`
	ParentID    *string   `json:"parent_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EPICDTO represents an EPIC with specific metadata
type EPICDTO struct {
	TaskDTO
	EstimatedStoryPoints int      `json:"estimated_story_points"`
	Priority             int      `json:"priority"`
	Labels               []string `json:"labels"`
	AssignedAgent        string   `json:"assigned_agent"`
	PBIIDs               []string `json:"pbi_ids"`
	PBICount             int      `json:"pbi_count"`
}

// PBIDTO represents a PBI with specific metadata
type PBIDTO struct {
	TaskDTO
	StoryPoints        int      `json:"story_points"`
	Priority           int      `json:"priority"`
	Labels             []string `json:"labels"`
	AssignedAgent      string   `json:"assigned_agent"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	SBIIDs             []string `json:"sbi_ids"`
	SBICount           int      `json:"sbi_count"`
}

// SBIDTO represents an SBI with specific metadata and execution state
type SBIDTO struct {
	TaskDTO
	EstimatedHours float64   `json:"estimated_hours"`
	Priority       int       `json:"priority"`
	Sequence       int       `json:"sequence"`       // Registration sequence number (auto-incremented)
	RegisteredAt   time.Time `json:"registered_at"`  // Explicit registration timestamp
	Labels         []string  `json:"labels"`
	AssignedAgent  string    `json:"assigned_agent"`
	FilePaths      []string  `json:"file_paths"`

	// Execution state
	CurrentTurn    int      `json:"current_turn"`
	CurrentAttempt int      `json:"current_attempt"`
	MaxTurns       int      `json:"max_turns"`
	MaxAttempts    int      `json:"max_attempts"`
	LastError      string   `json:"last_error,omitempty"`
	ArtifactPaths  []string `json:"artifact_paths"`
}

// CreateEPICRequest represents a request to create an EPIC
type CreateEPICRequest struct {
	Title                string   `json:"title" validate:"required"`
	Description          string   `json:"description"`
	EstimatedStoryPoints int      `json:"estimated_story_points"`
	Priority             int      `json:"priority"`
	Labels               []string `json:"labels"`
	AssignedAgent        string   `json:"assigned_agent"`
}

// CreatePBIRequest represents a request to create a PBI
type CreatePBIRequest struct {
	Title              string   `json:"title" validate:"required"`
	Description        string   `json:"description"`
	ParentEPICID       *string  `json:"parent_epic_id,omitempty"`
	StoryPoints        int      `json:"story_points"`
	Priority           int      `json:"priority"`
	Labels             []string `json:"labels"`
	AssignedAgent      string   `json:"assigned_agent"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
}

// CreateSBIRequest represents a request to create an SBI
type CreateSBIRequest struct {
	Title          string   `json:"title" validate:"required"`
	Description    string   `json:"description"`
	ParentPBIID    *string  `json:"parent_pbi_id,omitempty"`
	EstimatedHours float64  `json:"estimated_hours"`
	Priority       int      `json:"priority"`
	Labels         []string `json:"labels"`
	AssignedAgent  string   `json:"assigned_agent"`
	FilePaths      []string `json:"file_paths"`
	MaxTurns       *int     `json:"max_turns,omitempty"`
	MaxAttempts    *int     `json:"max_attempts,omitempty"`
}

// ListTasksRequest represents a request to list tasks
type ListTasksRequest struct {
	Types     []string `json:"types,omitempty"`      // Filter by task types
	Statuses  []string `json:"statuses,omitempty"`   // Filter by statuses
	ParentID  *string  `json:"parent_id,omitempty"`  // Filter by parent
	HasParent *bool    `json:"has_parent,omitempty"` // Filter by parent existence
	Limit     int      `json:"limit"`
	Offset    int      `json:"offset"`
}

// ListTasksResponse represents a response with task list
type ListTasksResponse struct {
	Tasks      []TaskDTO `json:"tasks"`
	TotalCount int       `json:"total_count"`
	Limit      int       `json:"limit"`
	Offset     int       `json:"offset"`
}

// ImplementTaskRequest represents a request to implement a task
type ImplementTaskRequest struct {
	TaskID string `json:"task_id" validate:"required"`
}

// ImplementTaskResponse represents the result of task implementation
type ImplementTaskResponse struct {
	Success      bool     `json:"success"`
	Message      string   `json:"message"`
	TaskID       string   `json:"task_id"`
	NextStep     string   `json:"next_step"`
	Artifacts    []string `json:"artifacts"`
	ChildTaskIDs []string `json:"child_task_ids,omitempty"` // For EPIC/PBI decomposition
}

// ReviewTaskRequest represents a request to review a task
type ReviewTaskRequest struct {
	TaskID   string `json:"task_id" validate:"required"`
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback,omitempty"`
}

// ReviewTaskResponse represents the result of task review
type ReviewTaskResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	TaskID   string `json:"task_id"`
	NextStep string `json:"next_step"`
}
