package task

import (
	"context"
	"errors"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/domain/factory"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	domaintask "github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// TaskUseCaseImpl implements the TaskUseCase interface
type TaskUseCaseImpl struct {
	taskRepo    repository.TaskRepository
	epicRepo    repository.EPICRepository
	pbiRepo     repository.PBIRepository
	sbiRepo     repository.SBIRepository
	taskFactory *factory.Factory
	txManager   output.TransactionManager
}

// NewTaskUseCaseImpl creates a new task use case implementation
func NewTaskUseCaseImpl(
	taskRepo repository.TaskRepository,
	epicRepo repository.EPICRepository,
	pbiRepo repository.PBIRepository,
	sbiRepo repository.SBIRepository,
	taskFactory *factory.Factory,
	txManager output.TransactionManager,
) *TaskUseCaseImpl {
	return &TaskUseCaseImpl{
		taskRepo:    taskRepo,
		epicRepo:    epicRepo,
		pbiRepo:     pbiRepo,
		sbiRepo:     sbiRepo,
		taskFactory: taskFactory,
		txManager:   txManager,
	}
}

// CreateEPIC creates a new EPIC task
func (uc *TaskUseCaseImpl) CreateEPIC(ctx context.Context, req dto.CreateEPICRequest) (*dto.EPICDTO, error) {
	// Validate request
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	// Create EPIC using factory
	epicTask, err := uc.taskFactory.CreateEPIC(
		req.Title,
		req.Description,
		epic.EPICMetadata{
			EstimatedStoryPoints: req.EstimatedStoryPoints,
			Priority:             req.Priority,
			Labels:               req.Labels,
			AssignedAgent:        req.AssignedAgent,
		},
	)
	if err != nil {
		return nil, err
	}

	// Save EPIC in transaction
	err = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		return uc.epicRepo.Save(txCtx, epicTask)
	})
	if err != nil {
		return nil, err
	}

	// Convert to DTO
	return uc.epicToDTO(epicTask), nil
}

