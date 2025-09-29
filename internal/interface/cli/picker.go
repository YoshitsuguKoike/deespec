package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Task represents a task loaded from the SBI directory
type Task struct {
	ID         string                 `json:"id"`
	SpecPath   string                 `json:"spec_path"`
	Title      string                 `json:"title"`
	Priority   int                    `json:"priority"`
	POR        int                    `json:"por"`         // Priority of Requirements
	DependsOn  []string               `json:"depends_on"`  // Task dependencies
	Meta       map[string]interface{} `json:"meta"`        // Metadata from meta.yaml
	Status     string                 `json:"status"`      // Current status from journal
	PromptPath string                 `json:"prompt_path"` // Path to prompt file
}

// TaskMeta represents the structure of meta.yaml in task directories
type TaskMeta struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Priority  int      `yaml:"priority"`
	POR       int      `yaml:"por"`
	DependsOn []string `yaml:"depends_on"`
	Phase     string   `yaml:"phase"`
	Role      string   `yaml:"role"`
}

// PickConfig represents configuration for task picking
type PickConfig struct {
	SpecsDir    string   // Base directory for specs (default: .deespec/specs/sbi)
	JournalPath string   // Path to journal file
	OrderBy     []string // Sort order: ["por", "priority", "id"]
	StderrLevel string   // Log level
}

// PickNextTask selects the next task to process based on priority rules
func PickNextTask(cfg PickConfig) (*Task, string, error) {
	if cfg.SpecsDir == "" {
		cfg.SpecsDir = ".deespec/specs/sbi"
	}
	if cfg.OrderBy == nil {
		cfg.OrderBy = []string{"por", "priority", "id"}
	}

	// Load all tasks
	tasks, err := loadAllTasks(cfg.SpecsDir)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load tasks: %w", err)
	}

	if len(tasks) == 0 {
		return nil, "no tasks found", nil
	}

	// Build pick context for incomplete detection
	completedTasks := getCompletedTasksFromJournal(cfg.JournalPath)
	pickCtx := &PickContext{
		JournalPath:    cfg.JournalPath,
		CompletedTasks: completedTasks,
		AllTasks:       tasks,
	}

	// Filter to ready tasks (not completed, dependencies met)
	readyTasks := filterReadyTasks(tasks, cfg.JournalPath)

	if len(readyTasks) == 0 {
		return nil, "no ready tasks", nil
	}

	// Sort by priority rules
	sortTasksByPriority(readyTasks, cfg.OrderBy)

	// Pick the first one
	picked := readyTasks[0]

	// Detect incomplete instructions before picking
	drafts, err := DetectIncomplete(picked, pickCtx)
	if err != nil {
		Warn("Failed to detect incomplete for %s: %v", picked.ID, err)
	}

	// Process detected incomplete instructions
	for _, draft := range drafts {
		Info("Detected incomplete instruction for %s: %s",
			draft.TargetTaskID, draft.ReasonCode)

		// Persist FB draft to SBI directory
		sbiDir := filepath.Join(".deespec", "specs", "sbi", draft.TargetTaskID)
		draftPath, err := PersistFBDraft(draft, sbiDir)
		if err != nil {
			Warn("Failed to persist FB draft: %v", err)
		} else {
			Info("FB draft saved to %s", draftPath)

			// Record in journal (use turn 0 for pick phase)
			if err := RecordFBDraftInJournal(draft, cfg.JournalPath, 0); err != nil {
				Warn("Failed to record FB draft in journal: %v", err)
			}
		}
	}

	reason := fmt.Sprintf("picked by %s (POR=%d, priority=%d)",
		strings.Join(cfg.OrderBy, ","), picked.POR, picked.Priority)

	return picked, reason, nil
}

// loadAllTasks scans the specs directory for tasks
func loadAllTasks(specsDir string) ([]*Task, error) {
	var tasks []*Task

	// Check if specs directory exists
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		return tasks, nil // Return empty list if no specs dir
	}

	// Walk through the directory
	err := filepath.Walk(specsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Look for meta.yaml or meta.yml files
		if info.Name() == "meta.yaml" || info.Name() == "meta.yml" {
			task, err := loadTaskFromMetaFile(path)
			if err != nil {
				// Log warning but continue
				Warn("failed to load task from %s: %v", path, err)
				return nil
			}
			if task != nil {
				tasks = append(tasks, task)
			}
		}

		return nil
	})

	return tasks, err
}

