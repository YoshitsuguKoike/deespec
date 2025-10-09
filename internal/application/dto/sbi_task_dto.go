package dto

// SBITaskDTO represents a Spec Backlog Item task loaded from file system
// This is a DTO for the legacy file-based task system
type SBITaskDTO struct {
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

// TaskMetaYAML represents the structure of meta.yaml in task directories
type TaskMetaYAML struct {
	ID        string   `yaml:"id"`
	Title     string   `yaml:"title"`
	Priority  int      `yaml:"priority"`
	POR       int      `yaml:"por"`
	DependsOn []string `yaml:"depends_on"`
	Phase     string   `yaml:"phase"`
	Role      string   `yaml:"role"`
}

// NewSBITaskDTO creates a new SBITaskDTO with default values
func NewSBITaskDTO(id string) *SBITaskDTO {
	return &SBITaskDTO{
		ID:       id,
		Priority: 999, // Low priority by default
		POR:      999, // Low POR by default
		Status:   "PENDING",
		Meta:     make(map[string]interface{}),
	}
}

// HasDependencies checks if the task has any dependencies
func (t *SBITaskDTO) HasDependencies() bool {
	return len(t.DependsOn) > 0
}

// AreDependenciesMet checks if all dependencies are satisfied
func (t *SBITaskDTO) AreDependenciesMet(completedTasks map[string]bool) bool {
	for _, depID := range t.DependsOn {
		if !completedTasks[depID] {
			return false
		}
	}
	return true
}

// TaskPickInput represents input for task picking operation
type TaskPickInput struct {
	SpecsDir    string   // Base directory for specs (default: .deespec/specs/sbi)
	JournalPath string   // Path to journal file
	OrderBy     []string // Sort order: ["por", "priority", "id"]
	StderrLevel string   // Log level
}

// TaskPickOutput represents the result of task picking operation
type TaskPickOutput struct {
	Task   *SBITaskDTO // Selected task (nil if no task available)
	Reason string      // Reason for selection or why no task was selected
	Error  error       // Error if operation failed
}

// PickContext provides context for task picking operations
type PickContext struct {
	JournalPath    string
	CompletedTasks map[string]bool
	AllTasks       []*SBITaskDTO
}

// IncompleteReason represents the reason a task is considered incomplete
type IncompleteReason string

const (
	DepUnresolved IncompleteReason = "DEP_UNRESOLVED"
	DepCycle      IncompleteReason = "DEP_CYCLE"
	MetaMissing   IncompleteReason = "META_MISSING"
	PathInvalid   IncompleteReason = "PATH_INVALID"
	PromptError   IncompleteReason = "PROMPT_ERROR"
	TimeFormat    IncompleteReason = "TIME_FORMAT"
	JournalGuard  IncompleteReason = "JOURNAL_GUARD"
)

// FBDraft represents a feedback draft for incomplete instructions
type FBDraft struct {
	TargetTaskID  string           `json:"target_task_id"`
	ReasonCode    IncompleteReason `json:"reason_code"`
	Title         string           `json:"title"`
	Summary       string           `json:"summary"`
	EvidencePaths []string         `json:"evidence_paths"`
	SuggestedFBID string           `json:"suggested_fb_id"`
	CreatedAt     string           `json:"created_at"` // RFC3339Nano format
}
