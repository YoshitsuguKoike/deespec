package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// IncompleteDetector defines the interface for detecting incomplete instructions
// This will be implemented by the incomplete detection logic from incomplete.go
type IncompleteDetector interface {
	DetectIncomplete(ctx context.Context, task *dto.SBITaskDTO, pickContext *dto.PickContext) ([]dto.FBDraft, error)
	PersistFBDraft(ctx context.Context, draft dto.FBDraft, sbiDir string) (string, error)
	RecordFBDraftInJournal(ctx context.Context, draft dto.FBDraft, journalPath string, turn int) error
}

// TaskPickerService handles task selection and prioritization logic
type TaskPickerService struct {
	repository         repository.SBITaskRepository
	incompleteDetector IncompleteDetector
	infoLog            func(format string, args ...interface{})
	warnLog            func(format string, args ...interface{})
}

// NewTaskPickerService creates a new task picker service
func NewTaskPickerService(
	repository repository.SBITaskRepository,
	incompleteDetector IncompleteDetector,
	infoLog func(format string, args ...interface{}),
	warnLog func(format string, args ...interface{}),
) *TaskPickerService {
	// Provide no-op functions if not provided
	if infoLog == nil {
		infoLog = func(format string, args ...interface{}) {}
	}
	if warnLog == nil {
		warnLog = func(format string, args ...interface{}) {}
	}

	return &TaskPickerService{
		repository:         repository,
		incompleteDetector: incompleteDetector,
		infoLog:            infoLog,
		warnLog:            warnLog,
	}
}

// PickNextTask selects the next task to process based on priority rules
func (s *TaskPickerService) PickNextTask(ctx context.Context, input dto.TaskPickInput) *dto.TaskPickOutput {
	// Set defaults
	if input.SpecsDir == "" {
		input.SpecsDir = ".deespec/specs/sbi"
	}
	if input.OrderBy == nil {
		// Default order: priority DESC → registered_at ASC → sequence ASC
		// This matches the SQLite ORDER BY clause for consistent task execution order
		input.OrderBy = []string{"priority", "registered_at", "sequence", "id"}
	}

	// Load all tasks
	tasks, err := s.repository.LoadAllTasks(ctx, input.SpecsDir)
	if err != nil {
		return &dto.TaskPickOutput{
			Task:   nil,
			Reason: "",
			Error:  fmt.Errorf("failed to load tasks: %w", err),
		}
	}

	if len(tasks) == 0 {
		return &dto.TaskPickOutput{
			Task:   nil,
			Reason: "no tasks found",
			Error:  nil,
		}
	}

	// Build pick context for incomplete detection
	completedTasks, err := s.repository.GetCompletedTasks(ctx, input.JournalPath)
	if err != nil {
		return &dto.TaskPickOutput{
			Task:   nil,
			Reason: "",
			Error:  fmt.Errorf("failed to get completed tasks: %w", err),
		}
	}

	pickCtx := &dto.PickContext{
		JournalPath:    input.JournalPath,
		CompletedTasks: completedTasks,
		AllTasks:       tasks,
	}

	// Filter to ready tasks (not completed, dependencies met)
	readyTasks := s.filterReadyTasks(tasks, completedTasks)

	if len(readyTasks) == 0 {
		return &dto.TaskPickOutput{
			Task:   nil,
			Reason: "no ready tasks",
			Error:  nil,
		}
	}

	// Sort by priority rules
	s.sortTasksByPriority(readyTasks, input.OrderBy)

	// Pick the first one
	picked := readyTasks[0]

	// Detect incomplete instructions before picking (if detector is available)
	if s.incompleteDetector != nil {
		drafts, err := s.incompleteDetector.DetectIncomplete(ctx, picked, pickCtx)
		if err != nil {
			s.warnLog("Failed to detect incomplete for %s: %v", picked.ID, err)
		}

		// Process detected incomplete instructions
		for _, draft := range drafts {
			s.infoLog("Detected incomplete instruction for %s: %s",
				draft.TargetTaskID, draft.ReasonCode)

			// Persist FB draft to SBI directory
			sbiDir := fmt.Sprintf(".deespec/specs/sbi/%s", draft.TargetTaskID)
			draftPath, err := s.incompleteDetector.PersistFBDraft(ctx, draft, sbiDir)
			if err != nil {
				s.warnLog("Failed to persist FB draft: %v", err)
			} else {
				s.infoLog("FB draft saved to %s", draftPath)

				// Record in journal (use turn 0 for pick phase)
				if err := s.incompleteDetector.RecordFBDraftInJournal(ctx, draft, input.JournalPath, 0); err != nil {
					s.warnLog("Failed to record FB draft in journal: %v", err)
				}
			}
		}
	}

	reason := fmt.Sprintf("picked by %s (POR=%d, priority=%d)",
		strings.Join(input.OrderBy, ","), picked.POR, picked.Priority)

	return &dto.TaskPickOutput{
		Task:   picked,
		Reason: reason,
		Error:  nil,
	}
}

