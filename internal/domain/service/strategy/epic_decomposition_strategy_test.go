package strategy

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
)

// MockAgentExecutor is a mock implementation of AgentExecutor for testing
type MockAgentExecutor struct {
	ExecuteFunc func(ctx context.Context, prompt string, taskType model.TaskType) (string, error)
	callCount   int
	lastPrompt  string
	lastType    model.TaskType
}

func (m *MockAgentExecutor) Execute(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
	m.callCount++
	m.lastPrompt = prompt
	m.lastType = taskType
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, prompt, taskType)
	}
	return "mock response", nil
}

func TestNewEPICDecompositionStrategy(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewEPICDecompositionStrategy(mockExecutor)

	if strategy == nil {
		t.Fatal("Expected non-nil strategy")
	}

	if strategy.agentExecutor != mockExecutor {
		t.Error("Expected strategy to use provided agent executor")
	}
}

func TestEPICDecompositionStrategy_CanHandle(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewEPICDecompositionStrategy(mockExecutor)

	tests := []struct {
		name     string
		taskType model.TaskType
		expected bool
	}{
		{
			name:     "EPIC task type",
			taskType: model.TaskTypeEPIC,
			expected: true,
		},
		{
			name:     "PBI task type",
			taskType: model.TaskTypePBI,
			expected: false,
		},
		{
			name:     "SBI task type",
			taskType: model.TaskTypeSBI,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strategy.CanHandle(tt.taskType)
			if result != tt.expected {
				t.Errorf("Expected CanHandle(%v) to return %v, got %v", tt.taskType, tt.expected, result)
			}
		})
	}
}

func TestEPICDecompositionStrategy_GetName(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewEPICDecompositionStrategy(mockExecutor)

	expectedName := "EPICDecompositionStrategy"
	if strategy.GetName() != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, strategy.GetName())
	}
}

func TestEPICDecompositionStrategy_Execute_Success(t *testing.T) {
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			// Verify the prompt contains EPIC details
			if !strings.Contains(prompt, "Test EPIC") {
				t.Errorf("Prompt should contain EPIC title")
			}
			if !strings.Contains(prompt, "Test description") {
				t.Errorf("Prompt should contain EPIC description")
			}
			return "## PBI 1: Create User Interface\n**Description:** Build the UI\n**Story Points:** 5", nil
		},
	}

	strategy := NewEPICDecompositionStrategy(mockExecutor)

	// Create a test EPIC
	metadata := epic.EPICMetadata{
		EstimatedStoryPoints: 50,
		Priority:             1,
	}
	epicTask, err := epic.NewEPIC("Test EPIC", "Test description", metadata)
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	// Execute the strategy
	ctx := context.Background()
	result, err := strategy.Execute(ctx, epicTask)

	// Verify the result
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Message != "Successfully generated PBI proposals" {
		t.Errorf("Expected success message, got '%s'", result.Message)
	}

	if result.NextStep != model.StepReview {
		t.Errorf("Expected NextStep to be Review, got %v", result.NextStep)
	}

	// Verify artifacts
	if len(result.Artifacts) != 1 {
		t.Fatalf("Expected 1 artifact, got %d", len(result.Artifacts))
	}

	artifact := result.Artifacts[0]
	if artifact.Path != "pbi_proposals.md" {
		t.Errorf("Expected artifact path 'pbi_proposals.md', got '%s'", artifact.Path)
	}

	if artifact.Type != ArtifactTypeTask {
		t.Errorf("Expected artifact type Task, got %v", artifact.Type)
	}

	if !strings.Contains(artifact.Description, "Test EPIC") {
		t.Error("Artifact description should contain EPIC title")
	}

	// Verify metadata
	if result.Metadata == nil {
		t.Fatal("Expected non-nil metadata")
	}

	epicID, ok := result.Metadata["epic_id"].(string)
	if !ok || epicID == "" {
		t.Error("Metadata should contain epic_id")
	}

	decomposed, ok := result.Metadata["decomposed"].(bool)
	if !ok || !decomposed {
		t.Error("Metadata should indicate decomposition occurred")
	}

	// Verify executor was called correctly
	if mockExecutor.callCount != 1 {
		t.Errorf("Expected executor to be called once, got %d calls", mockExecutor.callCount)
	}

	if mockExecutor.lastType != model.TaskTypeEPIC {
		t.Errorf("Expected executor to be called with TaskTypeEPIC, got %v", mockExecutor.lastType)
	}
}

