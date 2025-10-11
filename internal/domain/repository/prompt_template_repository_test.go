package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// MockPromptTemplateRepository is a mock implementation of PromptTemplateRepository for testing
type MockPromptTemplateRepository struct {
	mu                      sync.RWMutex
	templates               map[string]string
	labels                  map[string]string
	metaFiles               map[string][]string
	pbiDecomposeTemplate    string
	pbiDecomposeTemplateSet bool
}

// NewMockPromptTemplateRepository creates a new mock prompt template repository
func NewMockPromptTemplateRepository() *MockPromptTemplateRepository {
	return &MockPromptTemplateRepository{
		templates: make(map[string]string),
		labels:    make(map[string]string),
		metaFiles: make(map[string][]string),
	}
}

// LoadTemplate loads a prompt template based on status
func (m *MockPromptTemplateRepository) LoadTemplate(ctx context.Context, status string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Validate status
	validStatuses := map[string]bool{
		"READY":      true,
		"WIP":        true,
		"REVIEW":     true,
		"REVIEW&WIP": true,
	}

	if !validStatuses[status] {
		return "", errors.New("unknown status: " + status)
	}

	// Return template if exists
	if template, exists := m.templates[status]; exists {
		return template, nil
	}

	return "", errors.New("template not found for status: " + status)
}

// LoadLabelContent loads the content for a specific label
func (m *MockPromptTemplateRepository) LoadLabelContent(ctx context.Context, labelName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return label content if exists, otherwise return empty string
	if content, exists := m.labels[labelName]; exists {
		return content
	}

	return ""
}

// LoadMetaLabels loads labels from a task's meta.yaml file
func (m *MockPromptTemplateRepository) LoadMetaLabels(ctx context.Context, sbiID string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return labels if meta file exists
	if labels, exists := m.metaFiles[sbiID]; exists {
		return labels, nil
	}

	return nil, errors.New("meta file not found for " + sbiID)
}

// LoadPBIDecomposeTemplate loads the PBI decomposition prompt template
func (m *MockPromptTemplateRepository) LoadPBIDecomposeTemplate(ctx context.Context) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.pbiDecomposeTemplateSet {
		return "", errors.New("PBI decompose template not set")
	}

	return m.pbiDecomposeTemplate, nil
}

// Helper methods for setting up test data

func (m *MockPromptTemplateRepository) SetTemplate(status, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.templates[status] = content
}

func (m *MockPromptTemplateRepository) SetLabel(labelName, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.labels[labelName] = content
}

func (m *MockPromptTemplateRepository) SetMetaLabels(sbiID string, labels []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metaFiles[sbiID] = labels
}

func (m *MockPromptTemplateRepository) SetPBIDecomposeTemplate(content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pbiDecomposeTemplate = content
	m.pbiDecomposeTemplateSet = true
}

// Test Suite for PromptTemplateRepository

func TestPromptTemplateRepository_LoadTemplate(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up test templates
	repo.SetTemplate("WIP", "# WIP Prompt\nImplementation guidance")
	repo.SetTemplate("REVIEW", "# Review Prompt\nReview guidance")

	// Test loading WIP template
	template, err := repo.LoadTemplate(ctx, "WIP")
	if err != nil {
		t.Fatalf("Failed to load WIP template: %v", err)
	}

	if template != "# WIP Prompt\nImplementation guidance" {
		t.Errorf("Expected WIP template content, got: %s", template)
	}

	// Test loading REVIEW template
	template, err = repo.LoadTemplate(ctx, "REVIEW")
	if err != nil {
		t.Fatalf("Failed to load REVIEW template: %v", err)
	}

	if template != "# Review Prompt\nReview guidance" {
		t.Errorf("Expected REVIEW template content, got: %s", template)
	}
}