// CreatePBI creates a new PBI task
func (uc *TaskUseCaseImpl) CreatePBI(ctx context.Context, req dto.CreatePBIRequest) (*dto.PBIDTO, error) {
	// Validate request
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	// Convert parent EPIC ID if provided
	var parentEPICID *model.TaskID
	if req.ParentEPICID != nil {
		id, err := model.NewTaskIDFromString(*req.ParentEPICID)
		if err != nil {
			return nil, err
		}
		parentEPICID = &id

		// Verify parent EPIC exists
		_, err = uc.epicRepo.Find(ctx, repository.EPICID(*req.ParentEPICID))
		if err != nil {
			return nil, errors.New("parent EPIC not found")
		}
	}

	// Create PBI using factory
	pbiTask, err := uc.taskFactory.CreatePBI(
		req.Title,
		req.Description,
		parentEPICID,
		pbi.PBIMetadata{
			StoryPoints:        req.StoryPoints,
			Priority:           req.Priority,
			Labels:             req.Labels,
			AssignedAgent:      req.AssignedAgent,
			AcceptanceCriteria: req.AcceptanceCriteria,
		},
	)
	if err != nil {
		return nil, err
	}

	// Save PBI and update parent EPIC in transaction
	err = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		if err := uc.pbiRepo.Save(txCtx, pbiTask); err != nil {
			return err
		}

		// If has parent EPIC, add this PBI to it
		if parentEPICID != nil {
			parentEPIC, err := uc.epicRepo.Find(txCtx, repository.EPICID(parentEPICID.String()))
			if err != nil {
				return err
			}
			if err := parentEPIC.AddPBI(pbiTask.ID()); err != nil {
				return err
			}
			if err := uc.epicRepo.Save(txCtx, parentEPIC); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert to DTO
	return uc.pbiToDTO(pbiTask), nil
}

// CreateSBI creates a new SBI task
func (uc *TaskUseCaseImpl) CreateSBI(ctx context.Context, req dto.CreateSBIRequest) (*dto.SBIDTO, error) {
	// Validate request
	if req.Title == "" {
		return nil, errors.New("title is required")
	}

	// Convert parent PBI ID if provided
	var parentPBIID *model.TaskID
	if req.ParentPBIID != nil {
		id, err := model.NewTaskIDFromString(*req.ParentPBIID)
		if err != nil {
			return nil, err
		}
		parentPBIID = &id

		// Verify parent PBI exists
		_, err = uc.pbiRepo.Find(ctx, repository.PBIID(*req.ParentPBIID))
		if err != nil {
			return nil, errors.New("parent PBI not found")
		}
	}

	// Create SBI using factory
	sbiTask, err := uc.taskFactory.CreateSBI(
		req.Title,
		req.Description,
		parentPBIID,
		sbi.SBIMetadata{
			EstimatedHours: req.EstimatedHours,
			Priority:       req.Priority,
			Labels:         req.Labels,
			AssignedAgent:  req.AssignedAgent,
			FilePaths:      req.FilePaths,
		},
	)
	if err != nil {
		return nil, err
	}

	// Set custom limits if provided
	if req.MaxTurns != nil {
		sbiTask.SetMaxTurns(*req.MaxTurns)
	}
	if req.MaxAttempts != nil {
		sbiTask.SetMaxAttempts(*req.MaxAttempts)
	}

	// Save SBI and update parent PBI in transaction
	err = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		if err := uc.sbiRepo.Save(txCtx, sbiTask); err != nil {
			return err
		}

		// If has parent PBI, add this SBI to it
		if parentPBIID != nil {
			parentPBI, err := uc.pbiRepo.Find(txCtx, repository.PBIID(parentPBIID.String()))
			if err != nil {
				return err
			}
			if err := parentPBI.AddSBI(sbiTask.ID()); err != nil {
				return err
			}
			if err := uc.pbiRepo.Save(txCtx, parentPBI); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert to DTO
	return uc.sbiToDTO(sbiTask), nil
}

// GetTask retrieves a task by ID (polymorphic)
func (uc *TaskUseCaseImpl) GetTask(ctx context.Context, taskID string) (*dto.TaskDTO, error) {
	id, err := model.NewTaskIDFromString(taskID)
	if err != nil {
		return nil, err
	}

	task, err := uc.taskRepo.FindByID(ctx, repository.TaskID(id.String()))
	if err != nil {
		return nil, err
	}

	return uc.taskToDTO(task), nil
}

// GetEPIC retrieves an EPIC by ID
func (uc *TaskUseCaseImpl) GetEPIC(ctx context.Context, epicID string) (*dto.EPICDTO, error) {
	epicTask, err := uc.epicRepo.Find(ctx, repository.EPICID(epicID))
	if err != nil {
		return nil, err
	}

	return uc.epicToDTO(epicTask), nil
}

// GetPBI retrieves a PBI by ID
func (uc *TaskUseCaseImpl) GetPBI(ctx context.Context, pbiID string) (*dto.PBIDTO, error) {
	pbiTask, err := uc.pbiRepo.Find(ctx, repository.PBIID(pbiID))
	if err != nil {
		return nil, err
	}

	return uc.pbiToDTO(pbiTask), nil
}

// GetSBI retrieves an SBI by ID
func (uc *TaskUseCaseImpl) GetSBI(ctx context.Context, sbiID string) (*dto.SBIDTO, error) {
	sbiTask, err := uc.sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return nil, err
	}

	return uc.sbiToDTO(sbiTask), nil
}

// ListTasks lists tasks with filters
func (uc *TaskUseCaseImpl) ListTasks(ctx context.Context, req dto.ListTasksRequest) (*dto.ListTasksResponse, error) {
	// Convert request to repository filter
	filter := repository.TaskFilter{
		Limit:  req.Limit,
		Offset: req.Offset,
	}

	// Convert types
	if len(req.Types) > 0 {
		filter.Types = make([]repository.TaskType, len(req.Types))
		for i, t := range req.Types {
			filter.Types[i] = repository.TaskType(t)
		}
	}

	// Convert statuses
	if len(req.Statuses) > 0 {
		filter.Statuses = make([]repository.Status, len(req.Statuses))
		for i, s := range req.Statuses {
			filter.Statuses[i] = repository.Status(s)
		}
	}

	// Convert parent ID
	if req.ParentID != nil {
		id, err := model.NewTaskIDFromString(*req.ParentID)
		if err != nil {
			return nil, err
		}
		taskID := repository.TaskID(id.String())
		filter.ParentID = &taskID
	}

	filter.HasParent = req.HasParent

	// Fetch tasks
	tasks, err := uc.taskRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Convert to DTOs
	taskDTOs := make([]dto.TaskDTO, len(tasks))
	for i, task := range tasks {
		taskDTOs[i] = *uc.taskToDTO(task)
	}

	return &dto.ListTasksResponse{
		Tasks:      taskDTOs,
		TotalCount: len(taskDTOs),
		Limit:      req.Limit,
		Offset:     req.Offset,
	}, nil
}

// UpdateTaskStatus updates the status of a task
func (uc *TaskUseCaseImpl) UpdateTaskStatus(ctx context.Context, taskID string, newStatus string) error {
	id, err := model.NewTaskIDFromString(taskID)
	if err != nil {
		return err
	}

	status := model.Status(newStatus)
	if !status.IsValid() {
		return errors.New("invalid status")
	}

	return uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		task, err := uc.taskRepo.FindByID(txCtx, repository.TaskID(id.String()))
		if err != nil {
			return err
		}

		if err := task.UpdateStatus(status); err != nil {
			return err
		}

		return uc.taskRepo.Save(txCtx, task)
	})
}

// DeleteTask deletes a task
func (uc *TaskUseCaseImpl) DeleteTask(ctx context.Context, taskID string) error {
	id, err := model.NewTaskIDFromString(taskID)
	if err != nil {
		return err
	}

	return uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		return uc.taskRepo.Delete(txCtx, repository.TaskID(id.String()))
	})
}

