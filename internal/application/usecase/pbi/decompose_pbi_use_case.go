package pbi

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/pbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// DecomposeOptions defines options for PBI decomposition
type DecomposeOptions struct {
	MinSBIs    int  // Minimum number of SBIs to generate
	MaxSBIs    int  // Maximum number of SBIs to generate
	DryRun     bool // If true, only build prompt without executing
	OutputOnly bool // If true, only output prompt to stdout (for future use)
}

// DecomposeResult represents the result of PBI decomposition
type DecomposeResult struct {
	PBIID          string   // ID of the decomposed PBI
	SBICount       int      // Number of SBIs created (0 for dry-run)
	SBIFiles       []string // Paths to generated SBI files (empty for dry-run)
	PromptFilePath string   // Path to the generated prompt file
	Message        string   // Result message
	Prompt         string   // Generated prompt (populated in dry-run mode)
}

// DecomposePBIUseCase handles PBI decomposition into SBIs
type DecomposePBIUseCase struct {
	pbiRepo      pbi.Repository
	promptRepo   repository.PromptTemplateRepository
	approvalRepo repository.SBIApprovalRepository
	labelRepo    repository.LabelRepository // Label repository for loading label instructions
	agentGateway output.AgentGateway        // Agent gateway for AI execution (optional, can be nil for testing)
	workingDir   string                     // Base working directory (default: ".")
}

// NewDecomposePBIUseCase creates a new DecomposePBIUseCase instance
func NewDecomposePBIUseCase(
	pbiRepo pbi.Repository,
	promptRepo repository.PromptTemplateRepository,
	approvalRepo repository.SBIApprovalRepository,
	labelRepo repository.LabelRepository,
	agentGateway output.AgentGateway,
) *DecomposePBIUseCase {
	return &DecomposePBIUseCase{
		pbiRepo:      pbiRepo,
		promptRepo:   promptRepo,
		approvalRepo: approvalRepo,
		labelRepo:    labelRepo,
		agentGateway: agentGateway,
		workingDir:   ".", // Default to current directory
	}
}