func TestPromptTemplateRepository_LoadTemplateInvalidStatus(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Test with invalid status
	_, err := repo.LoadTemplate(ctx, "INVALID_STATUS")
	if err == nil {
		t.Error("Expected error for invalid status")
	}

	expectedError := "unknown status: INVALID_STATUS"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestPromptTemplateRepository_LoadTemplateNotFound(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Test with valid status but no template set
	_, err := repo.LoadTemplate(ctx, "WIP")
	if err == nil {
		t.Error("Expected error when template not found")
	}

	if err.Error() != "template not found for status: WIP" {
		t.Errorf("Expected 'template not found' error, got: %s", err.Error())
	}
}

func TestPromptTemplateRepository_LoadTemplateAllValidStatuses(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up templates for all valid statuses
	statuses := []string{"READY", "WIP", "REVIEW", "REVIEW&WIP"}
	for _, status := range statuses {
		repo.SetTemplate(status, "Template for "+status)
	}

	// Test loading each status
	for _, status := range statuses {
		template, err := repo.LoadTemplate(ctx, status)
		if err != nil {
			t.Errorf("Failed to load template for status %s: %v", status, err)
		}

		expectedContent := "Template for " + status
		if template != expectedContent {
			t.Errorf("For status %s, expected '%s', got '%s'", status, expectedContent, template)
		}
	}
}

func TestPromptTemplateRepository_LoadLabelContent(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up test labels
	repo.SetLabel("backend", "# Backend Guidelines\nBackend development guidance")
	repo.SetLabel("frontend", "# Frontend Guidelines\nFrontend development guidance")

	// Test loading backend label
	content := repo.LoadLabelContent(ctx, "backend")
	if content != "# Backend Guidelines\nBackend development guidance" {
		t.Errorf("Expected backend label content, got: %s", content)
	}

	// Test loading frontend label
	content = repo.LoadLabelContent(ctx, "frontend")
	if content != "# Frontend Guidelines\nFrontend development guidance" {
		t.Errorf("Expected frontend label content, got: %s", content)
	}
}

func TestPromptTemplateRepository_LoadLabelContentNotFound(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Test loading non-existent label
	content := repo.LoadLabelContent(ctx, "nonexistent")
	if content != "" {
		t.Errorf("Expected empty string for non-existent label, got: %s", content)
	}
}

func TestPromptTemplateRepository_LoadLabelContentHierarchical(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up hierarchical labels
	repo.SetLabel("frontend/architecture", "# Frontend Architecture\nArchitecture guidance")
	repo.SetLabel("backend/api", "# Backend API\nAPI development guidance")

	// Test loading hierarchical labels
	content := repo.LoadLabelContent(ctx, "frontend/architecture")
	if content != "# Frontend Architecture\nArchitecture guidance" {
		t.Errorf("Expected frontend/architecture label content, got: %s", content)
	}

	content = repo.LoadLabelContent(ctx, "backend/api")
	if content != "# Backend API\nAPI development guidance" {
		t.Errorf("Expected backend/api label content, got: %s", content)
	}
}

func TestPromptTemplateRepository_LoadLabelContentEmptyString(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up label with empty content
	repo.SetLabel("empty-label", "")

	// Test loading label with empty content
	content := repo.LoadLabelContent(ctx, "empty-label")
	if content != "" {
		t.Errorf("Expected empty string, got: %s", content)
	}
}

func TestPromptTemplateRepository_LoadMetaLabels(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up test meta labels
	sbiID := "test-sbi-001"
	expectedLabels := []string{"backend", "api", "performance"}
	repo.SetMetaLabels(sbiID, expectedLabels)

	// Test loading meta labels
	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to load meta labels: %v", err)
	}

	if len(labels) != len(expectedLabels) {
		t.Errorf("Expected %d labels, got %d", len(expectedLabels), len(labels))
	}

	for i, label := range expectedLabels {
		if labels[i] != label {
			t.Errorf("Expected label[%d] = '%s', got '%s'", i, label, labels[i])
		}
	}
}

func TestPromptTemplateRepository_LoadMetaLabelsNotFound(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Test loading meta labels for non-existent SBI
	_, err := repo.LoadMetaLabels(ctx, "nonexistent-sbi")
	if err == nil {
		t.Error("Expected error when meta file not found")
	}

	expectedError := "meta file not found for nonexistent-sbi"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

func TestPromptTemplateRepository_LoadMetaLabelsEmpty(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up SBI with empty labels
	sbiID := "test-sbi-empty"
	repo.SetMetaLabels(sbiID, []string{})

	// Test loading empty labels
	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to load empty meta labels: %v", err)
	}

	if len(labels) != 0 {
		t.Errorf("Expected 0 labels, got %d", len(labels))
	}
}

func TestPromptTemplateRepository_LoadMetaLabelsMultiple(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up multiple SBIs with different labels
	testCases := []struct {
		sbiID  string
		labels []string
	}{
		{
			sbiID:  "sbi-001",
			labels: []string{"backend", "api"},
		},
		{
			sbiID:  "sbi-002",
			labels: []string{"frontend", "ui", "react"},
		},
		{
			sbiID:  "sbi-003",
			labels: []string{"testing"},
		},
	}

	for _, tc := range testCases {
		repo.SetMetaLabels(tc.sbiID, tc.labels)
	}

	// Test loading labels for each SBI
	for _, tc := range testCases {
		labels, err := repo.LoadMetaLabels(ctx, tc.sbiID)
		if err != nil {
			t.Errorf("Failed to load meta labels for %s: %v", tc.sbiID, err)
			continue
		}

		if len(labels) != len(tc.labels) {
			t.Errorf("For %s: expected %d labels, got %d", tc.sbiID, len(tc.labels), len(labels))
			continue
		}

		for i, expectedLabel := range tc.labels {
			if labels[i] != expectedLabel {
				t.Errorf("For %s: expected label[%d] = '%s', got '%s'", tc.sbiID, i, expectedLabel, labels[i])
			}
		}
	}
}

