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
	POR        int                    `json:"por"`       // Priority of Requirements
	DependsOn  []string               `json:"depends_on"` // Task dependencies
	Meta       map[string]interface{} `json:"meta"`      // Metadata from meta.yaml
	Status     string                 `json:"status"`    // Current status from journal
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
	SpecsDir     string   // Base directory for specs (default: .deespec/specs/sbi)
	JournalPath  string   // Path to journal file
	OrderBy      []string // Sort order: ["por", "priority", "id"]
	StderrLevel  string   // Log level
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

	// Filter to ready tasks (not completed, dependencies met)
	readyTasks := filterReadyTasks(tasks, cfg.JournalPath)

	if len(readyTasks) == 0 {
		return nil, "no ready tasks", nil
	}

	// Sort by priority rules
	sortTasksByPriority(readyTasks, cfg.OrderBy)

	// Pick the first one
	picked := readyTasks[0]
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

		// Look for meta.yaml files
		if info.Name() == "meta.yaml" {
			task, err := loadTaskFromMetaFile(path)
			if err != nil {
				// Log warning but continue
				fmt.Fprintf(os.Stderr, "WARN: failed to load task from %s: %v\n", path, err)
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

	var ready []*Task
	for _, task := range tasks {
		// Skip if already completed
		if completedTasks[task.ID] {
			continue
		}

		// Check dependencies
		if !areDepenciesMet(task, completedTasks) {
			continue
		}

		ready = append(ready, task)
	}

	return ready
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

		// Look for done steps
		if step, ok := entry["step"].(string); ok && step == "done" {
			// Extract task ID from artifacts
			if artifacts, ok := entry["artifacts"].([]interface{}); ok {
				for _, artifact := range artifacts {
					if artifactMap, ok := artifact.(map[string]interface{}); ok {
						if artifactType, ok := artifactMap["type"].(string); ok && artifactType == "pick" {
							if taskID, ok := artifactMap["id"].(string); ok {
								completed[taskID] = true
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
	if st.CurrentTaskID == "" {
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

	reason := fmt.Sprintf("resuming task %s at step %s", st.CurrentTaskID, st.Current)
	return true, reason, nil
}

// SyncStateWithJournal synchronizes state with the journal
func SyncStateWithJournal(st *State, lastEntry map[string]interface{}) (bool, *State, error) {
	if lastEntry == nil {
		// No journal entry, state is authoritative
		return false, st, nil
	}

	// Check if journal and state are in sync
	journalStep, _ := lastEntry["step"].(string)
	journalTurn := int(lastEntry["turn"].(float64))

	if st.Current == journalStep && st.Turn == journalTurn {
		// Already in sync
		return false, st, nil
	}

	// Journal is the source of truth - update state to match
	newState := *st
	newState.Current = journalStep
	newState.Turn = journalTurn

	// If journal shows done, clear the current task
	if journalStep == "done" {
		newState.CurrentTaskID = ""
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

	entry := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       turn,
		"step":       "plan",
		"decision":   "PENDING",
		"elapsed_ms": 0,
		"error":      "",
		"artifacts": []map[string]interface{}{
			{
				"type":      "pick",
				"id":        task.ID,
				"spec_path": task.SpecPath,
			},
		},
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