func TestEPICDecompositionStrategy_Execute_AgentError(t *testing.T) {
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			return "", errors.New("agent execution failed")
		},
	}

	strategy := NewEPICDecompositionStrategy(mockExecutor)

	metadata := epic.EPICMetadata{}
	epicTask, _ := epic.NewEPIC("Test EPIC", "Test description", metadata)

	ctx := context.Background()
	result, err := strategy.Execute(ctx, epicTask)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error from Execute")
	}

	if !strings.Contains(err.Error(), "agent execution failed") {
		t.Errorf("Expected error to contain agent error message, got '%v'", err)
	}

	// Result should still be returned but with Success=false
	if result == nil {
		t.Fatal("Expected non-nil result even on error")
	}

	if result.Success {
		t.Error("Expected Success to be false on error")
	}

	if !strings.Contains(result.Message, "Failed to decompose EPIC") {
		t.Errorf("Expected failure message, got '%s'", result.Message)
	}

	// Next step should be Implement (retry)
	if result.NextStep != model.StepImplement {
		t.Errorf("Expected NextStep to be Implement for retry, got %v", result.NextStep)
	}
}

func TestEPICDecompositionStrategy_Execute_InvalidTaskType(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewEPICDecompositionStrategy(mockExecutor)

	// Create a non-EPIC task (SBI)
	metadata := sbi.SBIMetadata{}
	sbiTask, _ := sbi.NewSBI("Test SBI", "Test description", nil, metadata)

	ctx := context.Background()
	result, err := strategy.Execute(ctx, sbiTask)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error when passing non-EPIC task")
	}

	if !strings.Contains(err.Error(), "not an EPIC") {
		t.Errorf("Expected error about wrong task type, got '%v'", err)
	}

	// Result should be nil
	if result != nil {
		t.Error("Expected nil result for invalid task type")
	}

	// Agent executor should not be called
	if mockExecutor.callCount != 0 {
		t.Error("Agent executor should not be called for invalid task type")
	}
}

func TestEPICDecompositionStrategy_Execute_ContextCancellation(t *testing.T) {
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			// Check if context is cancelled
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "response", nil
		},
	}

	strategy := NewEPICDecompositionStrategy(mockExecutor)

	metadata := epic.EPICMetadata{}
	epicTask, _ := epic.NewEPIC("Test EPIC", "Test description", metadata)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := strategy.Execute(ctx, epicTask)

	// Should handle context cancellation
	if err == nil {
		t.Fatal("Expected error due to context cancellation")
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Success {
		t.Error("Expected Success to be false on context cancellation")
	}
}

func TestEPICDecompositionStrategy_buildDecompositionPrompt(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewEPICDecompositionStrategy(mockExecutor)

	metadata := epic.EPICMetadata{
		EstimatedStoryPoints: 50,
	}
	epicTask, _ := epic.NewEPIC("User Authentication", "Implement complete user authentication system", metadata)

	prompt := strategy.buildDecompositionPrompt(epicTask)

	// Verify prompt contains required elements
	requiredElements := []string{
		"User Authentication",                           // Title
		"Implement complete user authentication system", // Description
		"Product Backlog Items (PBIs)",                  // Mention of PBIs
		"3-7 PBIs",                                      // Number range
		"Title",                                         // Required fields
		"Description",
		"Story Points",
		"Acceptance Criteria",
		"1, 2, 3, 5, 8, 13", // Story point options
	}

	for _, element := range requiredElements {
		if !strings.Contains(prompt, element) {
			t.Errorf("Prompt should contain '%s'", element)
		}
	}

	// Verify prompt structure
	if !strings.Contains(prompt, "EPIC Title:") {
		t.Error("Prompt should have EPIC Title label")
	}

	if !strings.Contains(prompt, "EPIC Description:") {
		t.Error("Prompt should have EPIC Description label")
	}

	// Verify markdown format instructions
	if !strings.Contains(prompt, "## PBI") {
		t.Error("Prompt should include markdown format example")
	}
}

