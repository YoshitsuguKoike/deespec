package strategy

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// ImplementationStrategy defines the interface for task implementation strategies
// Different task types (EPIC, PBI, SBI) have different implementation behaviors
type ImplementationStrategy interface {
	// Execute performs the implementation step for a specific task type
	Execute(ctx context.Context, t task.Task) (*ImplementationResult, error)

	// CanHandle checks if this strategy can handle the given task type
	CanHandle(taskType model.TaskType) bool

	// GetName returns the strategy name
	GetName() string
}

// ImplementationResult represents the result of an implementation step
type ImplementationResult struct {
	Success      bool
	Message      string
	Artifacts    []Artifact
	NextStep     model.Step
	ChildTaskIDs []model.TaskID // For EPIC/PBI decomposition
	Metadata     map[string]interface{}
}

// Artifact represents a generated artifact (file, code, etc.)
type Artifact struct {
	Path        string
	Content     string
	Type        ArtifactType
	Description string
}

// ArtifactType represents the type of artifact
type ArtifactType string

const (
	ArtifactTypeCode          ArtifactType = "CODE"
	ArtifactTypeDocumentation ArtifactType = "DOCUMENTATION"
	ArtifactTypeTest          ArtifactType = "TEST"
	ArtifactTypeConfig        ArtifactType = "CONFIG"
	ArtifactTypeTask          ArtifactType = "TASK" // For decomposed tasks
)

// StrategyRegistry manages implementation strategies
type StrategyRegistry struct {
	strategies map[model.TaskType]ImplementationStrategy
}

// NewStrategyRegistry creates a new strategy registry
func NewStrategyRegistry() *StrategyRegistry {
	return &StrategyRegistry{
		strategies: make(map[model.TaskType]ImplementationStrategy),
	}
}

// Register registers a strategy for a task type
func (r *StrategyRegistry) Register(taskType model.TaskType, strategy ImplementationStrategy) {
	r.strategies[taskType] = strategy
}

// GetStrategy retrieves the strategy for a task type
func (r *StrategyRegistry) GetStrategy(taskType model.TaskType) (ImplementationStrategy, bool) {
	strategy, exists := r.strategies[taskType]
	return strategy, exists
}

// ExecuteImplementation executes the implementation strategy for a task
func (r *StrategyRegistry) ExecuteImplementation(ctx context.Context, t task.Task) (*ImplementationResult, error) {
	strategy, exists := r.GetStrategy(t.Type())
	if !exists {
		return nil, &StrategyNotFoundError{TaskType: t.Type()}
	}

	return strategy.Execute(ctx, t)
}

// StrategyNotFoundError is returned when no strategy is found for a task type
type StrategyNotFoundError struct {
	TaskType model.TaskType
}

func (e *StrategyNotFoundError) Error() string {
	return "no implementation strategy found for task type: " + e.TaskType.String()
}
