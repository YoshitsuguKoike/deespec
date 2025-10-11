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

func TestNewSBICodeGenerationStrategy(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	if strategy == nil {
		t.Fatal("Expected non-nil strategy")
	}

	if strategy.agentExecutor != mockExecutor {
		t.Error("Expected strategy to use provided agent executor")
	}
}

func TestSBICodeGenerationStrategy_CanHandle(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	tests := []struct {
		name     string
		taskType model.TaskType
		expected bool
	}{
		{
			name:     "SBI task type",
			taskType: model.TaskTypeSBI,
			expected: true,
		},
		{
			name:     "EPIC task type",
			taskType: model.TaskTypeEPIC,
			expected: false,
		},
		{
			name:     "PBI task type",
			taskType: model.TaskTypePBI,
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

func TestSBICodeGenerationStrategy_GetName(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	expectedName := "SBICodeGenerationStrategy"
	if strategy.GetName() != expectedName {
		t.Errorf("Expected name '%s', got '%s'", expectedName, strategy.GetName())
	}
}

func TestSBICodeGenerationStrategy_Execute_Success(t *testing.T) {
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			// Verify the prompt contains SBI details
			if !strings.Contains(prompt, "Test SBI") {
				t.Errorf("Prompt should contain SBI title")
			}
			if !strings.Contains(prompt, "Test description") {
				t.Errorf("Prompt should contain SBI description")
			}
			return `## File: main.go
` + "```" + `go
package main

func main() {
    println("Hello, World!")
}
` + "```" + `

## Tests: main_test.go
` + "```" + `go
package main

import "testing"

func TestMain(t *testing.T) {
    // Test implementation
}
` + "```", nil
		},
	}

	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	// Create a test SBI
	metadata := sbi.SBIMetadata{
		EstimatedHours: 2.0,
		Priority:       1,
		FilePaths:      []string{"main.go"},
	}
	parentID := model.NewTaskID()
	sbiTask, err := sbi.NewSBI("Test SBI", "Test description", &parentID, metadata)
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	// Execute the strategy
	ctx := context.Background()
	result, err := strategy.Execute(ctx, sbiTask)

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

	if result.Message != "Successfully generated code" {
		t.Errorf("Expected success message, got '%s'", result.Message)
	}

	if result.NextStep != model.StepReview {
		t.Errorf("Expected NextStep to be Review, got %v", result.NextStep)
	}

	// Verify artifacts
	if len(result.Artifacts) != 2 {
		t.Fatalf("Expected 2 artifacts (code + test), got %d", len(result.Artifacts))
	}

	// Verify code artifact
	codeArtifact := result.Artifacts[0]
	if codeArtifact.Path != "main.go" {
		t.Errorf("Expected code artifact path 'main.go', got '%s'", codeArtifact.Path)
	}
	if codeArtifact.Type != ArtifactTypeCode {
		t.Errorf("Expected artifact type Code, got %v", codeArtifact.Type)
	}

	// Verify test artifact
	testArtifact := result.Artifacts[1]
	if testArtifact.Path != "main_test.go" {
		t.Errorf("Expected test artifact path 'main_test.go', got '%s'", testArtifact.Path)
	}
	if testArtifact.Type != ArtifactTypeTest {
		t.Errorf("Expected artifact type Test, got %v", testArtifact.Type)
	}

	// Verify metadata
	if result.Metadata == nil {
		t.Fatal("Expected non-nil metadata")
	}

	sbiID, ok := result.Metadata["sbi_id"].(string)
	if !ok || sbiID == "" {
		t.Error("Metadata should contain sbi_id")
	}

	codeGenerated, ok := result.Metadata["code_generated"].(bool)
	if !ok || !codeGenerated {
		t.Error("Metadata should indicate code was generated")
	}

	// Verify SBI state changes
	if sbiTask.ExecutionState().CurrentTurn.Value() != 1 {
		t.Errorf("Expected turn to be incremented to 1, got %d", sbiTask.ExecutionState().CurrentTurn.Value())
	}

	if sbiTask.ExecutionState().LastError != "" {
		t.Error("Expected last error to be cleared")
	}

	// Verify executor was called correctly
	if mockExecutor.callCount != 1 {
		t.Errorf("Expected executor to be called once, got %d calls", mockExecutor.callCount)
	}

	if mockExecutor.lastType != model.TaskTypeSBI {
		t.Errorf("Expected executor to be called with TaskTypeSBI, got %v", mockExecutor.lastType)
	}
}

func TestSBICodeGenerationStrategy_Execute_MaxTurnsExceeded(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	// Create SBI with max turns exceeded
	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Test description", &parentID, metadata)

	// Set max turns to 5 and current turn to 6 (exceeded)
	sbiTask.SetMaxTurns(5)
	for i := 0; i < 6; i++ {
		sbiTask.IncrementTurn()
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, sbiTask)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error when max turns exceeded")
	}

	if !strings.Contains(err.Error(), "maximum turns exceeded") {
		t.Errorf("Expected error about max turns, got '%v'", err)
	}

	// Result should still be returned but with Success=false
	if result == nil {
		t.Fatal("Expected non-nil result even on error")
	}

	if result.Success {
		t.Error("Expected Success to be false on max turns exceeded")
	}

	if result.NextStep != model.StepReview {
		t.Errorf("Expected NextStep to be Review for manual intervention, got %v", result.NextStep)
	}

	// Agent executor should not be called
	if mockExecutor.callCount != 0 {
		t.Error("Agent executor should not be called when max turns exceeded")
	}
}

