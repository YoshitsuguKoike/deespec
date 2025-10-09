package claude_prompt

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	infraRepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
)

// ClaudeCodePromptBuilder builds prompts specifically for Claude Code
// This is now a thin wrapper around PromptBuilderService
type ClaudeCodePromptBuilder struct {
	WorkDir   string
	SBIDir    string
	SBIID     string
	Turn      int
	Step      string
	LabelRepo repository.LabelRepository // Phase 9.1e: Repository-based label enrichment

	// Internal service
	service *service.PromptBuilderService
}

// NewClaudeCodePromptBuilder creates a new prompt builder with default repositories
func NewClaudeCodePromptBuilder(
	workDir, sbiDir, sbiID string,
	turn int,
	step string,
	labelRepo repository.LabelRepository,
) *ClaudeCodePromptBuilder {
	// Create default repositories
	templateRepo := infraRepo.NewPromptTemplateRepositoryImpl()

	// Create service
	promptService := service.NewPromptBuilderService(templateRepo, labelRepo)

	return &ClaudeCodePromptBuilder{
		WorkDir:   workDir,
		SBIDir:    sbiDir,
		SBIID:     sbiID,
		Turn:      turn,
		Step:      step,
		LabelRepo: labelRepo,
		service:   promptService,
	}
}

// LoadExternalPrompt loads a prompt template from external file
func (b *ClaudeCodePromptBuilder) LoadExternalPrompt(status string, taskDescription string) (string, error) {
	ctx := context.Background()

	promptCtx := &dto.PromptContextDTO{
		WorkDir:         b.WorkDir,
		SBIDir:          b.SBIDir,
		SBIID:           b.SBIID,
		Turn:            b.Turn,
		Step:            b.Step,
		TaskDescription: taskDescription,
	}

	result, err := b.service.LoadExternalPrompt(ctx, status, promptCtx)
	if err != nil {
		return "", err
	}

	// Note: Warnings from result are currently discarded for backward compatibility
	// Future enhancement: expose warnings through builder interface
	return result.Content, nil
}

// BuildImplementPrompt creates an implementation prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildImplementPrompt(taskDescription string) string {
	promptCtx := &dto.PromptContextDTO{
		WorkDir:         b.WorkDir,
		SBIDir:          b.SBIDir,
		SBIID:           b.SBIID,
		Turn:            b.Turn,
		Step:            b.Step,
		TaskDescription: taskDescription,
	}

	return b.service.BuildImplementPrompt(promptCtx)
}

// BuildTestPrompt creates a test prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildTestPrompt(previousArtifact string) string {
	promptCtx := &dto.PromptContextDTO{
		WorkDir: b.WorkDir,
		SBIDir:  b.SBIDir,
		SBIID:   b.SBIID,
		Turn:    b.Turn,
		Step:    b.Step,
	}

	return b.service.BuildTestPrompt(promptCtx, previousArtifact)
}

// BuildReviewPrompt creates a review prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildReviewPrompt(implementArtifact, testArtifact string) string {
	promptCtx := &dto.PromptContextDTO{
		WorkDir: b.WorkDir,
		SBIDir:  b.SBIDir,
		SBIID:   b.SBIID,
		Turn:    b.Turn,
		Step:    b.Step,
	}

	artifacts := &dto.ArtifactLocationDTO{
		ImplementArtifact: implementArtifact,
		TestArtifact:      testArtifact,
	}

	return b.service.BuildReviewPrompt(promptCtx, artifacts)
}

// BuildPlanPrompt creates a planning prompt for Claude Code
func (b *ClaudeCodePromptBuilder) BuildPlanPrompt(todo string) string {
	promptCtx := &dto.PromptContextDTO{
		WorkDir: b.WorkDir,
		SBIDir:  b.SBIDir,
		SBIID:   b.SBIID,
		Turn:    b.Turn,
		Step:    b.Step,
	}

	return b.service.BuildPlanPrompt(promptCtx, todo)
}

// GetLastArtifact returns the path to the last artifact for a given step
func (b *ClaudeCodePromptBuilder) GetLastArtifact(step string, turn int) string {
	return b.service.GetLastArtifact(b.SBIDir, step, turn)
}

// EnrichTaskWithLabels enriches the task description with label-based context
// Phase 9.1e: Repository-based implementation with integrity validation
// This is now a convenience method that delegates to the service
func (b *ClaudeCodePromptBuilder) EnrichTaskWithLabels(taskDescription string, sbiID string) string {
	ctx := context.Background()
	enriched, _ := b.service.EnrichTaskWithLabels(ctx, taskDescription, sbiID)
	// Note: Warnings are currently discarded for backward compatibility
	return enriched
}

// fallbackLabelContent provides file-based label content retrieval for backward compatibility
// Deprecated: This method is maintained for compatibility but delegates to the repository
func (b *ClaudeCodePromptBuilder) fallbackLabelContent(labelName string) string {
	ctx := context.Background()
	templateRepo := infraRepo.NewPromptTemplateRepositoryImpl()
	return templateRepo.LoadLabelContent(ctx, labelName)
}
