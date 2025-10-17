package workflow

import (
	"context"
	"errors"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/domain/service/strategy"
)

// WorkflowUseCaseImpl implements workflow use cases
type WorkflowUseCaseImpl struct {
	taskRepo         repository.TaskRepository
	epicRepo         repository.EPICRepository
	pbiRepo          repository.PBIRepository
	sbiRepo          repository.SBIRepository
	strategyRegistry *strategy.StrategyRegistry
	txManager        output.TransactionManager
}

// NewWorkflowUseCaseImpl creates a new workflow use case implementation
func NewWorkflowUseCaseImpl(
	taskRepo repository.TaskRepository,
	epicRepo repository.EPICRepository,
	pbiRepo repository.PBIRepository,
	sbiRepo repository.SBIRepository,
	strategyRegistry *strategy.StrategyRegistry,
	txManager output.TransactionManager,
) *WorkflowUseCaseImpl {
	return &WorkflowUseCaseImpl{
		taskRepo:         taskRepo,
		epicRepo:         epicRepo,
		pbiRepo:          pbiRepo,
		sbiRepo:          sbiRepo,
		strategyRegistry: strategyRegistry,
		txManager:        txManager,
	}
}

// PickTask picks a task for implementation
func (uc *WorkflowUseCaseImpl) PickTask(ctx context.Context, taskID string) error {
	id, err := model.NewTaskIDFromString(taskID)
	if err != nil {
		return err
	}

	return uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		task, err := uc.taskRepo.FindByID(txCtx, repository.TaskID(id.String()))
		if err != nil {
			return err
		}

		// Update status to PICKED
		if err := task.UpdateStatus(model.StatusPicked); err != nil {
			return err
		}

		return uc.taskRepo.Save(txCtx, task)
	})
}

// ImplementTask executes the implementation step for a task
func (uc *WorkflowUseCaseImpl) ImplementTask(ctx context.Context, req dto.ImplementTaskRequest) (*dto.ImplementTaskResponse, error) {
	id, err := model.NewTaskIDFromString(req.TaskID)
	if err != nil {
		return nil, err
	}

	// Fetch task
	task, err := uc.taskRepo.FindByID(ctx, repository.TaskID(id.String()))
	if err != nil {
		return nil, err
	}

	// Verify task is in correct state
	if task.Status() != model.StatusPicked && task.Status() != model.StatusImplementing {
		return nil, errors.New("task must be PICKED or IMPLEMENTING to execute implementation")
	}

	// If PICKED, only update status to IMPLEMENTING without executing implementation
	// This prevents duplicate execution: once at pick time and again at implementing time
	if task.Status() == model.StatusPicked {
		err = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
			if err := task.UpdateStatus(model.StatusImplementing); err != nil {
				return err
			}
			if err := task.UpdateStep(model.StepImplement); err != nil {
				return err
			}
			return uc.taskRepo.Save(txCtx, task)
		})
		if err != nil {
			return nil, err
		}

		return &dto.ImplementTaskResponse{
			Success:      true,
			Message:      "Task status updated to IMPLEMENTING. Call ImplementTask again to execute implementation.",
			TaskID:       req.TaskID,
			NextStep:     model.StepImplement.String(),
			Artifacts:    []string{},
			ChildTaskIDs: []string{},
		}, nil
	}

	// Task is already IMPLEMENTING - execute implementation strategy
	result, err := uc.strategyRegistry.ExecuteImplementation(ctx, task)
	if err != nil {
		// Record error and save task state
		_ = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
			return uc.taskRepo.Save(txCtx, task)
		})
		return nil, err
	}

	// Update task based on result
	err = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		if result.Success {
			// Check if this is an SBI with only_implement=true
			// If so, skip review and go directly to DONE
			sbiTask, isSBI := task.(interface{ OnlyImplement() bool })
			if isSBI && sbiTask.OnlyImplement() {
				// Implementation-only workflow: skip review, go to DONE
				if err := task.UpdateStep(model.StepDone); err != nil {
					return err
				}
				if err := task.UpdateStatus(model.StatusDone); err != nil {
					return err
				}
			} else {
				// Full workflow: move to next step (usually Review)
				if err := task.UpdateStep(result.NextStep); err != nil {
					return err
				}
				if result.NextStep == model.StepReview {
					if err := task.UpdateStatus(model.StatusReviewing); err != nil {
						return err
					}
				}
			}
		} else {
			// Implementation failed
			if err := task.UpdateStatus(model.StatusFailed); err != nil {
				return err
			}
		}
		return uc.taskRepo.Save(txCtx, task)
	})
	if err != nil {
		return nil, err
	}

	// Convert artifacts to paths
	artifactPaths := make([]string, len(result.Artifacts))
	for i, artifact := range result.Artifacts {
		artifactPaths[i] = artifact.Path
	}

	// Convert child task IDs to strings
	childTaskIDStrs := make([]string, len(result.ChildTaskIDs))
	for i, childID := range result.ChildTaskIDs {
		childTaskIDStrs[i] = childID.String()
	}

	// Determine actual next step based on only_implement flag
	actualNextStep := result.NextStep
	sbiTask, isSBI := task.(interface{ OnlyImplement() bool })
	if result.Success && isSBI && sbiTask.OnlyImplement() {
		actualNextStep = model.StepDone
	}

	return &dto.ImplementTaskResponse{
		Success:      result.Success,
		Message:      result.Message,
		TaskID:       req.TaskID,
		NextStep:     actualNextStep.String(),
		Artifacts:    artifactPaths,
		ChildTaskIDs: childTaskIDStrs,
	}, nil
}