func TestEPICDecompositionStrategy_Execute_EmptyEPICTitle(t *testing.T) {
	// Create EPIC with empty title (should fail during EPIC creation)
	metadata := epic.EPICMetadata{}
	_, err := epic.NewEPIC("", "Valid description", metadata)

	// Should fail at EPIC creation level
	if err == nil {
		t.Fatal("Expected error when creating EPIC with empty title")
	}
}

func TestEPICDecompositionStrategy_Execute_MultipleEPICs(t *testing.T) {
	callCount := 0
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			callCount++
			return "PBI proposals " + string(rune(callCount)), nil
		},
	}

	strategy := NewEPICDecompositionStrategy(mockExecutor)

	// Create multiple EPICs and execute strategy for each
	epics := []struct {
		title       string
		description string
	}{
		{"EPIC 1", "Description 1"},
		{"EPIC 2", "Description 2"},
		{"EPIC 3", "Description 3"},
	}

	ctx := context.Background()
	for i, e := range epics {
		metadata := epic.EPICMetadata{}
		epicTask, _ := epic.NewEPIC(e.title, e.description, metadata)

		result, err := strategy.Execute(ctx, epicTask)
		if err != nil {
			t.Fatalf("Execute failed for EPIC %d: %v", i+1, err)
		}

		if !result.Success {
			t.Errorf("Expected success for EPIC %d", i+1)
		}

		if len(result.Artifacts) == 0 {
			t.Errorf("Expected artifacts for EPIC %d", i+1)
		}
	}

	if callCount != 3 {
		t.Errorf("Expected executor to be called 3 times, got %d", callCount)
	}
}

func TestEPICDecompositionStrategy_Execute_ArtifactContent(t *testing.T) {
	expectedResponse := `## PBI 1: User Registration
**Description:** Implement user registration flow
**Story Points:** 5
**Acceptance Criteria:**
- Users can register with email
- Password validation is enforced
- Confirmation email is sent`

	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			return expectedResponse, nil
		},
	}

	strategy := NewEPICDecompositionStrategy(mockExecutor)

	metadata := epic.EPICMetadata{}
	epicTask, _ := epic.NewEPIC("Auth System", "Complete authentication", metadata)

	ctx := context.Background()
	result, err := strategy.Execute(ctx, epicTask)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if len(result.Artifacts) == 0 {
		t.Fatal("Expected at least one artifact")
	}

	artifact := result.Artifacts[0]
	if artifact.Content != expectedResponse {
		t.Errorf("Expected artifact content to match response.\nExpected: %s\nGot: %s", expectedResponse, artifact.Content)
	}
}

func TestEPICDecompositionStrategy_Execute_MetadataContent(t *testing.T) {
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			return "PBI proposals", nil
		},
	}

	strategy := NewEPICDecompositionStrategy(mockExecutor)

	metadata := epic.EPICMetadata{
		EstimatedStoryPoints: 100,
		Priority:             2,
		Labels:               []string{"critical", "v2"},
		AssignedAgent:        "claude-code",
	}
	epicTask, _ := epic.NewEPIC("Critical Feature", "High priority feature", metadata)

	ctx := context.Background()
	result, err := strategy.Execute(ctx, epicTask)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	// Verify metadata
	if result.Metadata == nil {
		t.Fatal("Expected non-nil metadata")
	}

	// Check epic_id is set
	epicID, ok := result.Metadata["epic_id"].(string)
	if !ok {
		t.Error("Metadata should contain epic_id as string")
	}
	if epicID == "" {
		t.Error("epic_id should not be empty")
	}

	// Check pbi_count is initialized
	pbiCount, ok := result.Metadata["pbi_count"].(int)
	if !ok {
		t.Error("Metadata should contain pbi_count as int")
	}
	if pbiCount != 0 {
		t.Errorf("Expected pbi_count to be 0 initially, got %d", pbiCount)
	}

	// Check decomposed flag
	decomposed, ok := result.Metadata["decomposed"].(bool)
	if !ok {
		t.Error("Metadata should contain decomposed as bool")
	}
	if !decomposed {
		t.Error("decomposed should be true")
	}
}