func TestSBICodeGenerationStrategy_Execute_MaxAttemptsExceeded(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	// Create SBI with max attempts exceeded
	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Test description", &parentID, metadata)

	// Set max attempts to 3 and current attempt to 4 (exceeded)
	sbiTask.SetMaxAttempts(3)
	for i := 0; i < 4; i++ {
		sbiTask.IncrementAttempt()
	}

	ctx := context.Background()
	result, err := strategy.Execute(ctx, sbiTask)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error when max attempts exceeded")
	}

	if !strings.Contains(err.Error(), "maximum attempts exceeded") {
		t.Errorf("Expected error about max attempts, got '%v'", err)
	}

	// Result should still be returned but with Success=false
	if result == nil {
		t.Fatal("Expected non-nil result even on error")
	}

	if result.Success {
		t.Error("Expected Success to be false on max attempts exceeded")
	}

	if result.NextStep != model.StepReview {
		t.Errorf("Expected NextStep to be Review for manual intervention, got %v", result.NextStep)
	}

	// Agent executor should not be called
	if mockExecutor.callCount != 0 {
		t.Error("Agent executor should not be called when max attempts exceeded")
	}
}

func TestSBICodeGenerationStrategy_Execute_AgentError(t *testing.T) {
	expectedError := errors.New("agent execution failed")
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			return "", expectedError
		},
	}

	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Test description", &parentID, metadata)

	ctx := context.Background()
	result, err := strategy.Execute(ctx, sbiTask)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error from Execute")
	}

	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error to be the agent error, got '%v'", err)
	}

	// Result should still be returned but with Success=false
	if result == nil {
		t.Fatal("Expected non-nil result even on error")
	}

	if result.Success {
		t.Error("Expected Success to be false on error")
	}

	if !strings.Contains(result.Message, "Failed to generate code") {
		t.Errorf("Expected failure message, got '%s'", result.Message)
	}

	// Next step should be Implement (retry)
	if result.NextStep != model.StepImplement {
		t.Errorf("Expected NextStep to be Implement for retry, got %v", result.NextStep)
	}

	// Verify SBI state changes on error
	if sbiTask.ExecutionState().LastError == "" {
		t.Error("Expected error to be recorded in SBI")
	}

	if !strings.Contains(sbiTask.ExecutionState().LastError, "agent execution failed") {
		t.Errorf("Expected error message in SBI state, got '%s'", sbiTask.ExecutionState().LastError)
	}

	// NewAttempt() starts at 1, so after one IncrementAttempt() it should be 2
	if sbiTask.ExecutionState().CurrentAttempt.Value() != 2 {
		t.Errorf("Expected attempt to be incremented to 2, got %d", sbiTask.ExecutionState().CurrentAttempt.Value())
	}
}

func TestSBICodeGenerationStrategy_Execute_InvalidTaskType(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	// Create a non-SBI task (EPIC)
	metadata := epic.EPICMetadata{}
	epicTask, _ := epic.NewEPIC("Test EPIC", "Test description", metadata)

	ctx := context.Background()
	result, err := strategy.Execute(ctx, epicTask)

	// Should return an error
	if err == nil {
		t.Fatal("Expected error when passing non-SBI task")
	}

	if !strings.Contains(err.Error(), "not an SBI") {
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

func TestSBICodeGenerationStrategy_Execute_ContextCancellation(t *testing.T) {
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			// Check if context is cancelled
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			return "response", nil
		},
	}

	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Test description", &parentID, metadata)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result, err := strategy.Execute(ctx, sbiTask)

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

	// Error should be recorded
	if sbiTask.ExecutionState().LastError == "" {
		t.Error("Expected error to be recorded")
	}
}