func TestPromptTemplateRepository_LoadMetaLabelsWithWhitespace(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up labels that might have whitespace (simulating YAML parsing)
	sbiID := "test-sbi-whitespace"
	labels := []string{"backend", "frontend", "api"}
	repo.SetMetaLabels(sbiID, labels)

	// Test loading labels
	result, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to load meta labels: %v", err)
	}

	// Verify labels are trimmed
	for i, label := range result {
		if label != labels[i] {
			t.Errorf("Expected label[%d] = '%s', got '%s'", i, labels[i], label)
		}
	}
}

func TestPromptTemplateRepository_ConcurrentTemplateAccess(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up templates
	repo.SetTemplate("WIP", "WIP Template Content")
	repo.SetTemplate("REVIEW", "REVIEW Template Content")

	var wg sync.WaitGroup
	errorChan := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			status := "WIP"
			if index%2 == 0 {
				status = "REVIEW"
			}

			_, err := repo.LoadTemplate(ctx, status)
			if err != nil {
				errorChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent template access failed: %v", err)
	}
}

func TestPromptTemplateRepository_ConcurrentLabelAccess(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up labels
	repo.SetLabel("backend", "Backend Content")
	repo.SetLabel("frontend", "Frontend Content")
	repo.SetLabel("api", "API Content")

	var wg sync.WaitGroup
	errorChan := make(chan error, 150)

	// Concurrent reads
	labels := []string{"backend", "frontend", "api"}
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			labelName := labels[index%len(labels)]
			content := repo.LoadLabelContent(ctx, labelName)
			if content == "" && labelName != "nonexistent" {
				errorChan <- errors.New("expected non-empty content for " + labelName)
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent label access failed: %v", err)
	}
}

func TestPromptTemplateRepository_ConcurrentMetaLabelsAccess(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up meta labels for multiple SBIs
	for i := 0; i < 10; i++ {
		sbiID := "test-sbi-" + string(rune('A'+i))
		labels := []string{"label1", "label2"}
		repo.SetMetaLabels(sbiID, labels)
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, 100)

	// Concurrent reads
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			sbiID := "test-sbi-" + string(rune('A'+(index%10)))
			_, err := repo.LoadMetaLabels(ctx, sbiID)
			if err != nil {
				errorChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Errorf("Concurrent meta labels access failed: %v", err)
	}
}

func TestPromptTemplateRepository_LoadTemplateEmptyContent(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up template with empty content
	repo.SetTemplate("WIP", "")

	// Test loading template with empty content
	template, err := repo.LoadTemplate(ctx, "WIP")
	if err != nil {
		t.Fatalf("Failed to load empty template: %v", err)
	}

	if template != "" {
		t.Errorf("Expected empty string, got: %s", template)
	}
}

func TestPromptTemplateRepository_LoadTemplateLargeContent(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Create large template content
	largeContent := ""
	for i := 0; i < 1000; i++ {
		largeContent += "This is line " + string(rune('0'+(i%10))) + " of the template.\n"
	}

	repo.SetTemplate("WIP", largeContent)

	// Test loading large template
	template, err := repo.LoadTemplate(ctx, "WIP")
	if err != nil {
		t.Fatalf("Failed to load large template: %v", err)
	}

	if template != largeContent {
		t.Error("Large template content was not preserved correctly")
	}
}

func TestPromptTemplateRepository_LoadLabelContentSpecialCharacters(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up labels with special characters
	specialLabels := map[string]string{
		"label-with-dashes":      "Content for dashed label",
		"label_with_underscores": "Content for underscored label",
		"label.with.dots":        "Content for dotted label",
		"label/with/slashes":     "Content for slashed label",
	}

	for labelName, content := range specialLabels {
		repo.SetLabel(labelName, content)
	}

	// Test loading each label
	for labelName, expectedContent := range specialLabels {
		content := repo.LoadLabelContent(ctx, labelName)
		if content != expectedContent {
			t.Errorf("For label '%s': expected '%s', got '%s'", labelName, expectedContent, content)
		}
	}
}

func TestPromptTemplateRepository_LoadMetaLabelsUnicodeContent(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up meta labels with Unicode content
	sbiID := "test-sbi-unicode"
	unicodeLabels := []string{"バックエンド", "フロントエンド", "API開発", "テスト"}
	repo.SetMetaLabels(sbiID, unicodeLabels)

	// Test loading Unicode labels
	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to load Unicode meta labels: %v", err)
	}

	if len(labels) != len(unicodeLabels) {
		t.Errorf("Expected %d labels, got %d", len(unicodeLabels), len(labels))
	}

	for i, expectedLabel := range unicodeLabels {
		if labels[i] != expectedLabel {
			t.Errorf("Expected label[%d] = '%s', got '%s'", i, expectedLabel, labels[i])
		}
	}
}

