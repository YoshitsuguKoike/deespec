package input

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

// TaskUseCase defines the interface for task management use cases
type TaskUseCase interface {
	// CreateEPIC creates a new EPIC task
	CreateEPIC(ctx context.Context, req dto.CreateEPICRequest) (*dto.EPICDTO, error)

	// CreatePBI creates a new PBI task
	CreatePBI(ctx context.Context, req dto.CreatePBIRequest) (*dto.PBIDTO, error)

	// CreateSBI creates a new SBI task
	CreateSBI(ctx context.Context, req dto.CreateSBIRequest) (*dto.SBIDTO, error)

	// GetTask retrieves a task by ID
	GetTask(ctx context.Context, taskID string) (*dto.TaskDTO, error)

	// GetEPIC retrieves an EPIC by ID
	GetEPIC(ctx context.Context, epicID string) (*dto.EPICDTO, error)

	// GetPBI retrieves a PBI by ID
	GetPBI(ctx context.Context, pbiID string) (*dto.PBIDTO, error)

	// GetSBI retrieves an SBI by ID
	GetSBI(ctx context.Context, sbiID string) (*dto.SBIDTO, error)

	// ListTasks lists tasks with filters
	ListTasks(ctx context.Context, req dto.ListTasksRequest) (*dto.ListTasksResponse, error)

	// UpdateTaskStatus updates the status of a task
	UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) error

	// DeleteTask deletes a task
	DeleteTask(ctx context.Context, taskID string) error
}

// WorkflowUseCase defines the interface for workflow operations
type WorkflowUseCase interface {
	// PickTask picks a task for implementation
	PickTask(ctx context.Context, taskID string) error

	// ImplementTask executes the implementation step for a task
	ImplementTask(ctx context.Context, req dto.ImplementTaskRequest) (*dto.ImplementTaskResponse, error)

	// ReviewTask reviews a task implementation
	ReviewTask(ctx context.Context, req dto.ReviewTaskRequest) (*dto.ReviewTaskResponse, error)

	// CompleteTask marks a task as done
	CompleteTask(ctx context.Context, taskID string) error
}

// EPICWorkflowUseCase defines EPIC-specific workflow operations
type EPICWorkflowUseCase interface {
	// DecomposeEPIC decomposes an EPIC into PBIs
	DecomposeEPIC(ctx context.Context, epicID string) (*dto.ImplementTaskResponse, error)

	// ApproveEPICDecomposition approves the generated PBIs and creates them
	ApproveEPICDecomposition(ctx context.Context, epicID string, pbiRequests []dto.CreatePBIRequest) ([]string, error)
}

// PBIWorkflowUseCase defines PBI-specific workflow operations
type PBIWorkflowUseCase interface {
	// DecomposePBI decomposes a PBI into SBIs
	DecomposePBI(ctx context.Context, pbiID string) (*dto.ImplementTaskResponse, error)

	// ApprovePBIDecomposition approves the generated SBIs and creates them
	ApprovePBIDecomposition(ctx context.Context, pbiID string, sbiRequests []dto.CreateSBIRequest) ([]string, error)
}

// SBIWorkflowUseCase defines SBI-specific workflow operations
type SBIWorkflowUseCase interface {
	// GenerateSBICode generates code for an SBI
	GenerateSBICode(ctx context.Context, sbiID string) (*dto.ImplementTaskResponse, error)

	// ApplySBICode applies the generated code to the filesystem
	ApplySBICode(ctx context.Context, sbiID string, artifactPaths []string) error

	// RetrySBIImplementation retries SBI implementation after failure
	RetrySBIImplementation(ctx context.Context, sbiID string) (*dto.ImplementTaskResponse, error)
}
