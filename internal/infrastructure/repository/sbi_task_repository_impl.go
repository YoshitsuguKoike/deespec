package repository

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"gopkg.in/yaml.v3"
)

// SBITaskRepositoryImpl implements SBITaskRepository for file-based storage
type SBITaskRepositoryImpl struct {
	// Logging functions (injected from CLI layer)
	warnLog  func(format string, args ...interface{})
	debugLog func(format string, args ...interface{})
	// Database repository for loading dependencies
	sbiRepo repository.SBIRepository
}

// NewSBITaskRepositoryImpl creates a new file-based SBI task repository
func NewSBITaskRepositoryImpl(
	warnLog func(format string, args ...interface{}),
	debugLog func(format string, args ...interface{}),
	sbiRepo repository.SBIRepository,
) repository.SBITaskRepository {
	// Provide no-op functions if not provided
	if warnLog == nil {
		warnLog = func(format string, args ...interface{}) {}
	}
	if debugLog == nil {
		debugLog = func(format string, args ...interface{}) {}
	}

	return &SBITaskRepositoryImpl{
		warnLog:  warnLog,
		debugLog: debugLog,
		sbiRepo:  sbiRepo,
	}
}

// LoadAllTasks loads all tasks from the specs directory
func (r *SBITaskRepositoryImpl) LoadAllTasks(ctx context.Context, specsDir string) ([]*dto.SBITaskDTO, error) {
	var tasks []*dto.SBITaskDTO

	// Check if specs directory exists
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return tasks, nil // Return empty list if no specs dir
	}

	// Walk through the directory
	err := filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			return err
		}

		// Look for meta.yaml or meta.yml files
		if info.Name() == "meta.yaml" || info.Name() == "meta.yml" {
			task, err := r.loadTaskFromMetaFile(path)
			if err != nil {
				// Log warning but continue
				r.warnLog("failed to load task from %s: %v", path, err)
				return nil
			}
			if task != nil {
				tasks = append(tasks, task)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Enrich tasks with database-based dependencies if sbiRepo is available
	if r.sbiRepo != nil {
		for _, task := range tasks {
			deps, err := r.sbiRepo.GetDependencies(ctx, repository.SBIID(task.ID))
			if err != nil {
				r.warnLog("failed to load dependencies for %s from database: %v", task.ID, err)
				// Continue with file-based dependencies
				continue
			}
			// Override with database dependencies if available
			if len(deps) > 0 {
				task.DependsOn = deps
				r.debugLog("loaded %d dependencies for %s from database", len(deps), task.ID)
			}
		}
	}

	return tasks, nil
}

// loadTaskFromMetaFile loads a task from a meta.yaml file
func (r *SBITaskRepositoryImpl) loadTaskFromMetaFile(metaPath string) (*dto.SBITaskDTO, error) {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta dto.TaskMetaYAML
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	// Derive spec path from meta.yaml location
	specDir := filepath.Dir(metaPath)

	task := &dto.SBITaskDTO{
		ID:        meta.ID,
		SpecPath:  specDir,
		Title:     meta.Title,
		Priority:  meta.Priority,
		POR:       meta.POR,
		DependsOn: meta.DependsOn,
		Status:    "PENDING", // Default status
		Meta: map[string]interface{}{
			"phase": meta.Phase,
			"role":  meta.Role,
		},
	}

	// Set defaults
	if task.Priority == 0 {
		task.Priority = 999 // Low priority by default
	}
	if task.POR == 0 {
		task.POR = 999 // Low POR by default
	}

	return task, nil
}

// GetCompletedTasks returns a map of completed task IDs from journal
func (r *SBITaskRepositoryImpl) GetCompletedTasks(ctx context.Context, journalPath string) (map[string]bool, error) {
	completed := make(map[string]bool)

	if journalPath == "" {
		journalPath = ".deespec/var/journal.ndjson"
	}

	data, err := os.ReadFile(journalPath)
	if err != nil {
		// Journal might not exist yet
		return completed, nil
	}

	// Parse NDJSON line by line
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return completed, ctx.Err()
		default:
		}

		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if step, ok := entry["step"].(string); ok && step == "plan" {
			// Output plan step entry for debugging
			r.debugLog("[JOURNAL] step=plan: %s", line)

			// Check for register type in artifacts
			if artifacts, ok := entry["artifacts"].([]interface{}); ok {
				for _, artifact := range artifacts {
					if artifactMap, ok := artifact.(map[string]interface{}); ok {
						if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "register" {
							if taskID, ok := artifactMap["id"].(string); ok {
								r.debugLog("[JOURNAL] Found registered task: %s", taskID)
							}
						}
					}
				}
			}
		}
		if step, ok := entry["step"].(string); ok && step == "implement" {
			// Output implement step entry for debugging
			r.debugLog("[JOURNAL] step=implement: %s", line)

			// Extract decision if present
			if decision, ok := entry["decision"].(string); ok && decision != "" {
				r.debugLog("[JOURNAL] implement decision: %s", decision)
			}
		}
		if step, ok := entry["step"].(string); ok && step == "test" {
			// Output test step entry for debugging
			r.debugLog("[JOURNAL] step=test: %s", line)

			// Extract decision if present
			if decision, ok := entry["decision"].(string); ok && decision != "" {
				r.debugLog("[JOURNAL] test decision: %s", decision)
			}
		}
		if step, ok := entry["step"].(string); ok && step == "review" {
			// Output review step entry for debugging
			r.debugLog("[JOURNAL] step=review: %s", line)

			// Extract decision for review (OK or NEEDS_CHANGES)
			if decision, ok := entry["decision"].(string); ok && decision != "" {
				r.debugLog("[JOURNAL] review decision: %s", decision)

				// Check if task needs rework
				if decision == "NEEDS_CHANGES" {
					if artifacts, ok := entry["artifacts"].([]interface{}); ok {
						for _, artifact := range artifacts {
							if artifactMap, ok := artifact.(map[string]interface{}); ok {
								if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "pick" {
									if taskID, ok := artifactMap["task_id"].(string); ok {
										r.debugLog("[JOURNAL] Task %s needs changes", taskID)
									}
								}
							}
						}
					}
				}
			}
		}
		// Look for done steps (old format)
		if step, ok := entry["step"].(string); ok && step == "done" {
			// Output done step entry for debugging
			r.debugLog("[JOURNAL] step=done: %s", line)

			// Extract task ID from artifacts
			if artifacts, ok := entry["artifacts"].([]interface{}); ok {
				for _, artifact := range artifacts {
					if artifactMap, ok := artifact.(map[string]interface{}); ok {
						if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "pick" {
							if taskID, ok := artifactMap["id"].(string); ok {
								completed[taskID] = true
								r.debugLog("[JOURNAL] Task completed (old format): %s", taskID)
							}
						}
					}
				}
			}
		}

		// Look for DONE status (new format)
		if status, ok := entry["status"].(string); ok && status == "DONE" {
			r.debugLog("[JOURNAL] status=DONE found: %s", line)

			// Extract task ID from artifact paths
			if artifacts, ok := entry["artifacts"].([]interface{}); ok {
				for _, artifact := range artifacts {
					// Check if artifact is a string (file path)
					if artifactPath, ok := artifact.(string); ok {
						// Extract task ID from path like ".deespec/specs/sbi/SBI-XXX/done_N.md"
						if strings.Contains(artifactPath, "/specs/sbi/") {
							parts := strings.Split(artifactPath, "/")
							for i, part := range parts {
								if part == "sbi" && i+1 < len(parts) {
									taskID := parts[i+1]
									if strings.HasPrefix(taskID, "SBI-") {
										completed[taskID] = true
										r.debugLog("[JOURNAL] Task completed (new format from path): %s", taskID)
									}
									break
								}
							}
						}
					}
				}
			}
		}
	}

	return completed, nil
}