// filterReadyTasks returns tasks that are ready to be executed
func (s *TaskPickerService) filterReadyTasks(tasks []*dto.SBITaskDTO, completedTasks map[string]bool) []*dto.SBITaskDTO {
	// Detect circular dependencies
	inCycle := s.detectCycles(tasks)

	var ready []*dto.SBITaskDTO
	for _, task := range tasks {
		// Skip if already completed
		if completedTasks[task.ID] {
			continue
		}

		// Skip if task is in a dependency cycle
		if inCycle[task.ID] {
			s.warnLog("Task %s is part of a circular dependency, skipping", task.ID)
			continue
		}

		// Check dependencies
		if !task.AreDependenciesMet(completedTasks) {
			// Check for unknown dependencies
			for _, depID := range task.DependsOn {
				if !taskExists(tasks, depID) && !completedTasks[depID] {
					s.warnLog("Task %s depends on unknown task %s, skipping", task.ID, depID)
				}
			}
			continue
		}

		ready = append(ready, task)
	}

	return ready
}

// detectCycles detects circular dependencies in the task graph
func (s *TaskPickerService) detectCycles(tasks []*dto.SBITaskDTO) map[string]bool {
	taskMap := make(map[string]*dto.SBITaskDTO)
	for _, t := range tasks {
		taskMap[t.ID] = t
	}

	inCycle := make(map[string]bool)
	visited := make(map[string]bool)
	visiting := make(map[string]bool)

	var dfs func(taskID string) bool
	dfs = func(taskID string) bool {
		if visiting[taskID] {
			// Found a cycle
			return true
		}
		if visited[taskID] {
			// Already processed
			return false
		}

		visiting[taskID] = true
		task, exists := taskMap[taskID]
		if exists {
			for _, depID := range task.DependsOn {
				if dfs(depID) {
					inCycle[taskID] = true
					return true
				}
			}
		}
		visiting[taskID] = false
		visited[taskID] = true
		return false
	}

	for _, task := range tasks {
		if !visited[task.ID] {
			if dfs(task.ID) {
				s.warnLog("Circular dependency detected involving task %s", task.ID)
			}
		}
	}

	return inCycle
}

// sortTasksByPriority sorts tasks according to the specified order
// IMPORTANT: For SQLite-based tasks, the correct order is:
// 1. priority (DESC - higher priority first)
// 2. registered_at (ASC - earlier registration first)
// 3. sequence (ASC - registration order)
func (s *TaskPickerService) sortTasksByPriority(tasks []*dto.SBITaskDTO, orderBy []string) {
	sort.SliceStable(tasks, func(i, j int) bool {
		for _, criterion := range orderBy {
			switch criterion {
			case "por":
				if tasks[i].POR != tasks[j].POR {
					return tasks[i].POR < tasks[j].POR
				}
			case "priority":
				// Higher priority values should come first (DESC)
				if tasks[i].Priority != tasks[j].Priority {
					return tasks[i].Priority > tasks[j].Priority
				}
			case "registered_at":
				// Earlier registration should come first (ASC)
				if tasks[i].RegisteredAt != tasks[j].RegisteredAt {
					return tasks[i].RegisteredAt.Before(tasks[j].RegisteredAt)
				}
			case "sequence":
				// Lower sequence numbers should come first (ASC)
				if tasks[i].Sequence != tasks[j].Sequence {
					return tasks[i].Sequence < tasks[j].Sequence
				}
			case "id":
				if tasks[i].ID != tasks[j].ID {
					return tasks[i].ID < tasks[j].ID
				}
			}
		}
		return false
	})
}

// Helper function to check if task exists in list
func taskExists(tasks []*dto.SBITaskDTO, id string) bool {
	for _, t := range tasks {
		if t.ID == id {
			return true
		}
	}
	return false
}
