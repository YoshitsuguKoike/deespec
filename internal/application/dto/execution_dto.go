package dto

import "time"

// RunTurnInput represents input for running a single turn
type RunTurnInput struct {
	AutoFB bool `json:"auto_fb"` // Automatically register FB-SBI drafts
}

// RunTurnOutput represents the result of running a single turn
type RunTurnOutput struct {
	// Basic info
	Turn        int       `json:"turn"`
	SBIID       string    `json:"sbi_id,omitempty"`       // Current SBI being processed (empty if no WIP)
	NoOp        bool      `json:"no_op"`                  // True if no work was done
	NoOpReason  string    `json:"no_op_reason,omitempty"` // Reason for NoOp: "lock_held", "no_tasks", or empty
	ElapsedMs   int64     `json:"elapsed_ms"`             // Execution time
	CompletedAt time.Time `json:"completed_at"`

	// State transition
	PrevStatus string `json:"prev_status,omitempty"` // Status before this turn
	NextStatus string `json:"next_status,omitempty"` // Status after this turn
	PrevStep   string `json:"prev_step,omitempty"`   // Step before this turn
	NextStep   string `json:"next_step,omitempty"`   // Step after this turn
	Decision   string `json:"decision,omitempty"`    // Review decision (SUCCEEDED, NEEDS_CHANGES, FAILED)
	Attempt    int    `json:"attempt,omitempty"`     // Current attempt number

	// Results
	ArtifactPath string `json:"artifact_path,omitempty"` // Main artifact path
	ErrorMsg     string `json:"error_msg,omitempty"`     // Error message if any

	// Task lifecycle events
	TaskPicked    bool `json:"task_picked"`    // True if a new task was picked
	TaskCompleted bool `json:"task_completed"` // True if task completed this turn
}

// ExecutionStateDTO represents the current execution state
type ExecutionStateDTO struct {
	WIP            string            `json:"wip"`              // Work in progress SBI ID
	Status         string            `json:"status"`           // Current status
	CurrentStep    string            `json:"current_step"`     // Current workflow step
	Turn           int               `json:"turn"`             // Current turn number
	Attempt        int               `json:"attempt"`          // Current attempt number
	Decision       string            `json:"decision"`         // Latest decision
	LeaseExpiresAt string            `json:"lease_expires_at"` // Lease expiration time
	Inputs         map[string]string `json:"inputs"`           // Input parameters
	LastArtifacts  map[string]string `json:"last_artifacts"`   // Last artifact paths by step
	Version        int               `json:"version"`          // State version for optimistic locking
}

// PickTaskOutput represents the result of picking a task
type PickTaskOutput struct {
	Picked bool   `json:"picked"`           // True if a task was picked
	SBIID  string `json:"sbi_id,omitempty"` // Picked SBI ID
	Title  string `json:"title,omitempty"`  // Task title
	Reason string `json:"reason"`           // Reason for pick or no-pick
}

// ExecuteStepInput represents input for executing a workflow step
type ExecuteStepInput struct {
	SBIID   string `json:"sbi_id" validate:"required"`
	Step    string `json:"step" validate:"required"` // implement, review, etc.
	Turn    int    `json:"turn" validate:"required"`
	Attempt int    `json:"attempt" validate:"required"`
	Prompt  string `json:"prompt" validate:"required"`
}

// ExecuteStepOutput represents the result of executing a workflow step
type ExecuteStepOutput struct {
	Success      bool      `json:"success"`
	Output       string    `json:"output"`              // AI agent output
	Decision     string    `json:"decision,omitempty"`  // Extracted decision (for review steps)
	ArtifactPath string    `json:"artifact_path"`       // Saved artifact path
	ErrorMsg     string    `json:"error_msg,omitempty"` // Error message if any
	ElapsedMs    int64     `json:"elapsed_ms"`          // Execution time
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}