// loadTaskFromMetaFile loads a task from a meta.yaml file
func loadTaskFromMetaFile(metaPath string) (*Task, error) {
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta TaskMeta
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	// Derive spec path from meta.yaml location
	specDir := filepath.Dir(metaPath)

	task := &Task{
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

// filterReadyTasks returns tasks that are ready to be executed
func filterReadyTasks(tasks []*Task, journalPath string) []*Task {
	// Load completed tasks from journal
	completedTasks := getCompletedTasksFromJournal(journalPath)

	// Detect circular dependencies
	inCycle := detectCycles(tasks)

	var ready []*Task
	for _, task := range tasks {
		// Skip if already completed
		if completedTasks[task.ID] {
			continue
		}

		// Skip if task is in a dependency cycle
		if inCycle[task.ID] {
			Warn("Task %s is part of a circular dependency, skipping", task.ID)
			continue
		}

		// Check dependencies
		if !areDepenciesMet(task, completedTasks) {
			// Check for unknown dependencies
			for _, depID := range task.DependsOn {
				if !taskExists(tasks, depID) && !completedTasks[depID] {
					Warn("Task %s depends on unknown task %s, skipping", task.ID, depID)
				}
			}
			continue
		}

		ready = append(ready, task)
	}

	return ready
}

// taskExists checks if a task ID exists in the task list
func taskExists(tasks []*Task, id string) bool {
	for _, t := range tasks {
		if t.ID == id {
			return true
		}
	}
	return false
}

// getCompletedTasksFromJournal reads the journal to find completed tasks
func getCompletedTasksFromJournal(journalPath string) map[string]bool {
	completed := make(map[string]bool)

	if journalPath == "" {
		journalPath = ".deespec/var/journal.ndjson"
	}

	data, err := os.ReadFile(journalPath)
	if err != nil {
		// Journal might not exist yet
		return completed
	}

	// Parse NDJSON line by line
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		if step, ok := entry["step"].(string); ok && step == "plan" {
			// Output plan step entry for debugging
			Debug("[JOURNAL] step=plan: %s", line)

			// Check for register type in artifacts
			if artifacts, ok := entry["artifacts"].([]interface{}); ok {
				for _, artifact := range artifacts {
					if artifactMap, ok := artifact.(map[string]interface{}); ok {
						if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "register" {
							if taskID, ok := artifactMap["id"].(string); ok {
								Debug("[JOURNAL] Found registered task: %s", taskID)
							}
						}
					}
				}
			}
		}
		if step, ok := entry["step"].(string); ok && step == "implement" {
			// Output implement step entry for debugging
			Debug("[JOURNAL] step=implement: %s", line)

			// Extract decision if present
			if decision, ok := entry["decision"].(string); ok && decision != "" {
				Debug("[JOURNAL] implement decision: %s", decision)
			}
		}
		if step, ok := entry["step"].(string); ok && step == "test" {
			// Output test step entry for debugging
			Debug("[JOURNAL] step=test: %s", line)

			// Extract decision if present
			if decision, ok := entry["decision"].(string); ok && decision != "" {
				Debug("[JOURNAL] test decision: %s", decision)
			}
		}
		if step, ok := entry["step"].(string); ok && step == "review" {
			// Output review step entry for debugging
			Debug("[JOURNAL] step=review: %s", line)

			// Extract decision for review (OK or NEEDS_CHANGES)
			if decision, ok := entry["decision"].(string); ok && decision != "" {
				Debug("[JOURNAL] review decision: %s", decision)

				// Check if task needs rework
				if decision == "NEEDS_CHANGES" {
					if artifacts, ok := entry["artifacts"].([]interface{}); ok {
						for _, artifact := range artifacts {
							if artifactMap, ok := artifact.(map[string]interface{}); ok {
								if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "pick" {
									if taskID, ok := artifactMap["task_id"].(string); ok {
										Debug("[JOURNAL] Task %s needs changes", taskID)
									}
								}
							}
						}
					}
				}
			}
		}
		// Look for done steps
		if step, ok := entry["step"].(string); ok && step == "done" {
			// Output done step entry for debugging
			Debug("[JOURNAL] step=done: %s", line)

			// Extract task ID from artifacts
			if artifacts, ok := entry["artifacts"].([]interface{}); ok {
				for _, artifact := range artifacts {
					if artifactMap, ok := artifact.(map[string]interface{}); ok {
						if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "pick" {
							if taskID, ok := artifactMap["id"].(string); ok {
								completed[taskID] = true
								Debug("[JOURNAL] Task completed: %s", taskID)
							}
						}
					}
				}
			}
		}
	}

	return completed
}

// areDepenciesMet checks if all dependencies for a task are completed
func areDepenciesMet(task *Task, completedTasks map[string]bool) bool {
	for _, depID := range task.DependsOn {
		if !completedTasks[depID] {
			return false
		}
	}
	return true
}

// detectCycles detects circular dependencies in the task graph
func detectCycles(tasks []*Task) map[string]bool {
	taskMap := make(map[string]*Task)
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
				Warn("Circular dependency detected involving task %s", task.ID)
			}
		}
	}

	return inCycle
}

