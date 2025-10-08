package strategy

import (
	"context"
	"errors"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// EPICDecompositionStrategy implements the strategy for EPIC tasks
// EPIC implementation means decomposing it into PBIs
type EPICDecompositionStrategy struct {
	agentExecutor AgentExecutor
}

// AgentExecutor is an interface for executing AI agent tasks
// This will be implemented by the application layer gateway
type AgentExecutor interface {
	Execute(ctx context.Context, prompt string, taskType model.TaskType) (string, error)
}

// NewEPICDecompositionStrategy creates a new EPIC decomposition strategy
func NewEPICDecompositionStrategy(agentExecutor AgentExecutor) *EPICDecompositionStrategy {
	return &EPICDecompositionStrategy{
		agentExecutor: agentExecutor,
	}
}

// Execute performs EPIC decomposition into PBIs
func (s *EPICDecompositionStrategy) Execute(ctx context.Context, t task.Task) (*ImplementationResult, error) {
	// Type assertion to EPIC
	epicTask, ok := t.(*epic.EPIC)
	if !ok {
		return nil, errors.New("task is not an EPIC")
	}

	// Build prompt for AI agent to decompose EPIC into PBIs
	prompt := s.buildDecompositionPrompt(epicTask)

	// Execute AI agent to generate PBI proposals
	response, err := s.agentExecutor.Execute(ctx, prompt, model.TaskTypeEPIC)
	if err != nil {
		return &ImplementationResult{
			Success:  false,
			Message:  "Failed to decompose EPIC: " + err.Error(),
			NextStep: model.StepImplement, // Retry
		}, err
	}

	// Parse the response and create artifact
	// The actual PBI creation will be done by the application layer use case
	artifact := Artifact{
		Path:        "pbi_proposals.md",
		Content:     response,
		Type:        ArtifactTypeTask,
		Description: "PBI proposals for EPIC: " + epicTask.Title(),
	}

	return &ImplementationResult{
		Success:   true,
		Message:   "Successfully generated PBI proposals",
		Artifacts: []Artifact{artifact},
		NextStep:  model.StepReview, // Move to review step
		Metadata: map[string]interface{}{
			"epic_id":    epicTask.ID().String(),
			"pbi_count":  0, // Will be populated after review
			"decomposed": true,
		},
	}, nil
}

// CanHandle checks if this strategy can handle the given task type
func (s *EPICDecompositionStrategy) CanHandle(taskType model.TaskType) bool {
	return taskType == model.TaskTypeEPIC
}

// GetName returns the strategy name
func (s *EPICDecompositionStrategy) GetName() string {
	return "EPICDecompositionStrategy"
}

// buildDecompositionPrompt builds the AI prompt for EPIC decomposition
func (s *EPICDecompositionStrategy) buildDecompositionPrompt(epicTask *epic.EPIC) string {
	return `You are a software development planner. Your task is to decompose an EPIC into Product Backlog Items (PBIs).

EPIC Title: ` + epicTask.Title() + `
EPIC Description: ` + epicTask.Description() + `

Please decompose this EPIC into 3-7 PBIs. For each PBI, provide:
1. Title (concise, action-oriented)
2. Description (what needs to be done)
3. Story Points (1, 2, 3, 5, 8, 13)
4. Acceptance Criteria (2-5 criteria)

Format your response as markdown with the following structure:
## PBI 1: [Title]
**Description:** [Description]
**Story Points:** [Points]
**Acceptance Criteria:**
- [Criterion 1]
- [Criterion 2]
...`
}