// ReviewTask reviews a task implementation
func (uc *WorkflowUseCaseImpl) ReviewTask(ctx context.Context, req dto.ReviewTaskRequest) (*dto.ReviewTaskResponse, error) {
	id, err := model.NewTaskIDFromString(req.TaskID)
	if err != nil {
		return nil, err
	}

	var nextStep model.Step
	var message string

	err = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		task, err := uc.taskRepo.FindByID(txCtx, repository.TaskID(id.String()))
		if err != nil {
			return err
		}

		// Verify task is in REVIEWING state
		if task.Status() != model.StatusReviewing {
			return errors.New("task must be in REVIEWING state")
		}

		if req.Approved {
			// Approved: move to DONE
			if err := task.UpdateStatus(model.StatusDone); err != nil {
				return err
			}
			if err := task.UpdateStep(model.StepDone); err != nil {
				return err
			}
			nextStep = model.StepDone
			message = "Task approved and completed"
		} else {
			// Rejected: go back to IMPLEMENTING
			if err := task.UpdateStatus(model.StatusImplementing); err != nil {
				return err
			}
			if err := task.UpdateStep(model.StepImplement); err != nil {
				return err
			}
			nextStep = model.StepImplement
			message = "Task rejected, needs rework: " + req.Feedback
		}

		return uc.taskRepo.Save(txCtx, task)
	})
	if err != nil {
		return nil, err
	}

	return &dto.ReviewTaskResponse{
		Success:  true,
		Message:  message,
		TaskID:   req.TaskID,
		NextStep: nextStep.String(),
	}, nil
}

// CompleteTask marks a task as done
func (uc *WorkflowUseCaseImpl) CompleteTask(ctx context.Context, taskID string) error {
	id, err := model.NewTaskIDFromString(taskID)
	if err != nil {
		return err
	}

	return uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		task, err := uc.taskRepo.FindByID(txCtx, repository.TaskID(id.String()))
		if err != nil {
			return err
		}

		if err := task.UpdateStatus(model.StatusDone); err != nil {
			return err
		}
		if err := task.UpdateStep(model.StepDone); err != nil {
			return err
		}

		return uc.taskRepo.Save(txCtx, task)
	})
}