// Execute decomposes a PBI into multiple SBIs
// This is the first half implementation focusing on:
// - PBI retrieval and validation
// - Prompt construction
// - Dry-run mode support
func (u *DecomposePBIUseCase) Execute(
	ctx context.Context,
	pbiID string,
	opts DecomposeOptions,
) (*DecomposeResult, error) {
	// 1. Retrieve PBI metadata
	pbiEntity, err := u.pbiRepo.FindByID(pbiID)
	if err != nil {
		return nil, fmt.Errorf("failed to find PBI %s: %w", pbiID, err)
	}

	// 2. Check if PBI can be decomposed
	if err := u.canDecompose(pbiEntity); err != nil {
		return nil, fmt.Errorf("PBI %s cannot be decomposed: %w", pbiID, err)
	}

	// 3. Retrieve PBI body content
	pbiBody, err := u.pbiRepo.GetBody(pbiID)
	if err != nil {
		return nil, fmt.Errorf("failed to get PBI body for %s: %w", pbiID, err)
	}

	// 4. Build decomposition prompt
	prompt, err := u.buildDecomposePrompt(ctx, pbiEntity, pbiBody, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to build decompose prompt: %w", err)
	}

	// 5. Write prompt to file (always write, even in dry-run mode)
	promptFilePath, err := u.writePromptFile(pbiID, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to write prompt file: %w", err)
	}

	// 6. Handle dry-run mode (return after writing prompt)
	if opts.DryRun {
		return &DecomposeResult{
			PBIID:          pbiID,
			SBICount:       0,
			SBIFiles:       []string{},
			PromptFilePath: promptFilePath,
			Message:        fmt.Sprintf("Prompt-only mode: prompt saved at %s", promptFilePath),
			Prompt:         prompt, // Also return prompt for display
		}, nil
	}

	// 7. Execute AI agent with prompt (if gateway is available)
	if u.agentGateway == nil {
		// No agent gateway available (e.g., in tests) - return prompt-only result
		log.Printf("Agent gateway not available (test mode)")
		return &DecomposeResult{
			PBIID:          pbiID,
			SBICount:       0,
			SBIFiles:       []string{},
			PromptFilePath: promptFilePath,
			Message:        fmt.Sprintf("Agent gateway not available. Prompt saved at: %s", promptFilePath),
			Prompt:         "",
		}, nil
	}

	// Health check - if Claude CLI is not available, return prompt-only result
	if err := u.agentGateway.(interface{ HealthCheck(context.Context) error }).HealthCheck(ctx); err != nil {
		log.Printf("Claude CLI not available: %v", err)
		return &DecomposeResult{
			PBIID:          pbiID,
			SBICount:       0,
			SBIFiles:       []string{},
			PromptFilePath: promptFilePath,
			Message:        fmt.Sprintf("Claude CLI not available. Prompt saved at: %s", promptFilePath),
			Prompt:         "",
		}, nil
	}

	// Execute AI agent
	agentReq := output.AgentRequest{
		Prompt:  prompt,
		Timeout: 10 * time.Minute,
	}

	_, err = u.agentGateway.Execute(ctx, agentReq)
	if err != nil {
		// AI execution failed, but prompt was saved - return partial success
		log.Printf("AI execution failed: %v", err)
		return &DecomposeResult{
			PBIID:          pbiID,
			SBICount:       0,
			SBIFiles:       []string{},
			PromptFilePath: promptFilePath,
			Message:        fmt.Sprintf("AI execution failed. Prompt saved at: %s", promptFilePath),
			Prompt:         "",
		}, nil
	}

	// 8. List generated SBI files
	sbiFiles, err := u.listGeneratedSBIs(pbiID)
	if err != nil {
		return nil, fmt.Errorf("failed to list generated SBIs: %w", err)
	}

	if len(sbiFiles) == 0 {
		return &DecomposeResult{
			PBIID:          pbiID,
			SBICount:       0,
			SBIFiles:       []string{},
			PromptFilePath: promptFilePath,
			Message:        "AI executed but no SBI files were generated",
			Prompt:         "",
		}, nil
	}

	// 9. Create approval.yaml manifest
	if err := u.createApprovalManifest(ctx, pbiID, sbiFiles); err != nil {
		return nil, fmt.Errorf("failed to create approval manifest: %w", err)
	}

	// 10. Update PBI status to planning
	if err := pbiEntity.UpdateStatus(pbi.StatusPlanning); err != nil {
		return nil, fmt.Errorf("failed to update PBI status: %w", err)
	}

	// 11. Save updated PBI (with existing body)
	if err := u.pbiRepo.Save(pbiEntity, pbiBody); err != nil {
		return nil, fmt.Errorf("failed to save PBI: %w", err)
	}

	// 12. Return success result
	return &DecomposeResult{
		PBIID:          pbiID,
		SBICount:       len(sbiFiles),
		SBIFiles:       sbiFiles,
		PromptFilePath: promptFilePath,
		Message:        fmt.Sprintf("Successfully generated %d SBI files", len(sbiFiles)),
		Prompt:         "",
	}, nil
}

// canDecompose checks if a PBI can be decomposed
// Only PBIs in "pending" or "planning" status can be decomposed
func (u *DecomposePBIUseCase) canDecompose(p *pbi.PBI) error {
	if p.Status != pbi.StatusPending && p.Status != pbi.StatusPlanning {
		return fmt.Errorf(
			"PBI must be in 'pending' or 'planning' status (current: %s)",
			p.Status,
		)
	}
	return nil
}