func TestSBICodeGenerationStrategy_buildCodeGenerationPrompt(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{
		EstimatedHours: 3.5,
		FilePaths:      []string{"cmd/main.go", "pkg/service/user.go"},
	}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Implement user service", "Create user management service with CRUD operations", &parentID, metadata)

	// Record an error to test error handling in prompt
	sbiTask.RecordError("previous compilation error")

	prompt := strategy.buildCodeGenerationPrompt(sbiTask)

	// Verify prompt contains required elements
	requiredElements := []string{
		"Implement user service",                              // Title
		"Create user management service with CRUD operations", // Description
		"cmd/main.go", // File paths
		"pkg/service/user.go",
		"previous compilation error", // Previous error
		"Please provide:",            // Instructions
		"Complete, working code",
		"Comments explaining key logic",
		"Error handling",
		"Unit tests",
	}

	for _, element := range requiredElements {
		if !strings.Contains(prompt, element) {
			t.Errorf("Prompt should contain '%s'", element)
		}
	}

	// Verify prompt structure
	if !strings.Contains(prompt, "SBI Title:") {
		t.Error("Prompt should have SBI Title label")
	}

	if !strings.Contains(prompt, "SBI Description:") {
		t.Error("Prompt should have SBI Description label")
	}

	if !strings.Contains(prompt, "Target Files:") {
		t.Error("Prompt should have Target Files label")
	}

	if !strings.Contains(prompt, "Previous Error:") {
		t.Error("Prompt should have Previous Error section when error exists")
	}

	// Verify markdown format instructions
	if !strings.Contains(prompt, "## File:") {
		t.Error("Prompt should include markdown format example for files")
	}

	if !strings.Contains(prompt, "## Tests:") {
		t.Error("Prompt should include markdown format example for tests")
	}
}

func TestSBICodeGenerationStrategy_buildCodeGenerationPrompt_NoError(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{
		EstimatedHours: 1.0,
		FilePaths:      []string{"main.go"},
	}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Simple task", "Description", &parentID, metadata)

	// No error recorded
	prompt := strategy.buildCodeGenerationPrompt(sbiTask)

	// Should not contain error section
	if strings.Contains(prompt, "Previous Error:") {
		t.Error("Prompt should not have Previous Error section when no error exists")
	}
}

func TestSBICodeGenerationStrategy_parseCodeArtifacts(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Description", &parentID, metadata)

	response := `## File: cmd/main.go
` + "```" + `go
package main

func main() {
    println("Hello")
}
` + "```" + `

## File: pkg/util.go
` + "```" + `go
package pkg

func Helper() string {
    return "help"
}
` + "```" + `

## Tests: cmd/main_test.go
` + "```" + `go
package main

import "testing"

func TestMain(t *testing.T) {
    // test
}
` + "```"

	artifacts := strategy.parseCodeArtifacts(response, sbiTask)

	// Verify artifact count
	if len(artifacts) != 3 {
		t.Fatalf("Expected 3 artifacts, got %d", len(artifacts))
	}

	// Verify first artifact (main.go)
	if artifacts[0].Path != "cmd/main.go" {
		t.Errorf("Expected path 'cmd/main.go', got '%s'", artifacts[0].Path)
	}
	if artifacts[0].Type != ArtifactTypeCode {
		t.Errorf("Expected type Code, got %v", artifacts[0].Type)
	}
	if !strings.Contains(artifacts[0].Content, "package main") {
		t.Error("Artifact content should contain code")
	}

	// Verify second artifact (util.go)
	if artifacts[1].Path != "pkg/util.go" {
		t.Errorf("Expected path 'pkg/util.go', got '%s'", artifacts[1].Path)
	}
	if artifacts[1].Type != ArtifactTypeCode {
		t.Errorf("Expected type Code, got %v", artifacts[1].Type)
	}

	// Verify third artifact (test)
	if artifacts[2].Path != "cmd/main_test.go" {
		t.Errorf("Expected path 'cmd/main_test.go', got '%s'", artifacts[2].Path)
	}
	if artifacts[2].Type != ArtifactTypeTest {
		t.Errorf("Expected type Test, got %v", artifacts[2].Type)
	}
	if !strings.Contains(artifacts[2].Content, "TestMain") {
		t.Error("Test artifact content should contain test code")
	}

	// Verify descriptions
	for _, artifact := range artifacts {
		if !strings.Contains(artifact.Description, "Test SBI") {
			t.Errorf("Artifact description should contain SBI title, got '%s'", artifact.Description)
		}
	}
}

func TestSBICodeGenerationStrategy_parseCodeArtifacts_EmptyResponse(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Description", &parentID, metadata)

	response := ""
	artifacts := strategy.parseCodeArtifacts(response, sbiTask)

	// parseCodeArtifacts returns nil for empty response, which is valid Go behavior
	// An empty slice and nil slice both have length 0
	if len(artifacts) != 0 {
		t.Errorf("Expected 0 artifacts for empty response, got %d", len(artifacts))
	}
}

