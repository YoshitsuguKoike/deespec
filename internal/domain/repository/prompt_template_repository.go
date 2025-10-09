package repository

import "context"

// PromptTemplateRepository defines operations for loading prompt templates
type PromptTemplateRepository interface {
	// LoadTemplate loads a prompt template based on status
	// Returns the template content or error if not found
	LoadTemplate(ctx context.Context, status string) (string, error)

	// LoadLabelContent loads the content for a specific label
	// Returns empty string if label file doesn't exist
	LoadLabelContent(ctx context.Context, labelName string) string

	// LoadMetaLabels loads labels from a task's meta.yaml file
	// Returns empty slice if meta file doesn't exist or has no labels
	LoadMetaLabels(ctx context.Context, sbiID string) ([]string, error)
}