// buildDecomposePrompt constructs the decomposition prompt from template
func (u *DecomposePBIUseCase) buildDecomposePrompt(
	ctx context.Context,
	p *pbi.PBI,
	pbiBody string,
	opts DecomposeOptions,
) (string, error) {
	// 1. Load PBI decompose template
	tmplContent, err := u.promptRepo.LoadPBIDecomposeTemplate(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load PBI decompose template: %w", err)
	}

	// 2. Parse template
	tmpl, err := template.New("pbi_decompose").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// 3. Load label instructions
	labelInstructions, err := u.loadLabelInstructions(ctx)
	if err != nil {
		// Log error but don't fail - labels are optional
		fmt.Fprintf(os.Stderr, "Warning: Failed to load labels: %v\n", err)
		labelInstructions = ""
	}

	// 4. Prepare template data
	pbiDir := filepath.Join(".deespec", "specs", "pbi", p.ID)
	templateData := map[string]interface{}{
		"PBIID":             p.ID,
		"Title":             p.Title,
		"StoryPoints":       p.EstimatedStoryPoints,
		"Priority":          u.formatPriority(p.Priority),
		"PBIBody":           pbiBody,
		"MinSBIs":           opts.MinSBIs,
		"MaxSBIs":           opts.MaxSBIs,
		"PBIDir":            pbiDir,
		"LabelInstructions": labelInstructions,
	}

	// 5. Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// formatPriority converts Priority enum to human-readable string
func (u *DecomposePBIUseCase) formatPriority(priority pbi.Priority) string {
	// Use the String() method from pbi.Priority
	return priority.String()
}

// loadLabelInstructions loads and formats label information for the prompt
// Returns a formatted string containing label metadata for AI agent guidance
func (u *DecomposePBIUseCase) loadLabelInstructions(ctx context.Context) (string, error) {
	// Return empty if labelRepo is not available
	if u.labelRepo == nil {
		return "", fmt.Errorf("label repository not available")
	}

	// 1. Load active labels
	labels, err := u.labelRepo.FindActive(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load active labels: %w", err)
	}

	// 2. If no labels, return empty
	if len(labels) == 0 {
		return "", fmt.Errorf("no active labels found")
	}

	// 3. Format label information
	var buf strings.Builder
	buf.WriteString("## Available Labels\n\n")
	buf.WriteString("The following labels are available for categorizing SBIs:\n\n")

	for _, lbl := range labels {
		buf.WriteString(fmt.Sprintf("### %s\n", lbl.Name()))
		if lbl.Description() != "" {
			buf.WriteString(fmt.Sprintf("- **Description**: %s\n", lbl.Description()))
		}
		buf.WriteString(fmt.Sprintf("- **Priority**: %d\n", lbl.Priority()))

		// 4. Load and include first 10 lines from label template files
		templatePaths := lbl.TemplatePaths()
		if len(templatePaths) > 0 {
			buf.WriteString("- **Content Preview** (first 10 lines):\n")
			for _, templatePath := range templatePaths {
				// Read the template file
				content, err := os.ReadFile(templatePath)
				if err != nil {
					// Skip if file cannot be read, but log the error
					fmt.Fprintf(os.Stderr, "Warning: Failed to read template file %s: %v\n", templatePath, err)
					continue
				}

				// Extract first 10 lines
				lines := strings.Split(string(content), "\n")
				previewLines := lines
				if len(lines) > 10 {
					previewLines = lines[:10]
				}

				buf.WriteString("```\n")
				buf.WriteString(strings.Join(previewLines, "\n"))
				if len(lines) > 10 {
					buf.WriteString(fmt.Sprintf("\n... (truncated, %d more lines)\n", len(lines)-10))
				}
				buf.WriteString("\n```\n")
			}
		}

		buf.WriteString("\n")
	}

	buf.WriteString("\n**Instructions for AI Agent:**\n")
	buf.WriteString("- **MUST**: Each SBI file must include a `Labels:` line in the metadata section at the end\n")
	buf.WriteString("- Analyze each task and assign appropriate labels from the list above\n")
	buf.WriteString("- Review the content preview to understand what each label represents\n")
	buf.WriteString("- Each SBI can have multiple labels (comma-separated)\n")
	buf.WriteString("- Use label names exactly as shown above (case-sensitive)\n")
	buf.WriteString("- If no labels apply, write `Labels: none`\n")
	buf.WriteString("\n**Example metadata section:**\n")
	buf.WriteString("```\n")
	buf.WriteString("---\n")
	buf.WriteString("Parent PBI: PBI-001\n")
	buf.WriteString("Sequence: 1\n")
	buf.WriteString("Labels: security, backend\n")
	buf.WriteString("```\n")

	return buf.String(), nil
}

// writePromptFile writes the generated prompt to a file
// Returns the absolute path to the created file
func (u *DecomposePBIUseCase) writePromptFile(pbiID string, prompt string) (string, error) {
	// 1. Build output directory path
	pbiDir := filepath.Join(u.workingDir, ".deespec", "specs", "pbi", pbiID)

	// 2. Create directory if it doesn't exist (with parent directories)
	if err := os.MkdirAll(pbiDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", pbiDir, err)
	}

	// 3. Build prompt file path
	promptFilePath := filepath.Join(pbiDir, "decompose_prompt.md")

	// 4. Write prompt to file with appropriate permissions
	if err := os.WriteFile(promptFilePath, []byte(prompt), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", promptFilePath, err)
	}

	return promptFilePath, nil
}

// listGeneratedSBIs scans the PBI directory for generated SBI files
// Returns a list of SBI file paths matching the pattern sbi_*.md
func (u *DecomposePBIUseCase) listGeneratedSBIs(pbiID string) ([]string, error) {
	// 1. Build PBI directory path
	pbiDir := filepath.Join(u.workingDir, ".deespec", "specs", "pbi", pbiID)

	// 2. Build glob pattern for SBI files
	pattern := filepath.Join(pbiDir, "sbi_*.md")

	// 3. Find matching files
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list SBI files: %w", err)
	}

	// 4. Convert to relative paths for better readability
	var sbiFiles []string
	for _, match := range matches {
		// Get just the filename, not the full path
		sbiFiles = append(sbiFiles, filepath.Base(match))
	}

	return sbiFiles, nil
}

