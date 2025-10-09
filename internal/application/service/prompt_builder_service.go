package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// PromptBuilderService handles building prompts for Claude Code
type PromptBuilderService struct {
	templateRepo repository.PromptTemplateRepository
	labelRepo    repository.LabelRepository
}

// NewPromptBuilderService creates a new prompt builder service
func NewPromptBuilderService(
	templateRepo repository.PromptTemplateRepository,
	labelRepo repository.LabelRepository,
) *PromptBuilderService {
	return &PromptBuilderService{
		templateRepo: templateRepo,
		labelRepo:    labelRepo,
	}
}

// LoadExternalPrompt loads and processes an external prompt template
func (s *PromptBuilderService) LoadExternalPrompt(
	ctx context.Context,
	status string,
	promptCtx *dto.PromptContextDTO,
) (*dto.PromptResultDTO, error) {
	// Load template from repository
	template, err := s.templateRepo.LoadTemplate(ctx, status)
	if err != nil {
		return nil, err
	}

	// Enrich task description with labels
	enrichedTaskDescription, warnings := s.EnrichTaskWithLabels(ctx, promptCtx.TaskDescription, promptCtx.SBIID)

	// Replace placeholders in template
	prompt := s.replacePlaceholders(template, promptCtx, enrichedTaskDescription)

	return &dto.PromptResultDTO{
		Content:  prompt,
		Warnings: warnings,
	}, nil
}

// replacePlaceholders replaces all template placeholders with actual values
func (s *PromptBuilderService) replacePlaceholders(
	template string,
	promptCtx *dto.PromptContextDTO,
	enrichedTaskDescription string,
) string {
	prompt := template
	prompt = strings.ReplaceAll(prompt, "{{.WorkDir}}", promptCtx.WorkDir)
	prompt = strings.ReplaceAll(prompt, "{{.SBIID}}", promptCtx.SBIID)
	prompt = strings.ReplaceAll(prompt, "{{.Turn}}", fmt.Sprintf("%d", promptCtx.Turn))
	prompt = strings.ReplaceAll(prompt, "{{.Step}}", promptCtx.Step)
	prompt = strings.ReplaceAll(prompt, "{{.SBIDir}}", promptCtx.SBIDir)
	prompt = strings.ReplaceAll(prompt, "{{.TaskDescription}}", enrichedTaskDescription)
	prompt = strings.ReplaceAll(prompt, "{{.Timestamp}}", time.Now().Format("2006-01-02 15:04:05"))

	return prompt
}

