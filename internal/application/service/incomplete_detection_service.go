package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

// IncompleteDetectionService handles detection of incomplete task instructions
type IncompleteDetectionService struct {
	warnLog func(format string, args ...interface{})
}

// NewIncompleteDetectionService creates a new incomplete detection service
func NewIncompleteDetectionService(warnLog func(format string, args ...interface{})) *IncompleteDetectionService {
	if warnLog == nil {
		warnLog = func(format string, args ...interface{}) {}
	}
	return &IncompleteDetectionService{
		warnLog: warnLog,
	}
}

// DetectIncomplete checks for incomplete instructions in a task
func (s *IncompleteDetectionService) DetectIncomplete(
	ctx context.Context,
	task *dto.SBITaskDTO,
	pickContext *dto.PickContext,
) ([]dto.FBDraft, error) {
	var drafts []dto.FBDraft

	// Check for unresolved dependencies
	if len(task.DependsOn) > 0 {
		for _, depID := range task.DependsOn {
			if !pickContext.CompletedTasks[depID] {
				draft := dto.FBDraft{
					TargetTaskID:  task.ID,
					ReasonCode:    dto.DepUnresolved,
					Title:         fmt.Sprintf("【FB】%s の不完全指示修正", task.ID),
					Summary:       fmt.Sprintf("依存未解決: depends_on=[%s]（未完了）", depID),
					EvidencePaths: []string{},
					CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
				}
				drafts = append(drafts, draft)
			}
		}
	}

	// Check for cyclic dependencies
	if s.detectTaskCycle(task, pickContext.AllTasks) {
		draft := dto.FBDraft{
			TargetTaskID:  task.ID,
			ReasonCode:    dto.DepCycle,
			Title:         fmt.Sprintf("【FB】%s の不完全指示修正", task.ID),
			Summary:       fmt.Sprintf("循環依存検出: %s が依存グラフにサイクルを形成", task.ID),
			EvidencePaths: []string{},
			CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		}
		drafts = append(drafts, draft)
	}

	// Check for missing meta fields
	if task.Title == "" {
		draft := dto.FBDraft{
			TargetTaskID:  task.ID,
			ReasonCode:    dto.MetaMissing,
			Title:         fmt.Sprintf("【FB】%s の不完全指示修正", task.ID),
			Summary:       "meta.yaml の title フィールドが空です",
			EvidencePaths: []string{},
			CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		}
		drafts = append(drafts, draft)
	}

	// Check for invalid paths
	if s.containsInvalidPath(task.SpecPath) {
		draft := dto.FBDraft{
			TargetTaskID:  task.ID,
			ReasonCode:    dto.PathInvalid,
			Title:         fmt.Sprintf("【FB】%s の不完全指示修正", task.ID),
			Summary:       fmt.Sprintf("spec_path に不正な文字が含まれています: %s", task.SpecPath),
			EvidencePaths: []string{},
			CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		}
		drafts = append(drafts, draft)
	}

	return drafts, nil
}

// detectTaskCycle detects if a task has circular dependencies
func (s *IncompleteDetectionService) detectTaskCycle(task *dto.SBITaskDTO, allTasks []*dto.SBITaskDTO) bool {
	// Build task map
	taskMap := make(map[string]*dto.SBITaskDTO)
	for _, t := range allTasks {
		taskMap[t.ID] = t
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	return s.hasCycleDFS(task.ID, taskMap, visited, recStack)
}

// hasCycleDFS performs depth-first search to detect cycles
func (s *IncompleteDetectionService) hasCycleDFS(
	taskID string,
	taskMap map[string]*dto.SBITaskDTO,
	visited, recStack map[string]bool,
) bool {
	if recStack[taskID] {
		// Cycle detected
		return true
	}

	if visited[taskID] {
		// Already visited in another path
		return false
	}

	// Mark as visited and add to recursion stack
	visited[taskID] = true
	recStack[taskID] = true

	// Visit all dependencies
	if task, exists := taskMap[taskID]; exists {
		for _, depID := range task.DependsOn {
			if s.hasCycleDFS(depID, taskMap, visited, recStack) {
				return true
			}
		}
	}

	// Remove from recursion stack
	recStack[taskID] = false
	return false
}

// containsInvalidPath checks if a path contains invalid characters
func (s *IncompleteDetectionService) containsInvalidPath(path string) bool {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return true
	}

	// Check for absolute paths (should be relative)
	if strings.HasPrefix(path, "/") || (len(path) > 1 && path[1] == ':') {
		return true
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return true
	}

	return false
}