// Helper methods for DTO conversion
func (uc *TaskUseCaseImpl) taskToDTO(t domaintask.Task) *dto.TaskDTO {
	var parentID *string
	if t.ParentTaskID() != nil {
		pid := t.ParentTaskID().String()
		parentID = &pid
	}

	return &dto.TaskDTO{
		ID:          t.ID().String(),
		Type:        t.Type().String(),
		Title:       t.Title(),
		Description: t.Description(),
		Status:      t.Status().String(),
		CurrentStep: t.CurrentStep().String(),
		ParentID:    parentID,
		CreatedAt:   t.CreatedAt().Value(),
		UpdatedAt:   t.UpdatedAt().Value(),
	}
}

func (uc *TaskUseCaseImpl) epicToDTO(epicTask *epic.EPIC) *dto.EPICDTO {
	metadata := epicTask.Metadata()
	pbiIDs := epicTask.PBIIDs()
	pbiIDStrs := make([]string, len(pbiIDs))
	for i, id := range pbiIDs {
		pbiIDStrs[i] = id.String()
	}

	return &dto.EPICDTO{
		TaskDTO: dto.TaskDTO{
			ID:          epicTask.ID().String(),
			Type:        epicTask.Type().String(),
			Title:       epicTask.Title(),
			Description: epicTask.Description(),
			Status:      epicTask.Status().String(),
			CurrentStep: epicTask.CurrentStep().String(),
			ParentID:    nil, // EPIC has no parent
			CreatedAt:   epicTask.CreatedAt().Value(),
			UpdatedAt:   epicTask.UpdatedAt().Value(),
		},
		EstimatedStoryPoints: metadata.EstimatedStoryPoints,
		Priority:             metadata.Priority,
		Labels:               metadata.Labels,
		AssignedAgent:        metadata.AssignedAgent,
		PBIIDs:               pbiIDStrs,
		PBICount:             epicTask.PBICount(),
	}
}

func (uc *TaskUseCaseImpl) pbiToDTO(pbiTask *pbi.PBI) *dto.PBIDTO {
	metadata := pbiTask.Metadata()
	sbiIDs := pbiTask.SBIIDs()
	sbiIDStrs := make([]string, len(sbiIDs))
	for i, id := range sbiIDs {
		sbiIDStrs[i] = id.String()
	}

	var parentID *string
	if pbiTask.ParentTaskID() != nil {
		pid := pbiTask.ParentTaskID().String()
		parentID = &pid
	}

	return &dto.PBIDTO{
		TaskDTO: dto.TaskDTO{
			ID:          pbiTask.ID().String(),
			Type:        pbiTask.Type().String(),
			Title:       pbiTask.Title(),
			Description: pbiTask.Description(),
			Status:      pbiTask.Status().String(),
			CurrentStep: pbiTask.CurrentStep().String(),
			ParentID:    parentID,
			CreatedAt:   pbiTask.CreatedAt().Value(),
			UpdatedAt:   pbiTask.UpdatedAt().Value(),
		},
		StoryPoints:        metadata.StoryPoints,
		Priority:           metadata.Priority,
		Labels:             metadata.Labels,
		AssignedAgent:      metadata.AssignedAgent,
		AcceptanceCriteria: metadata.AcceptanceCriteria,
		SBIIDs:             sbiIDStrs,
		SBICount:           pbiTask.SBICount(),
	}
}

func (uc *TaskUseCaseImpl) sbiToDTO(sbiTask *sbi.SBI) *dto.SBIDTO {
	metadata := sbiTask.Metadata()
	execState := sbiTask.ExecutionState()

	var parentID *string
	if sbiTask.ParentTaskID() != nil {
		pid := sbiTask.ParentTaskID().String()
		parentID = &pid
	}

	return &dto.SBIDTO{
		TaskDTO: dto.TaskDTO{
			ID:          sbiTask.ID().String(),
			Type:        sbiTask.Type().String(),
			Title:       sbiTask.Title(),
			Description: sbiTask.Description(),
			Status:      sbiTask.Status().String(),
			CurrentStep: sbiTask.CurrentStep().String(),
			ParentID:    parentID,
			CreatedAt:   sbiTask.CreatedAt().Value(),
			UpdatedAt:   sbiTask.UpdatedAt().Value(),
		},
		EstimatedHours: metadata.EstimatedHours,
		Priority:       metadata.Priority,
		Labels:         metadata.Labels,
		AssignedAgent:  metadata.AssignedAgent,
		FilePaths:      metadata.FilePaths,
		CurrentTurn:    execState.CurrentTurn.Value(),
		CurrentAttempt: execState.CurrentAttempt.Value(),
		MaxTurns:       execState.MaxTurns,
		MaxAttempts:    execState.MaxAttempts,
		LastError:      execState.LastError,
		ArtifactPaths:  execState.ArtifactPaths,
	}
}