// BuildImplementPrompt creates an implementation prompt
func (s *PromptBuilderService) BuildImplementPrompt(promptCtx *dto.PromptContextDTO) string {
	var sb strings.Builder

	sb.WriteString("# Implementation Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", promptCtx.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", promptCtx.SBIID))
	sb.WriteString(fmt.Sprintf("- Turn: %d\n", promptCtx.Turn))
	sb.WriteString(fmt.Sprintf("- Step: %s\n", promptCtx.Step))
	sb.WriteString(fmt.Sprintf("- Artifacts Directory: `%s`\n", promptCtx.SBIDir))
	sb.WriteString("\n")

	sb.WriteString("## Task Description\n")
	sb.WriteString(promptCtx.TaskDescription)
	sb.WriteString("\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Analyze the task requirements and existing code structure\n")
	sb.WriteString("2. Use Read/Grep/Glob tools to understand the codebase\n")
	sb.WriteString("3. Implement required changes using Edit/MultiEdit/Write tools\n")
	sb.WriteString("4. Follow existing code patterns and conventions\n")
	sb.WriteString("5. Ensure changes are atomic and don't break existing functionality\n")
	sb.WriteString("6. Add appropriate error handling and validation\n")
	sb.WriteString("\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to all Claude Code tools for implementation.\n")
	sb.WriteString("\n")

	sb.WriteString("## Output Requirements\n")
	sb.WriteString("At the end of your implementation, provide:\n")
	sb.WriteString("1. **Summary of Changes**: List all files modified and what was changed\n")
	sb.WriteString("2. **Key Decisions**: Explain important implementation choices\n")
	sb.WriteString("3. **Testing Recommendations**: Suggest how to verify the changes\n")
	sb.WriteString("\n")
	sb.WriteString("## Implementation Note\n")
	sb.WriteString("End with a section '## Implementation Note' containing a 2-3 sentence summary.\n")

	return sb.String()
}

// BuildTestPrompt creates a test prompt
func (s *PromptBuilderService) BuildTestPrompt(promptCtx *dto.PromptContextDTO, previousArtifact string) string {
	var sb strings.Builder

	sb.WriteString("# Test Verification Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", promptCtx.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", promptCtx.SBIID))
	sb.WriteString(fmt.Sprintf("- Turn: %d\n", promptCtx.Turn))
	sb.WriteString(fmt.Sprintf("- Previous Step: implement\n"))
	sb.WriteString("\n")

	if previousArtifact != "" {
		sb.WriteString("## Previous Implementation\n")
		sb.WriteString(fmt.Sprintf("Implementation artifact location: `%s`\n", previousArtifact))
		sb.WriteString("First read this artifact to understand what was implemented.\n")
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n")
	sb.WriteString("1. Read the implementation artifact to understand changes\n")
	sb.WriteString("2. Identify the test strategy based on the project type\n")
	sb.WriteString("3. Run appropriate tests using Bash tool:\n")
	sb.WriteString("   - For Go: `go test ./...` or specific package tests\n")
	sb.WriteString("   - For Node: `npm test` or `yarn test`\n")
	sb.WriteString("   - For Python: `pytest` or `python -m unittest`\n")
	sb.WriteString("4. If no tests exist, create simple verification commands\n")
	sb.WriteString("5. Document any failures or warnings\n")
	sb.WriteString("\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to all Claude Code tools for testing.\n")
	sb.WriteString("\n")

	sb.WriteString("## Test Output Format\n")
	sb.WriteString("Provide:\n")
	sb.WriteString("1. **Test Commands Run**: List all test commands executed\n")
	sb.WriteString("2. **Results Summary**: Pass/Fail status and counts\n")
	sb.WriteString("3. **Issues Found**: Any failures or warnings\n")
	sb.WriteString("4. **Coverage**: What was tested and what wasn't\n")
	sb.WriteString("\n")
	sb.WriteString("## Test Note\n")
	sb.WriteString("End with a '## Test Note' section summarizing the test results.\n")

	return sb.String()
}

// BuildReviewPrompt creates a review prompt
func (s *PromptBuilderService) BuildReviewPrompt(
	promptCtx *dto.PromptContextDTO,
	artifacts *dto.ArtifactLocationDTO,
) string {
	var sb strings.Builder

	sb.WriteString("# Code Review Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", promptCtx.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", promptCtx.SBIID))
	sb.WriteString(fmt.Sprintf("- Turn: %d\n", promptCtx.Turn))
	sb.WriteString(fmt.Sprintf("- Reviewing: implementation and test results\n"))
	sb.WriteString("\n")

	sb.WriteString("## Artifacts to Review\n")
	sb.WriteString("Read these artifacts to understand what was done:\n")
	if artifacts.ImplementArtifact != "" {
		sb.WriteString(fmt.Sprintf("1. Implementation: `%s`\n", artifacts.ImplementArtifact))
	}
	if artifacts.TestArtifact != "" {
		sb.WriteString(fmt.Sprintf("2. Test Results: `%s`\n", artifacts.TestArtifact))
	}
	sb.WriteString("\n")

	sb.WriteString("## Review Process\n")
	sb.WriteString("1. Read both artifacts carefully\n")
	sb.WriteString("2. Use Read/Grep tools to verify actual code changes\n")
	sb.WriteString("3. Check if implementation matches requirements\n")
	sb.WriteString("4. Verify test results are satisfactory\n")
	sb.WriteString("5. Look for potential issues or improvements\n")
	sb.WriteString("\n")

	sb.WriteString("## Available Tools\n")
	sb.WriteString("You have access to all Claude Code tools for review.\n")
	sb.WriteString("\n")

	sb.WriteString("## Review Criteria\n")
	sb.WriteString("Evaluate based on:\n")
	sb.WriteString("1. **Functionality**: Does it solve the intended problem?\n")
	sb.WriteString("2. **Code Quality**: Is it well-structured and maintainable?\n")
	sb.WriteString("3. **Testing**: Are tests comprehensive and passing?\n")
	sb.WriteString("4. **Standards**: Does it follow project conventions?\n")
	sb.WriteString("5. **Edge Cases**: Are error cases handled properly?\n")
	sb.WriteString("\n")

	sb.WriteString("## Decision Guidelines\n")
	sb.WriteString("Make your decision based on the review:\n")
	sb.WriteString("- `DECISION: SUCCEEDED` - Implementation is correct and tests pass\n")
	sb.WriteString("- `DECISION: NEEDS_CHANGES` - Issues found that need fixing\n")
	sb.WriteString("- `DECISION: FAILED` - Critical issues or unable to complete\n")
	sb.WriteString("\n")
	sb.WriteString("## Review Note\n")
	sb.WriteString("End with a '## Review Note' section explaining your decision with specific details.\n")

	return sb.String()
}

// BuildPlanPrompt creates a planning prompt
func (s *PromptBuilderService) BuildPlanPrompt(promptCtx *dto.PromptContextDTO, todo string) string {
	var sb strings.Builder

	sb.WriteString("# Planning Task\n\n")
	sb.WriteString("## Context\n")
	sb.WriteString(fmt.Sprintf("- Working Directory: `%s`\n", promptCtx.WorkDir))
	sb.WriteString(fmt.Sprintf("- SBI ID: %s\n", promptCtx.SBIID))
	sb.WriteString(fmt.Sprintf("- Starting Turn: %d\n", promptCtx.Turn))
	sb.WriteString("\n")

	sb.WriteString("## TODO\n")
	sb.WriteString(todo)
	sb.WriteString("\n\n")

	sb.WriteString("## Instructions\n")
	sb.WriteString("Create a detailed plan for implementing this task:\n")
	sb.WriteString("1. Analyze the requirements\n")
	sb.WriteString("2. Identify files that need to be modified\n")
	sb.WriteString("3. Break down the implementation into clear steps\n")
	sb.WriteString("4. Note any dependencies or prerequisites\n")
	sb.WriteString("5. Highlight potential challenges\n")
	sb.WriteString("\n")

	sb.WriteString("## Expected Output\n")
	sb.WriteString("Provide a structured plan with:\n")
	sb.WriteString("- Overview (200 characters max)\n")
	sb.WriteString("- Step-by-step implementation approach\n")
	sb.WriteString("- Files to be modified\n")
	sb.WriteString("- Testing strategy\n")

	return sb.String()
}

// GetLastArtifact returns the path to the last artifact for a given step
func (s *PromptBuilderService) GetLastArtifact(sbiDir string, step string, turn int) string {
	if sbiDir == "" || step == "" || turn <= 0 {
		return ""
	}
	return filepath.Join(sbiDir, fmt.Sprintf("%s_%d.md", step, turn))
}

// EnrichTaskWithLabels enriches the task description with label-based context
func (s *PromptBuilderService) EnrichTaskWithLabels(
	ctx context.Context,
	taskDescription string,
	sbiID string,
) (string, []string) {
	// Load labels from meta.yaml via repository
	labels, err := s.templateRepo.LoadMetaLabels(ctx, sbiID)
	if err != nil || len(labels) == 0 {
		return taskDescription, nil
	}

	// Build enriched description
	var enriched strings.Builder
	enriched.WriteString(taskDescription)

	labelInstructions := make([]string, 0)
	warnings := make([]string, 0)

	for _, labelName := range labels {
		// If LabelRepo is available, use Repository-based approach
		if s.labelRepo != nil {
			lbl, err := s.labelRepo.FindByName(ctx, labelName)
			if err != nil {
				// Label not found in DB, fall back to template repository
				warnings = append(warnings, fmt.Sprintf("⚠ Label '%s' not found in database (using fallback)", labelName))
				content := s.templateRepo.LoadLabelContent(ctx, labelName)
				if content != "" {
					labelInstructions = append(labelInstructions, fmt.Sprintf("### Label: %s\n%s", labelName, content))
				}
				continue
			}

			// Validate label integrity
			validationResult, err := s.labelRepo.ValidateIntegrity(ctx, lbl.ID())
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("⚠ Label '%s': validation failed - %v", labelName, err))
			} else if validationResult.Status == repository.ValidationModified {
				warnings = append(warnings, fmt.Sprintf("⚠ Label '%s': template file has been modified since last sync", labelName))
			} else if validationResult.Status == repository.ValidationMissing {
				warnings = append(warnings, fmt.Sprintf("⚠ Label '%s': template file is missing", labelName))
			}

			// Get label content from template paths
			var labelContent strings.Builder
			for _, templatePath := range lbl.TemplatePaths() {
				content := s.templateRepo.LoadLabelContent(ctx, templatePath)
				if content != "" {
					labelContent.WriteString(content)
					labelContent.WriteString("\n")
				} else {
					warnings = append(warnings, fmt.Sprintf("⚠ Label '%s': failed to read template '%s'", labelName, templatePath))
				}
			}

			if labelContent.Len() > 0 {
				labelInstructions = append(labelInstructions, fmt.Sprintf("### Label: %s\n%s", labelName, labelContent.String()))
			}
		} else {
			// Fallback: File-based approach via template repository
			content := s.templateRepo.LoadLabelContent(ctx, labelName)
			if content != "" {
				labelInstructions = append(labelInstructions, fmt.Sprintf("### Label: %s\n%s", labelName, content))
			}
		}
	}

	// Add warnings if any
	if len(warnings) > 0 {
		enriched.WriteString("\n\n## Label Warnings\n")
		for _, warning := range warnings {
			enriched.WriteString(warning)
			enriched.WriteString("\n")
		}
	}

	// Add label instructions if any exist
	if len(labelInstructions) > 0 {
		enriched.WriteString("\n\n## Label-Specific Guidelines\n")
		enriched.WriteString("The following guidelines apply based on the task labels:\n\n")
		for _, instruction := range labelInstructions {
			enriched.WriteString(instruction)
			enriched.WriteString("\n\n")
		}
	} else if len(labels) > 0 {
		// Just list the labels if no specific instructions
		enriched.WriteString("\n\n## Task Labels\n")
		enriched.WriteString("This task is tagged with: ")
		enriched.WriteString(strings.Join(labels, ", "))
		enriched.WriteString("\n")
	}

	return enriched.String(), warnings
}
