package strategy

import (
	"context"
	"errors"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// SBICodeGenerationStrategy implements the strategy for SBI tasks
// SBI implementation means generating actual code
type SBICodeGenerationStrategy struct {
	agentExecutor AgentExecutor
}

// NewSBICodeGenerationStrategy creates a new SBI code generation strategy
func NewSBICodeGenerationStrategy(agentExecutor AgentExecutor) *SBICodeGenerationStrategy {
	return &SBICodeGenerationStrategy{
		agentExecutor: agentExecutor,
	}
}

// Execute performs SBI code generation
func (s *SBICodeGenerationStrategy) Execute(ctx context.Context, t task.Task) (*ImplementationResult, error) {
	// Type assertion to SBI
	sbiTask, ok := t.(*sbi.SBI)
	if !ok {
		return nil, errors.New("task is not an SBI")
	}

	// Check if max turns or attempts exceeded
	if sbiTask.HasExceededMaxTurns() {
		return &ImplementationResult{
			Success:  false,
			Message:  "Maximum turns exceeded",
			NextStep: model.StepReview, // Move to review for manual intervention
		}, errors.New("maximum turns exceeded")
	}

	if sbiTask.HasExceededMaxAttempts() {
		return &ImplementationResult{
			Success:  false,
			Message:  "Maximum attempts exceeded",
			NextStep: model.StepReview, // Move to review for manual intervention
		}, errors.New("maximum attempts exceeded")
	}

	// Build prompt for AI agent to generate code
	prompt := s.buildCodeGenerationPrompt(sbiTask)

	// Execute AI agent to generate code
	response, err := s.agentExecutor.Execute(ctx, prompt, model.TaskTypeSBI)
	if err != nil {
		// Record error and increment attempt
		sbiTask.RecordError(err.Error())
		sbiTask.IncrementAttempt()

		return &ImplementationResult{
			Success:  false,
			Message:  "Failed to generate code: " + err.Error(),
			NextStep: model.StepImplement, // Retry
		}, err
	}

	// Parse the response and create artifacts
	artifacts := s.parseCodeArtifacts(response, sbiTask)

	// Clear error and increment turn
	sbiTask.ClearError()
	sbiTask.IncrementTurn()

	return &ImplementationResult{
		Success:   true,
		Message:   "Successfully generated code",
		Artifacts: artifacts,
		NextStep:  model.StepReview, // Move to review step
		Metadata: map[string]interface{}{
			"sbi_id":         sbiTask.ID().String(),
			"current_turn":   sbiTask.ExecutionState().CurrentTurn.Value(),
			"code_generated": true,
		},
	}, nil
}

// CanHandle checks if this strategy can handle the given task type
func (s *SBICodeGenerationStrategy) CanHandle(taskType model.TaskType) bool {
	return taskType == model.TaskTypeSBI
}

// GetName returns the strategy name
func (s *SBICodeGenerationStrategy) GetName() string {
	return "SBICodeGenerationStrategy"
}

// buildCodeGenerationPrompt builds the AI prompt for code generation
func (s *SBICodeGenerationStrategy) buildCodeGenerationPrompt(sbiTask *sbi.SBI) string {
	metadata := sbiTask.Metadata()
	executionState := sbiTask.ExecutionState()

	filePaths := ""
	for _, path := range metadata.FilePaths {
		filePaths += "\n- " + path
	}

	lastErrorInfo := ""
	if executionState.LastError != "" {
		lastErrorInfo = "\n\n**Previous Error:**\n" + executionState.LastError + "\n\nPlease fix this error in your implementation."
	}

	return `You are a software developer. Your task is to implement the following SBI.

SBI Title: ` + sbiTask.Title() + `
SBI Description: ` + sbiTask.Description() + `
Estimated Hours: ` + string(rune(int(metadata.EstimatedHours))) + `
Target Files:` + filePaths + `
Current Turn: ` + executionState.CurrentTurn.String() + `
Current Attempt: ` + executionState.CurrentAttempt.String() + lastErrorInfo + `

Please provide:
1. Complete, working code for each file
2. Comments explaining key logic
3. Error handling where appropriate
4. Unit tests if applicable

Format your response as markdown with code blocks for each file:
## File: [path]
` + "```" + `[language]
[code]
` + "```" + `

## Tests: [test_path]
` + "```" + `[language]
[test_code]
` + "```" + ``
}

// parseCodeArtifacts parses the AI response into code artifacts
func (s *SBICodeGenerationStrategy) parseCodeArtifacts(response string, sbiTask *sbi.SBI) []Artifact {
	var artifacts []Artifact
	lines := strings.Split(response, "\n")

	var currentPath string
	var currentType ArtifactType
	var inCodeBlock bool
	var codeLines []string

	for _, line := range lines {
		// ヘッダー検出
		if strings.HasPrefix(line, "## File: ") {
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "## File: "))
			currentType = ArtifactTypeCode
			continue
		}
		if strings.HasPrefix(line, "## Tests: ") {
			currentPath = strings.TrimSpace(strings.TrimPrefix(line, "## Tests: "))
			currentType = ArtifactTypeTest
			continue
		}

		// コードブロック開始
		if strings.HasPrefix(line, "```") && !inCodeBlock {
			inCodeBlock = true
			codeLines = []string{}
			continue
		}

		// コードブロック終了
		if strings.HasPrefix(line, "```") && inCodeBlock {
			inCodeBlock = false
			if currentPath != "" {
				artifacts = append(artifacts, Artifact{
					Path:        currentPath,
					Content:     strings.Join(codeLines, "\n"),
					Type:        currentType,
					Description: string(currentType) + " for SBI: " + sbiTask.Title(),
				})
			}
			currentPath = ""
			continue
		}

		// コードブロック内
		if inCodeBlock {
			codeLines = append(codeLines, line)
		}
	}

	return artifacts
}