// createApprovalManifest creates an initial approval.yaml manifest
// for the generated SBI files
func (u *DecomposePBIUseCase) createApprovalManifest(
	ctx context.Context,
	pbiID string,
	sbiFiles []string,
) error {
	// 1. Create approval manifest with all SBIs in pending status
	manifest := pbi.NewSBIApprovalManifest(pbiID, sbiFiles)

	// 2. Save manifest using repository
	if err := u.approvalRepo.SaveManifest(ctx, manifest); err != nil {
		return fmt.Errorf("failed to save approval manifest: %w", err)
	}

	return nil
}

// ValidateSBIFile validates a single SBI file for required sections and metadata
// Returns an error if the file doesn't meet the deespec SBI format requirements
func (u *DecomposePBIUseCase) ValidateSBIFile(filePath string) error {
	// 1. Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	contentStr := string(content)

	// 2. Validate required sections
	requiredSections := []string{
		"## 概要",
		"## タスク詳細",
		"## 受け入れ基準",
		"## 推定工数",
	}

	missingSections := []string{}
	for _, section := range requiredSections {
		if !strings.Contains(contentStr, section) {
			missingSections = append(missingSections, section)
		}
	}

	if len(missingSections) > 0 {
		return fmt.Errorf(
			"missing required sections in %s: %s",
			filepath.Base(filePath),
			strings.Join(missingSections, ", "),
		)
	}

	// 3. Validate required metadata
	requiredMetadata := []string{
		"Parent PBI:",
		"Sequence:",
	}

	missingMetadata := []string{}
	for _, metadata := range requiredMetadata {
		if !strings.Contains(contentStr, metadata) {
			missingMetadata = append(missingMetadata, metadata)
		}
	}

	if len(missingMetadata) > 0 {
		return fmt.Errorf(
			"missing required metadata in %s: %s",
			filepath.Base(filePath),
			strings.Join(missingMetadata, ", "),
		)
	}

	return nil
}

// ValidateGeneratedSBIs validates all generated SBI files for a given PBI
// Returns a list of valid SBI file paths or an error if validation fails
func (u *DecomposePBIUseCase) ValidateGeneratedSBIs(pbiID string) ([]string, error) {
	// 1. Build PBI directory path
	pbiDir := filepath.Join(u.workingDir, ".deespec", "specs", "pbi", pbiID)

	// 2. Build glob pattern for SBI files
	pattern := filepath.Join(pbiDir, "sbi_*.md")

	// 3. Find matching files
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list SBI files: %w", err)
	}

	// 4. Check if any SBI files were found
	if len(matches) == 0 {
		return nil, fmt.Errorf("no SBI files found in %s (pattern: sbi_*.md)", pbiDir)
	}

	// 5. Validate each SBI file
	var validFiles []string
	var validationErrors []string

	for _, filePath := range matches {
		if err := u.ValidateSBIFile(filePath); err != nil {
			validationErrors = append(
				validationErrors,
				fmt.Sprintf("%s: %v", filepath.Base(filePath), err),
			)
		} else {
			validFiles = append(validFiles, filePath)
		}
	}

	// 6. Return error if any validations failed
	if len(validationErrors) > 0 {
		return nil, fmt.Errorf(
			"validation failed for %d/%d SBI files:\n%s",
			len(validationErrors),
			len(matches),
			strings.Join(validationErrors, "\n"),
		)
	}

	return validFiles, nil
}