// sortTasksByPriority sorts tasks according to the specified order
func sortTasksByPriority(tasks []*Task, orderBy []string) {
	sort.SliceStable(tasks, func(i, j int) bool {
		for _, criterion := range orderBy {
			switch criterion {
			case "por":
				if tasks[i].POR != tasks[j].POR {
					return tasks[i].POR < tasks[j].POR
				}
			case "priority":
				if tasks[i].Priority != tasks[j].Priority {
					return tasks[i].Priority < tasks[j].Priority
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

// ResumeIfInProgress checks if there's a task in progress and resumes it
func ResumeIfInProgress(st *State, journalPath string) (bool, string, error) {
	if st.WIP == "" {
		return false, "", nil
	}

	// Get the last journal entry
	lastEntry, err := getLastJournalEntry(journalPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to read journal: %w", err)
	}

	// Sync state with journal if needed
	changed, newState, err := SyncStateWithJournal(st, lastEntry)
	if err != nil {
		return false, "", err
	}

	if changed {
		*st = *newState
	}

	// Build context for incomplete detection during resume
	tasks, _ := loadAllTasks(".deespec/specs/sbi")
	completedTasks := getCompletedTasksFromJournal(journalPath)
	pickCtx := &PickContext{
		JournalPath:    journalPath,
		CompletedTasks: completedTasks,
		AllTasks:       tasks,
	}

	// Find the current task
	var currentTask *Task
	for _, t := range tasks {
		if t.ID == st.WIP {
			currentTask = t
			break
		}
	}

	if currentTask != nil {
		// Detect incomplete instructions during resume
		drafts, err := DetectIncomplete(currentTask, pickCtx)
		if err != nil {
			Warn("Failed to detect incomplete during resume: %v", err)
		}

		for _, draft := range drafts {
			Info("Detected incomplete during resume for %s: %s",
				draft.TargetTaskID, draft.ReasonCode)

			// Persist FB draft to SBI directory
			sbiDir := filepath.Join(".deespec", "specs", "sbi", draft.TargetTaskID)
			draftPath, err := PersistFBDraft(draft, sbiDir)
			if err != nil {
				Warn("Failed to persist FB draft: %v", err)
			} else {
				Info("FB draft saved to %s", draftPath)

				// Record in journal
				if err := RecordFBDraftInJournal(draft, journalPath, st.Turn); err != nil {
					Warn("Failed to record FB draft in journal: %v", err)
				}
			}
		}
	}

	reason := fmt.Sprintf("resuming task %s at step %s", st.WIP, st.Current)
	return true, reason, nil
}

// SyncStateWithJournal synchronizes state with the journal according to the state-journal sync matrix
func SyncStateWithJournal(st *State, lastEntry map[string]interface{}) (bool, *State, error) {
	if lastEntry == nil {
		// No journal entry, state is authoritative
		return false, st, nil
	}

	// Extract journal step and decision
	journalStep, _ := lastEntry["step"].(string)
	journalDecision, _ := lastEntry["decision"].(string)

	// Apply state-journal sync matrix
	newState := *st
	changed := false

	switch journalStep {
	case "plan":
		if journalDecision == "PENDING" {
			// Pick completed, ready for implement
			newState.Current = "implement"
			changed = true
		}
	case "implement":
		if journalDecision == "PENDING" {
			// Implement step needs to be retried (idempotent)
			newState.Current = "implement"
			changed = true
		}
	case "test":
		if journalDecision == "PENDING" {
			// Test step needs to be retried (idempotent)
			newState.Current = "test"
			changed = true
		}
	case "review":
		if journalDecision == "NEEDS_CHANGES" {
			// Back to implement due to review feedback
			newState.Current = "implement"
			changed = true
		} else if journalDecision == "OK" {
			// Review approved, ready for done
			newState.Current = "done"
			changed = true
		}
	case "done":
		if journalDecision == "OK" {
			// Task completed, clear WIP and reset to plan
			newState.Current = "plan"
			newState.WIP = ""
			changed = true
		}
	}

	if !changed {
		return false, st, nil
	}

	return true, &newState, nil
}

// getLastJournalEntry reads the last entry from the journal
func getLastJournalEntry(journalPath string) (map[string]interface{}, error) {
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
func RecordPickInJournal(task *Task, turn int, journalPath string) error {
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