func TestSBICodeGenerationStrategy_parseCodeArtifacts_NoCodeBlocks(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Description", &parentID, metadata)

	response := `## File: main.go
This is just text without code blocks.

## Tests: main_test.go
Also no code blocks here.`

	artifacts := strategy.parseCodeArtifacts(response, sbiTask)

	// Should return empty slice when no code blocks found
	if len(artifacts) != 0 {
		t.Errorf("Expected 0 artifacts when no code blocks, got %d", len(artifacts))
	}
}

func TestSBICodeGenerationStrategy_parseCodeArtifacts_MixedContent(t *testing.T) {
	mockExecutor := &MockAgentExecutor{}
	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Test SBI", "Description", &parentID, metadata)

	response := `Some explanatory text here.

## File: config.yaml
` + "```" + `yaml
key: value
port: 8080
` + "```" + `

More text explaining the implementation.

## File: main.go
` + "```" + `go
package main
` + "```" + `

Additional notes about the code.`

	artifacts := strategy.parseCodeArtifacts(response, sbiTask)

	// Should extract only the code blocks
	if len(artifacts) != 2 {
		t.Fatalf("Expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify config artifact
	if artifacts[0].Path != "config.yaml" {
		t.Errorf("Expected path 'config.yaml', got '%s'", artifacts[0].Path)
	}
	if !strings.Contains(artifacts[0].Content, "key: value") {
		t.Error("Config artifact should contain yaml content")
	}
}

func TestSBICodeGenerationStrategy_Execute_WithPreviousError(t *testing.T) {
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			// Verify error appears in prompt
			if !strings.Contains(prompt, "compilation failed") {
				t.Error("Prompt should contain previous error")
			}
			return `## File: main.go
` + "```" + `go
package main
// Fixed version
` + "```", nil
		},
	}

	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Fix bug", "Fix the compilation error", &parentID, metadata)

	// Record previous error
	sbiTask.RecordError("compilation failed: undefined variable")
	sbiTask.IncrementAttempt()

	ctx := context.Background()
	result, err := strategy.Execute(ctx, sbiTask)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if !result.Success {
		t.Error("Expected success after fix")
	}

	// Verify error was cleared after success
	if sbiTask.ExecutionState().LastError != "" {
		t.Error("Expected error to be cleared after successful execution")
	}

	// Verify turn was incremented and attempt was reset
	if sbiTask.ExecutionState().CurrentTurn.Value() != 1 {
		t.Errorf("Expected turn to be 1, got %d", sbiTask.ExecutionState().CurrentTurn.Value())
	}

	// After IncrementTurn, attempt should be reset to NewAttempt() which starts at 1
	if sbiTask.ExecutionState().CurrentAttempt.Value() != 1 {
		t.Errorf("Expected attempt to be reset to 1 after turn increment, got %d", sbiTask.ExecutionState().CurrentAttempt.Value())
	}
}

func TestSBICodeGenerationStrategy_Execute_MultipleTurns(t *testing.T) {
	callCount := 0
	mockExecutor := &MockAgentExecutor{
		ExecuteFunc: func(ctx context.Context, prompt string, taskType model.TaskType) (string, error) {
			callCount++
			return `## File: version` + string(rune('0'+callCount)) + `.go
` + "```" + `go
package main
// Version ` + string(rune('0'+callCount)) + `
` + "```", nil
		},
	}

	strategy := NewSBICodeGenerationStrategy(mockExecutor)

	metadata := sbi.SBIMetadata{}
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("Iterative development", "Description", &parentID, metadata)

	ctx := context.Background()

	// Execute multiple turns
	for turn := 1; turn <= 3; turn++ {
		result, err := strategy.Execute(ctx, sbiTask)
		if err != nil {
			t.Fatalf("Execute failed on turn %d: %v", turn, err)
		}

		if !result.Success {
			t.Errorf("Expected success on turn %d", turn)
		}

		// Verify turn counter
		if sbiTask.ExecutionState().CurrentTurn.Value() != turn {
			t.Errorf("Expected turn %d, got %d", turn, sbiTask.ExecutionState().CurrentTurn.Value())
		}

		// Verify current turn in metadata
		currentTurn, ok := result.Metadata["current_turn"].(int)
		if !ok || currentTurn != turn {
			t.Errorf("Expected current_turn %d in metadata, got %v", turn, result.Metadata["current_turn"])
		}
	}

	if callCount != 3 {
		t.Errorf("Expected executor to be called 3 times, got %d", callCount)
	}
}
