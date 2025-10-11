package strategy

import (
	"context"
	"errors"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// PBIDecompositionStrategy implements the strategy for PBI tasks
// PBI implementation means decomposing it into SBIs
type PBIDecompositionStrategy struct {
	agentExecutor AgentExecutor
}

// NewPBIDecompositionStrategy creates a new PBI decomposition strategy
func NewPBIDecompositionStrategy(agentExecutor AgentExecutor) *PBIDecompositionStrategy {
	return &PBIDecompositionStrategy{
		agentExecutor: agentExecutor,
	}
}

// Execute performs PBI decomposition into SBIs
func (s *PBIDecompositionStrategy) Execute(ctx context.Context, t task.Task) (*ImplementationResult, error) {
	// Verify task type
	if t.Type() != model.TaskTypePBI {
		return nil, errors.New("task is not a PBI")
	}

	// Build prompt for AI agent to decompose PBI into SBIs
	prompt := s.buildDecompositionPrompt(t)

	// Execute AI agent to generate SBI proposals
	response, err := s.agentExecutor.Execute(ctx, prompt, model.TaskTypePBI)
	if err != nil {
		return &ImplementationResult{
			Success:  false,
			Message:  "Failed to decompose PBI: " + err.Error(),
			NextStep: model.StepImplement, // Retry
		}, err
	}

	// Parse the response and create artifact
	artifact := Artifact{
		Path:        "sbi_proposals.md",
		Content:     response,
		Type:        ArtifactTypeTask,
		Description: "SBI proposals for PBI: " + t.Title(),
	}

	return &ImplementationResult{
		Success:   true,
		Message:   "Successfully generated SBI proposals",
		Artifacts: []Artifact{artifact},
		NextStep:  model.StepReview, // Move to review step
		Metadata: map[string]interface{}{
			"pbi_id":     t.ID().String(),
			"sbi_count":  0, // Will be populated after review
			"decomposed": true,
		},
	}, nil
}

// CanHandle checks if this strategy can handle the given task type
func (s *PBIDecompositionStrategy) CanHandle(taskType model.TaskType) bool {
	return taskType == model.TaskTypePBI
}

// GetName returns the strategy name
func (s *PBIDecompositionStrategy) GetName() string {
	return "PBIDecompositionStrategy"
}

// buildDecompositionPrompt builds the AI prompt for PBI decomposition
func (s *PBIDecompositionStrategy) buildDecompositionPrompt(t task.Task) string {
	return `You are a software development planner. Your task is to decompose a PBI into Small Backlog Items (SBIs).

PBI Title: ` + t.Title() + `
PBI Description: ` + t.Description() + `

Please decompose this PBI into 2-8 SBIs. Each SBI should be a concrete implementation task. For each SBI, provide:
1. Title (specific, technical)
2. Description (implementation details)
3. Estimated Hours (0.5, 1, 2, 4, 8)
4. File Paths (files to be created/modified)

Format your response as markdown with the following structure:
## SBI 1: [Title]
**Description:** [Implementation details]
**Estimated Hours:** [Hours]
**File Paths:**
- [Path 1]
- [Path 2]
...`
}