// GetLastJournalEntry returns the last entry from the journal
func (r *SBITaskRepositoryImpl) GetLastJournalEntry(ctx context.Context, journalPath string) (map[string]interface{}, error) {
	if journalPath == "" {
		journalPath = ".deespec/var/journal.ndjson"
	}

	data, err := os.ReadFile(journalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return nil, nil
	}

	// Get the last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if lines[i] != "" {
			var entry map[string]interface{}
			if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
				return nil, err
			}
			return entry, nil
		}
	}

	return nil, nil
}

// RecordPickInJournal records the pick event in the journal
func (r *SBITaskRepositoryImpl) RecordPickInJournal(ctx context.Context, task *dto.SBITaskDTO, turn int, journalPath string) error {
	if journalPath == "" {
		journalPath = ".deespec/var/journal.ndjson"
	}

	// Create artifact with required fields according to SBI-PICK-002
	artifact := map[string]interface{}{
		"type":      "pick",
		"task_id":   task.ID, // Primary key (required)
		"id":        task.ID, // Backward compatibility
		"spec_path": task.SpecPath,
	}

	// Add priority fields (null if not set)
	if task.POR > 0 {
		artifact["por"] = task.POR
	} else {
		artifact["por"] = nil
	}

	if task.Priority > 0 {
		artifact["priority"] = task.Priority
	} else {
		artifact["priority"] = nil
	}

	entry := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       turn,
		"step":       "plan",
		"decision":   "PENDING",
		"elapsed_ms": 0,
		"error":      "",
		"artifacts":  []map[string]interface{}{artifact},
	}

	// Serialize to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	// Append to journal with newline
	f, err := os.OpenFile(journalPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
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