func TestPromptTemplateRepository_LoadTemplateMultipleOverwrites(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set template multiple times
	repo.SetTemplate("WIP", "First version")
	repo.SetTemplate("WIP", "Second version")
	repo.SetTemplate("WIP", "Third version")

	// Test loading template should return latest version
	template, err := repo.LoadTemplate(ctx, "WIP")
	if err != nil {
		t.Fatalf("Failed to load template: %v", err)
	}

	if template != "Third version" {
		t.Errorf("Expected 'Third version', got: %s", template)
	}
}

func TestPromptTemplateRepository_LoadMetaLabelsSingleLabel(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up SBI with single label
	sbiID := "test-sbi-single"
	repo.SetMetaLabels(sbiID, []string{"backend"})

	// Test loading single label
	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to load meta labels: %v", err)
	}

	if len(labels) != 1 {
		t.Errorf("Expected 1 label, got %d", len(labels))
	}

	if labels[0] != "backend" {
		t.Errorf("Expected 'backend', got '%s'", labels[0])
	}
}

func TestPromptTemplateRepository_LoadMetaLabelsManyLabels(t *testing.T) {
	repo := NewMockPromptTemplateRepository()
	ctx := context.Background()

	// Set up SBI with many labels
	sbiID := "test-sbi-many"
	manyLabels := make([]string, 50)
	for i := 0; i < 50; i++ {
		manyLabels[i] = "label-" + string(rune('A'+(i%26)))
	}
	repo.SetMetaLabels(sbiID, manyLabels)

	// Test loading many labels
	labels, err := repo.LoadMetaLabels(ctx, sbiID)
	if err != nil {
		t.Fatalf("Failed to load meta labels: %v", err)
	}

	if len(labels) != 50 {
		t.Errorf("Expected 50 labels, got %d", len(labels))
	}

	for i := 0; i < 50; i++ {
		if labels[i] != manyLabels[i] {
			t.Errorf("Label mismatch at index %d", i)
		}
	}
}

func TestPromptTemplateRepository_InterfaceContractLoadTemplate(t *testing.T) {
	// This test verifies the contract of LoadTemplate method
	var repo repository.PromptTemplateRepository = NewMockPromptTemplateRepository()
	ctx := context.Background()

	mockRepo := repo.(*MockPromptTemplateRepository)
	mockRepo.SetTemplate("WIP", "Test Content")

	// Test that the interface method works correctly
	content, err := repo.LoadTemplate(ctx, "WIP")
	if err != nil {
		t.Fatalf("Interface method LoadTemplate failed: %v", err)
	}

	if content != "Test Content" {
		t.Errorf("Interface contract violated: expected 'Test Content', got '%s'", content)
	}
}

func TestPromptTemplateRepository_InterfaceContractLoadLabelContent(t *testing.T) {
	// This test verifies the contract of LoadLabelContent method
	var repo repository.PromptTemplateRepository = NewMockPromptTemplateRepository()
	ctx := context.Background()

	mockRepo := repo.(*MockPromptTemplateRepository)
	mockRepo.SetLabel("backend", "Backend Guidelines")

	// Test that the interface method works correctly
	content := repo.LoadLabelContent(ctx, "backend")
	if content != "Backend Guidelines" {
		t.Errorf("Interface contract violated: expected 'Backend Guidelines', got '%s'", content)
	}

	// Test with non-existent label should return empty string
	emptyContent := repo.LoadLabelContent(ctx, "nonexistent")
	if emptyContent != "" {
		t.Errorf("Interface contract violated: expected empty string, got '%s'", emptyContent)
	}
}

func TestPromptTemplateRepository_InterfaceContractLoadMetaLabels(t *testing.T) {
	// This test verifies the contract of LoadMetaLabels method
	var repo repository.PromptTemplateRepository = NewMockPromptTemplateRepository()
	ctx := context.Background()

	mockRepo := repo.(*MockPromptTemplateRepository)
	expectedLabels := []string{"label1", "label2", "label3"}
	mockRepo.SetMetaLabels("test-sbi", expectedLabels)

	// Test that the interface method works correctly
	labels, err := repo.LoadMetaLabels(ctx, "test-sbi")
	if err != nil {
		t.Fatalf("Interface method LoadMetaLabels failed: %v", err)
	}

	if len(labels) != len(expectedLabels) {
		t.Errorf("Interface contract violated: expected %d labels, got %d", len(expectedLabels), len(labels))
	}

	// Test with non-existent SBI should return error
	_, err = repo.LoadMetaLabels(ctx, "nonexistent")
	if err == nil {
		t.Error("Interface contract violated: expected error for non-existent SBI")
	}
}
