package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// PromptTemplateRepositoryImpl implements PromptTemplateRepository for file-based storage
type PromptTemplateRepositoryImpl struct{}

// NewPromptTemplateRepositoryImpl creates a new file-based prompt template repository
func NewPromptTemplateRepositoryImpl() repository.PromptTemplateRepository {
	return &PromptTemplateRepositoryImpl{}
}

// LoadTemplate loads a prompt template based on status
func (r *PromptTemplateRepositoryImpl) LoadTemplate(ctx context.Context, status string) (string, error) {
	// Determine prompt file name based on status
	var promptFile string
	switch status {
	case "READY", "WIP":
		promptFile = "WIP.md"
	case "REVIEW":
		promptFile = "REVIEW.md"
	case "REVIEW&WIP":
		promptFile = "REVIEW_AND_WIP.md"
	default:
		return "", fmt.Errorf("unknown status: %s", status)
	}

	// Try to load from .deespec/prompts/ directory
	promptPath := filepath.Join(".deespec", "prompts", promptFile)
	template, err := os.ReadFile(promptPath)
	if err != nil {
		// File doesn't exist or can't be read
		return "", err
	}

	return string(template), nil
}

// LoadLabelContent loads the content for a specific label
func (r *PromptTemplateRepositoryImpl) LoadLabelContent(ctx context.Context, labelName string) string {
	// Try hierarchical label paths (e.g., frontend/architecture)
	labelPath := filepath.Join(".deespec", "prompts", "labels", labelName+".md")

	content, err := os.ReadFile(labelPath)
	if err == nil {
		return string(content)
	}

	// Try direct template path (for when labelName is already a full path)
	// Support both .claude and .deespec/prompts/labels directories
	for _, dir := range []string{".claude", ".deespec/prompts/labels"} {
		fullPath := filepath.Join(dir, labelName)
		content, err := os.ReadFile(fullPath)
		if err == nil {
			return string(content)
		}
	}

	return ""
}

// LoadMetaLabels loads labels from a task's meta.yaml file
func (r *PromptTemplateRepositoryImpl) LoadMetaLabels(ctx context.Context, sbiID string) ([]string, error) {
	// Try meta.yml first, then meta.yaml as fallback
	metaPath := filepath.Join(".deespec", sbiID, "meta.yml")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		metaPath = filepath.Join(".deespec", sbiID, "meta.yaml")
		if _, err := os.Stat(metaPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("meta file not found for %s", sbiID)
		}
	}

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read meta file: %w", err)
	}

	// Manual simple parsing to extract labels
	lines := strings.Split(string(data), "\n")
	var inLabels bool
	var labels []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "labels:") {
			inLabels = true
			continue
		}
		if inLabels {
			if strings.HasPrefix(trimmed, "- ") {
				label := strings.TrimPrefix(trimmed, "- ")
				label = strings.TrimSpace(label)
				labels = append(labels, label)
			} else if trimmed != "" && !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "-") {
				// Next field started
				break
			}
		}
	}

	return labels, nil
}
