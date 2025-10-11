package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadTemplate_ValidStatuses tests LoadTemplate with all valid status values
func TestLoadTemplate_ValidStatuses(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Change to temp directory
	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/prompts directory
	promptsDir := filepath.Join(".deespec", "prompts")
	err = os.MkdirAll(promptsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Create test prompt files
	testFiles := map[string]string{
		"WIP.md":            "# WIP Prompt\nThis is the WIP prompt content.",
		"REVIEW.md":         "# Review Prompt\nThis is the review prompt content.",
		"REVIEW_AND_WIP.md": "# Review & WIP Prompt\nThis is the combined prompt.",
	}

	for filename, content := range testFiles {
		err = os.WriteFile(filepath.Join(promptsDir, filename), []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to write %s: %v", filename, err)
		}
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	// Test READY status (should load WIP.md)
	template, err := repo.LoadTemplate(ctx, "READY")
	if err != nil {
		t.Errorf("LoadTemplate(READY) failed: %v", err)
	}
	if template != "# WIP Prompt\nThis is the WIP prompt content." {
		t.Errorf("LoadTemplate(READY) returned wrong content: %s", template)
	}

	// Test WIP status (should load WIP.md)
	template, err = repo.LoadTemplate(ctx, "WIP")
	if err != nil {
		t.Errorf("LoadTemplate(WIP) failed: %v", err)
	}
	if template != "# WIP Prompt\nThis is the WIP prompt content." {
		t.Errorf("LoadTemplate(WIP) returned wrong content: %s", template)
	}

	// Test REVIEW status
	template, err = repo.LoadTemplate(ctx, "REVIEW")
	if err != nil {
		t.Errorf("LoadTemplate(REVIEW) failed: %v", err)
	}
	if template != "# Review Prompt\nThis is the review prompt content." {
		t.Errorf("LoadTemplate(REVIEW) returned wrong content: %s", template)
	}

	// Test REVIEW&WIP status
	template, err = repo.LoadTemplate(ctx, "REVIEW&WIP")
	if err != nil {
		t.Errorf("LoadTemplate(REVIEW&WIP) failed: %v", err)
	}
	if template != "# Review & WIP Prompt\nThis is the combined prompt." {
		t.Errorf("LoadTemplate(REVIEW&WIP) returned wrong content: %s", template)
	}
}

// TestLoadTemplate_UnknownStatus tests LoadTemplate with an invalid status
func TestLoadTemplate_UnknownStatus(t *testing.T) {
	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	_, err := repo.LoadTemplate(ctx, "INVALID_STATUS")
	if err == nil {
		t.Error("LoadTemplate should return error for unknown status")
	}
	if err.Error() != "unknown status: INVALID_STATUS" {
		t.Errorf("Expected 'unknown status: INVALID_STATUS', got '%s'", err.Error())
	}
}

// TestLoadTemplate_FileNotFound tests LoadTemplate when prompt file doesn't exist
func TestLoadTemplate_FileNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create directory but no files
	err = os.MkdirAll(filepath.Join(".deespec", "prompts"), 0755)
	if err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	_, err = repo.LoadTemplate(ctx, "WIP")
	if err == nil {
		t.Error("LoadTemplate should return error when file doesn't exist")
	}
}

// TestLoadLabelContent_SimpleLabel tests LoadLabelContent with a simple label name
func TestLoadLabelContent_SimpleLabel(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create label directory and file
	labelsDir := filepath.Join(".deespec", "prompts", "labels")
	err = os.MkdirAll(labelsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create labels directory: %v", err)
	}

	labelContent := "# Backend Label\nThis is backend-specific guidance."
	err = os.WriteFile(filepath.Join(labelsDir, "backend.md"), []byte(labelContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write label file: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	content := repo.LoadLabelContent(ctx, "backend")
	if content != labelContent {
		t.Errorf("Expected '%s', got '%s'", labelContent, content)
	}
}

// TestLoadLabelContent_HierarchicalLabel tests LoadLabelContent with nested label paths
func TestLoadLabelContent_HierarchicalLabel(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create hierarchical label directory
	labelsDir := filepath.Join(".deespec", "prompts", "labels", "frontend")
	err = os.MkdirAll(labelsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create labels directory: %v", err)
	}

	labelContent := "# Frontend Architecture\nArchitecture guidance for frontend."
	err = os.WriteFile(filepath.Join(labelsDir, "architecture.md"), []byte(labelContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write label file: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	content := repo.LoadLabelContent(ctx, "frontend/architecture")
	if content != labelContent {
		t.Errorf("Expected hierarchical label content, got '%s'", content)
	}
}

// TestLoadLabelContent_ClaudeDirectory tests LoadLabelContent fallback to .claude directory
func TestLoadLabelContent_ClaudeDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .claude directory with label
	claudeDir := ".claude"
	err = os.MkdirAll(claudeDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	labelContent := "# Claude Label\nLabel from .claude directory."
	err = os.WriteFile(filepath.Join(claudeDir, "custom-label.md"), []byte(labelContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write label file: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	content := repo.LoadLabelContent(ctx, "custom-label.md")
	if content != labelContent {
		t.Errorf("Expected label from .claude directory, got '%s'", content)
	}
}

// TestLoadLabelContent_NotFound tests LoadLabelContent when label doesn't exist
func TestLoadLabelContent_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	// Should return empty string when label doesn't exist
	content := repo.LoadLabelContent(ctx, "nonexistent")
	if content != "" {
		t.Errorf("Expected empty string for nonexistent label, got '%s'", content)
	}
}

// TestLoadMetaLabels_ValidMetaYml tests LoadMetaLabels with a valid meta.yml file
func TestLoadMetaLabels_ValidMetaYml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/sbi-id directory
	sbiID := "test-sbi-123"
	sbiDir := filepath.Join(".deespec", sbiID)
	err = os.MkdirAll(sbiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create SBI directory: %v", err)
	}

	// Create meta.yml with labels
	metaContent := `title: Test Task
status: WIP
labels:
  - backend
  - api
  - performance
priority: high
`
	err = os.WriteFile(filepath.Join(sbiDir, "meta.yml"), []byte(metaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write meta.yml: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("LoadMetaLabels failed: %v", err)
	}

	expectedLabels := []string{"backend", "api", "performance"}
	if len(labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
	}

	for i, expected := range expectedLabels {
		if i >= len(labels) || labels[i] != expected {
			t.Errorf("Expected label[%d] = '%s', got '%s'", i, expected, labels[i])
		}
	}
}

// TestLoadMetaLabels_ValidMetaYaml tests LoadMetaLabels with a valid meta.yaml file
func TestLoadMetaLabels_ValidMetaYaml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/sbi-id directory
	sbiID := "test-sbi-456"
	sbiDir := filepath.Join(".deespec", sbiID)
	err = os.MkdirAll(sbiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create SBI directory: %v", err)
	}

	// Create meta.yaml (not meta.yml) with labels
	metaContent := `title: Another Test
labels:
  - frontend
  - ui
`
	err = os.WriteFile(filepath.Join(sbiDir, "meta.yaml"), []byte(metaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write meta.yaml: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("LoadMetaLabels failed: %v", err)
	}

	expectedLabels := []string{"frontend", "ui"}
	if len(labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
	}

	for i, expected := range expectedLabels {
		if i >= len(labels) || labels[i] != expected {
			t.Errorf("Expected label[%d] = '%s', got '%s'", i, expected, labels[i])
		}
	}
}

// TestLoadMetaLabels_EmptyLabels tests LoadMetaLabels with a meta file without labels
func TestLoadMetaLabels_EmptyLabels(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/sbi-id directory
	sbiID := "test-sbi-789"
	sbiDir := filepath.Join(".deespec", sbiID)
	err = os.MkdirAll(sbiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create SBI directory: %v", err)
	}

	// Create meta.yml without labels section
	metaContent := `title: Test Without Labels
status: WIP
priority: medium
`
	err = os.WriteFile(filepath.Join(sbiDir, "meta.yml"), []byte(metaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write meta.yml: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("LoadMetaLabels failed: %v", err)
	}

	if len(labels) != 0 {
		t.Errorf("Expected 0 labels, got %d", len(labels))
	}
}

// TestLoadMetaLabels_EmptyLabelsList tests LoadMetaLabels with empty labels list
func TestLoadMetaLabels_EmptyLabelsList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/sbi-id directory
	sbiID := "test-sbi-empty"
	sbiDir := filepath.Join(".deespec", sbiID)
	err = os.MkdirAll(sbiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create SBI directory: %v", err)
	}

	// Create meta.yml with empty labels section
	metaContent := `title: Test With Empty Labels
labels:
status: WIP
`
	err = os.WriteFile(filepath.Join(sbiDir, "meta.yml"), []byte(metaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write meta.yml: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("LoadMetaLabels failed: %v", err)
	}

	if len(labels) != 0 {
		t.Errorf("Expected 0 labels for empty list, got %d", len(labels))
	}
}

// TestLoadMetaLabels_MetaFileNotFound tests LoadMetaLabels when meta file doesn't exist
func TestLoadMetaLabels_MetaFileNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	_, err = repo.LoadMetaLabels(ctx, "nonexistent-sbi")
	if err == nil {
		t.Error("LoadMetaLabels should return error when meta file doesn't exist")
	}
	if err.Error() != "meta file not found for nonexistent-sbi" {
		t.Errorf("Expected 'meta file not found' error, got '%s'", err.Error())
	}
}

// TestLoadMetaLabels_LabelsWithWhitespace tests LoadMetaLabels handling of labels with extra whitespace
func TestLoadMetaLabels_LabelsWithWhitespace(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/sbi-id directory
	sbiID := "test-sbi-whitespace"
	sbiDir := filepath.Join(".deespec", sbiID)
	err = os.MkdirAll(sbiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create SBI directory: %v", err)
	}

	// Create meta.yml with labels that have extra whitespace
	metaContent := `title: Test Whitespace
labels:
  - backend
  -   frontend
  - api
next_field: value
`
	err = os.WriteFile(filepath.Join(sbiDir, "meta.yml"), []byte(metaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write meta.yml: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("LoadMetaLabels failed: %v", err)
	}

	expectedLabels := []string{"backend", "frontend", "api"}
	if len(labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
	}

	for i, expected := range expectedLabels {
		if i >= len(labels) || labels[i] != expected {
			t.Errorf("Expected label[%d] = '%s', got '%s'", i, expected, labels[i])
		}
	}
}

// TestLoadMetaLabels_ComplexYaml tests LoadMetaLabels with complex YAML structure
func TestLoadMetaLabels_ComplexYaml(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/sbi-id directory
	sbiID := "test-sbi-complex"
	sbiDir := filepath.Join(".deespec", sbiID)
	err = os.MkdirAll(sbiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create SBI directory: %v", err)
	}

	// Create meta.yml with complex structure
	metaContent := `title: Complex Test
description: |
  Multi-line description
  with various content
labels:
  - security
  - testing
  - documentation
metadata:
  created: 2025-01-01
  tags:
    - important
    - urgent
`
	err = os.WriteFile(filepath.Join(sbiDir, "meta.yml"), []byte(metaContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write meta.yml: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("LoadMetaLabels failed: %v", err)
	}

	expectedLabels := []string{"security", "testing", "documentation"}
	if len(labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
	}

	for i, expected := range expectedLabels {
		if i >= len(labels) || labels[i] != expected {
			t.Errorf("Expected label[%d] = '%s', got '%s'", i, expected, labels[i])
		}
	}
}

// TestLoadTemplate_EmptyFile tests LoadTemplate with an empty prompt file
func TestLoadTemplate_EmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/prompts directory
	promptsDir := filepath.Join(".deespec", "prompts")
	err = os.MkdirAll(promptsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Create empty WIP.md file
	err = os.WriteFile(filepath.Join(promptsDir, "WIP.md"), []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write empty file: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	template, err := repo.LoadTemplate(ctx, "WIP")
	if err != nil {
		t.Errorf("LoadTemplate should succeed with empty file: %v", err)
	}
	if template != "" {
		t.Errorf("Expected empty string, got '%s'", template)
	}
}

// TestLoadPBIDecomposeTemplate_Success tests successful loading of PBI_DECOMPOSE.md
func TestLoadPBIDecomposeTemplate_Success(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/prompts directory
	promptsDir := filepath.Join(".deespec", "prompts")
	err = os.MkdirAll(promptsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Create PBI_DECOMPOSE.md with template content
	templateContent := `# PBI Decomposition Template
PBI ID: {{.PBIID}}
Title: {{.Title}}
`
	err = os.WriteFile(filepath.Join(promptsDir, "PBI_DECOMPOSE.md"), []byte(templateContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write PBI_DECOMPOSE.md: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	template, err := repo.LoadPBIDecomposeTemplate(ctx)
	if err != nil {
		t.Errorf("LoadPBIDecomposeTemplate failed: %v", err)
	}
	if template != templateContent {
		t.Errorf("Expected template content, got: %s", template)
	}
}

// TestLoadPBIDecomposeTemplate_FileNotFound tests error handling when PBI_DECOMPOSE.md doesn't exist
func TestLoadPBIDecomposeTemplate_FileNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create directory but no PBI_DECOMPOSE.md file
	err = os.MkdirAll(filepath.Join(".deespec", "prompts"), 0755)
	if err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	_, err = repo.LoadPBIDecomposeTemplate(ctx)
	if err == nil {
		t.Error("LoadPBIDecomposeTemplate should return error when file doesn't exist")
	}
	if !strings.Contains(err.Error(), "PBI decompose template not found") {
		t.Errorf("Expected 'PBI decompose template not found' error, got: %s", err.Error())
	}
	if !strings.Contains(err.Error(), "ensure 'deespec init' has been run") {
		t.Errorf("Expected helpful error message mentioning 'deespec init', got: %s", err.Error())
	}
}

// TestLoadPBIDecomposeTemplate_EmptyFile tests loading an empty PBI_DECOMPOSE.md
func TestLoadPBIDecomposeTemplate_EmptyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create .deespec/prompts directory
	promptsDir := filepath.Join(".deespec", "prompts")
	err = os.MkdirAll(promptsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create prompts directory: %v", err)
	}

	// Create empty PBI_DECOMPOSE.md
	err = os.WriteFile(filepath.Join(promptsDir, "PBI_DECOMPOSE.md"), []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to write empty PBI_DECOMPOSE.md: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	template, err := repo.LoadPBIDecomposeTemplate(ctx)
	if err != nil {
		t.Errorf("LoadPBIDecomposeTemplate should succeed with empty file: %v", err)
	}
	if template != "" {
		t.Errorf("Expected empty string, got: %s", template)
	}
}

// TestLoadLabelContent_MultipleDirectoryFallback tests that LoadLabelContent tries multiple directories in order
func TestLoadLabelContent_MultipleDirectoryFallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "prompt_template_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	originalDir, _ := os.Getwd()
	err = os.Chdir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer os.Chdir(originalDir)

	// Create label in .deespec/prompts/labels (second fallback directory)
	labelsDir := filepath.Join(".deespec", "prompts", "labels")
	err = os.MkdirAll(labelsDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create labels directory: %v", err)
	}

	labelContent := "# Fallback Label\nFrom .deespec/prompts/labels."
	err = os.WriteFile(filepath.Join(labelsDir, "fallback-test.md"), []byte(labelContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write label file: %v", err)
	}

	repo := NewPromptTemplateRepositoryImpl()
	ctx := context.Background()

	// Should find the file by trying the full path with fallback directories
	content := repo.LoadLabelContent(ctx, "fallback-test.md")
	if content != labelContent {
		t.Errorf("Expected label from fallback directory, got '%s'", content)
	}
}
