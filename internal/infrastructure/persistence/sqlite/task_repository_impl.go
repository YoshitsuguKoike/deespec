package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// TaskRepositoryImpl implements repository.TaskRepository with SQLite
type TaskRepositoryImpl struct {
	db       *sql.DB
	epicRepo *EPICRepositoryImpl
	// pbiRepo removed: PBI system is being refactored to Markdown + SQLite hybrid
	sbiRepo *SBIRepositoryImpl
}

// NewTaskRepository creates a new SQLite-based task repository
func NewTaskRepository(db *sql.DB) repository.TaskRepository {
	return &TaskRepositoryImpl{
		db:       db,
		epicRepo: &EPICRepositoryImpl{db: db},
		// pbiRepo removed: PBI system is being refactored to Markdown + SQLite hybrid
		sbiRepo: &SBIRepositoryImpl{db: db},
	}
}

// FindByID retrieves a task by ID (polymorphic)
func (r *TaskRepositoryImpl) FindByID(ctx context.Context, id repository.TaskID) (task.Task, error) {
	// 1. Determine task type by querying all tables
	taskType, err := r.determineTaskType(ctx, string(id))
	if err != nil {
		return nil, err
	}

	// 2. Load appropriate task type
	switch taskType {
	case repository.TaskTypeEPIC:
		return r.epicRepo.Find(ctx, repository.EPICID(id))
	case repository.TaskTypePBI:
		return nil, fmt.Errorf("PBI workflow is temporarily disabled - use 'deespec pbi show %s' instead", id)
	case repository.TaskTypeSBI:
		return r.sbiRepo.Find(ctx, repository.SBIID(id))
	default:
		return nil, fmt.Errorf("task not found: %s", id)
	}
}

// Save persists a task entity
func (r *TaskRepositoryImpl) Save(ctx context.Context, t task.Task) error {
	switch t.Type() {
	case model.TaskTypeEPIC:
		epicTask, ok := t.(*epic.EPIC)
		if !ok {
			return fmt.Errorf("type assertion failed: expected *epic.EPIC")
		}
		return r.epicRepo.Save(ctx, epicTask)
	case model.TaskTypePBI:
		// PBI workflow is temporarily disabled
		return fmt.Errorf("PBI workflow is temporarily disabled - use 'deespec pbi' commands instead")
	case model.TaskTypeSBI:
		sbiTask, ok := t.(*sbi.SBI)
		if !ok {
			return fmt.Errorf("type assertion failed: expected *sbi.SBI")
		}
		return r.sbiRepo.Save(ctx, sbiTask)
	default:
		return fmt.Errorf("unknown task type: %s", t.Type())
	}
}

// Delete removes a task
func (r *TaskRepositoryImpl) Delete(ctx context.Context, id repository.TaskID) error {
	// Try deleting from all tables (CASCADE will handle relations)
	queries := []string{
		"DELETE FROM epics WHERE id = ?",
		"DELETE FROM pbis WHERE id = ?",
		"DELETE FROM sbis WHERE id = ?",
	}

	for _, query := range queries {
		result, err := r.db.ExecContext(ctx, query, string(id))
		if err != nil {
			return fmt.Errorf("delete task failed: %w", err)
		}

		if rows, _ := result.RowsAffected(); rows > 0 {
			return nil // Successfully deleted
		}
	}

	return fmt.Errorf("task not found: %s", id)
}

// List retrieves tasks by filter
func (r *TaskRepositoryImpl) List(ctx context.Context, filter repository.TaskFilter) ([]task.Task, error) {
	var tasks []task.Task

	// Query each task type based on filter
	for _, taskType := range r.getTaskTypesToQuery(filter) {
		switch taskType {
		case repository.TaskTypeEPIC:
			epicFilter := repository.EPICFilter{
				Statuses: filter.Statuses,
				Limit:    filter.Limit,
				Offset:   filter.Offset,
			}
			epics, err := r.epicRepo.List(ctx, epicFilter)
			if err != nil {
				return nil, err
			}
			for _, e := range epics {
				tasks = append(tasks, e)
			}
		case repository.TaskTypePBI:
			// PBI workflow is temporarily disabled
			// Use 'deespec pbi list' command instead
			continue
		case repository.TaskTypeSBI:
			sbiFilter := repository.SBIFilter{
				Statuses: convertStatusesToModel(filter.Statuses),
				Limit:    filter.Limit,
				Offset:   filter.Offset,
			}
			sbis, err := r.sbiRepo.List(ctx, sbiFilter)
			if err != nil {
				return nil, err
			}
			for _, s := range sbis {
				tasks = append(tasks, s)
			}
		}
	}

	return tasks, nil
}

// determineTaskType determines which table contains the task ID
func (r *TaskRepositoryImpl) determineTaskType(ctx context.Context, id string) (repository.TaskType, error) {
	queries := map[repository.TaskType]string{
		repository.TaskTypeEPIC: "SELECT 1 FROM epics WHERE id = ?",
		repository.TaskTypePBI:  "SELECT 1 FROM pbis WHERE id = ?",
		repository.TaskTypeSBI:  "SELECT 1 FROM sbis WHERE id = ?",
	}

	for taskType, query := range queries {
		var exists int
		err := r.db.QueryRowContext(ctx, query, id).Scan(&exists)
		if err == nil {
			return taskType, nil
		}
		if err != sql.ErrNoRows {
			return "", fmt.Errorf("query failed: %w", err)
		}
	}

	return "", fmt.Errorf("task not found: %s", id)
}

// getTaskTypesToQuery returns task types to query based on filter
func (r *TaskRepositoryImpl) getTaskTypesToQuery(filter repository.TaskFilter) []repository.TaskType {
	if len(filter.Types) > 0 {
		return filter.Types
	}
	// If no types specified, query all
	return []repository.TaskType{
		repository.TaskTypeEPIC,
		repository.TaskTypePBI,
		repository.TaskTypeSBI,
	}
}

// convertStatusesToModel converts repository.Status to model.Status for SBI filter
func convertStatusesToModel(statuses []repository.Status) []model.Status {
	result := make([]model.Status, len(statuses))
	for i, s := range statuses {
		result[i] = model.Status(s)
	}
	return result
}