// DecomposeEPIC decomposes an EPIC into PBIs
func (uc *WorkflowUseCaseImpl) DecomposeEPIC(ctx context.Context, epicID string) (*dto.ImplementTaskResponse, error) {
	epicTask, err := uc.epicRepo.Find(ctx, repository.EPICID(epicID))
	if err != nil {
		return nil, err
	}

	// Execute EPIC decomposition strategy
	result, err := uc.strategyRegistry.ExecuteImplementation(ctx, epicTask)
	if err != nil {
		return nil, err
	}

	// Convert artifacts to paths
	artifactPaths := make([]string, len(result.Artifacts))
	for i, artifact := range result.Artifacts {
		artifactPaths[i] = artifact.Path
	}

	return &dto.ImplementTaskResponse{
		Success:   result.Success,
		Message:   result.Message,
		TaskID:    epicID,
		NextStep:  result.NextStep.String(),
		Artifacts: artifactPaths,
	}, nil
}

// DecomposePBI decomposes a PBI into SBIs
// TEMPORARILY DISABLED: PBI system is being refactored to use Markdown + SQLite hybrid
// Old workflow-based PBI decomposition is deprecated
func (uc *WorkflowUseCaseImpl) DecomposePBI(ctx context.Context, pbiID string) (*dto.ImplementTaskResponse, error) {
	return nil, errors.New("DecomposePBI is temporarily disabled - PBI system is being refactored. Use 'deespec pbi register' to create PBIs")

	// TODO: Re-enable when new PBI system integrates with workflow
	// pbiTask, err := uc.pbiRepo.Find(ctx, repository.PBIID(pbiID))
	// if err != nil {
	// 	return nil, err
	// }
	//
	// // Execute PBI decomposition strategy
	// result, err := uc.strategyRegistry.ExecuteImplementation(ctx, pbiTask)
	// if err != nil {
	// 	return nil, err
	// }
	//
	// // Convert artifacts to paths
	// artifactPaths := make([]string, len(result.Artifacts))
	// for i, artifact := range result.Artifacts {
	// 	artifactPaths[i] = artifact.Path
	// }
	//
	// return &dto.ImplementTaskResponse{
	// 	Success:   result.Success,
	// 	Message:   result.Message,
	// 	TaskID:    pbiID,
	// 	NextStep:  result.NextStep.String(),
	// 	Artifacts: artifactPaths,
	// }, nil
}

// GenerateSBICode generates code for an SBI
func (uc *WorkflowUseCaseImpl) GenerateSBICode(ctx context.Context, sbiID string) (*dto.ImplementTaskResponse, error) {
	sbiTask, err := uc.sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return nil, err
	}

	// Execute SBI code generation strategy
	result, err := uc.strategyRegistry.ExecuteImplementation(ctx, sbiTask)
	if err != nil {
		// Save SBI state (with error recorded)
		_ = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
			return uc.sbiRepo.Save(txCtx, sbiTask)
		})
		return nil, err
	}

	// Save SBI state
	err = uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		return uc.sbiRepo.Save(txCtx, sbiTask)
	})
	if err != nil {
		return nil, err
	}

	// Convert artifacts to paths
	artifactPaths := make([]string, len(result.Artifacts))
	for i, artifact := range result.Artifacts {
		artifactPaths[i] = artifact.Path
	}

	return &dto.ImplementTaskResponse{
		Success:   result.Success,
		Message:   result.Message,
		TaskID:    sbiID,
		NextStep:  result.NextStep.String(),
		Artifacts: artifactPaths,
	}, nil
}

// ApplySBICode applies the generated code to the filesystem
func (uc *WorkflowUseCaseImpl) ApplySBICode(ctx context.Context, sbiID string, artifactPaths []string) error {
	sbiTask, err := uc.sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return err
	}

	// Add artifacts to SBI
	return uc.txManager.InTransaction(ctx, func(txCtx context.Context) error {
		for _, path := range artifactPaths {
			sbiTask.AddArtifact(path)
		}
		return uc.sbiRepo.Save(txCtx, sbiTask)
	})
}

// RetrySBIImplementation retries SBI implementation after failure
func (uc *WorkflowUseCaseImpl) RetrySBIImplementation(ctx context.Context, sbiID string) (*dto.ImplementTaskResponse, error) {
	sbiTask, err := uc.sbiRepo.Find(ctx, repository.SBIID(sbiID))
	if err != nil {
		return nil, err
	}

	// Increment attempt
	sbiTask.IncrementAttempt()

	// Execute implementation again
	return uc.GenerateSBICode(ctx, sbiID)
}
