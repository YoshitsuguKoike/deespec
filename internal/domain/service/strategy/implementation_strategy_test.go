package strategy

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/epic"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/task"
)

// MockStrategy is a mock implementation of ImplementationStrategy for testing
type MockStrategy struct {
	name        string
	canHandle   func(model.TaskType) bool
	executeFunc func(context.Context, task.Task) (*ImplementationResult, error)
}

func (m *MockStrategy) Execute(ctx context.Context, t task.Task) (*ImplementationResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, t)
	}
	return &ImplementationResult{Success: true, Message: "mock execution"}, nil
}

func (m *MockStrategy) CanHandle(taskType model.TaskType) bool {
	if m.canHandle != nil {
		return m.canHandle(taskType)
	}
	return false
}

func (m *MockStrategy) GetName() string {
	return m.name
}

// TestNewStrategyRegistry tests the creation of a new strategy registry
func TestNewStrategyRegistry(t *testing.T) {
	registry := NewStrategyRegistry()

	if registry == nil {
		t.Fatal("Expected non-nil registry")
	}

	if registry.strategies == nil {
		t.Fatal("Expected strategies map to be initialized")
	}

	if len(registry.strategies) != 0 {
		t.Errorf("Expected empty strategies map, got %d entries", len(registry.strategies))
	}
}

// TestStrategyRegistry_Register tests registering strategies
func TestStrategyRegistry_Register(t *testing.T) {
	registry := NewStrategyRegistry()

	// Create mock strategies for each task type
	epicStrategy := &MockStrategy{
		name:      "EPICStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeEPIC },
	}
	pbiStrategy := &MockStrategy{
		name:      "PBIStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypePBI },
	}
	sbiStrategy := &MockStrategy{
		name:      "SBIStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeSBI },
	}

	// Register strategies
	registry.Register(model.TaskTypeEPIC, epicStrategy)
	registry.Register(model.TaskTypePBI, pbiStrategy)
	registry.Register(model.TaskTypeSBI, sbiStrategy)

	// Verify all strategies are registered
	if len(registry.strategies) != 3 {
		t.Errorf("Expected 3 strategies, got %d", len(registry.strategies))
	}

	// Verify each strategy can be retrieved
	strategy, exists := registry.GetStrategy(model.TaskTypeEPIC)
	if !exists {
		t.Error("EPIC strategy should exist")
	}
	if strategy.GetName() != "EPICStrategy" {
		t.Errorf("Expected EPICStrategy, got %s", strategy.GetName())
	}

	strategy, exists = registry.GetStrategy(model.TaskTypePBI)
	if !exists {
		t.Error("PBI strategy should exist")
	}
	if strategy.GetName() != "PBIStrategy" {
		t.Errorf("Expected PBIStrategy, got %s", strategy.GetName())
	}

	strategy, exists = registry.GetStrategy(model.TaskTypeSBI)
	if !exists {
		t.Error("SBI strategy should exist")
	}
	if strategy.GetName() != "SBIStrategy" {
		t.Errorf("Expected SBIStrategy, got %s", strategy.GetName())
	}
}

// TestStrategyRegistry_Register_Overwrite tests overwriting a registered strategy
func TestStrategyRegistry_Register_Overwrite(t *testing.T) {
	registry := NewStrategyRegistry()

	// Register first strategy
	strategy1 := &MockStrategy{name: "Strategy1"}
	registry.Register(model.TaskTypeEPIC, strategy1)

	// Verify first strategy is registered
	retrieved, exists := registry.GetStrategy(model.TaskTypeEPIC)
	if !exists || retrieved.GetName() != "Strategy1" {
		t.Fatal("First strategy should be registered")
	}

	// Overwrite with second strategy
	strategy2 := &MockStrategy{name: "Strategy2"}
	registry.Register(model.TaskTypeEPIC, strategy2)

	// Verify second strategy replaced first
	retrieved, exists = registry.GetStrategy(model.TaskTypeEPIC)
	if !exists {
		t.Fatal("Strategy should exist after overwrite")
	}
	if retrieved.GetName() != "Strategy2" {
		t.Errorf("Expected Strategy2 after overwrite, got %s", retrieved.GetName())
	}
}

// TestStrategyRegistry_GetStrategy tests retrieving strategies
func TestStrategyRegistry_GetStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	strategy := &MockStrategy{name: "TestStrategy"}
	registry.Register(model.TaskTypeEPIC, strategy)

	tests := []struct {
		name       string
		taskType   model.TaskType
		shouldFind bool
	}{
		{
			name:       "Existing strategy",
			taskType:   model.TaskTypeEPIC,
			shouldFind: true,
		},
		{
			name:       "Non-existing strategy",
			taskType:   model.TaskTypePBI,
			shouldFind: false,
		},
		{
			name:       "Another non-existing strategy",
			taskType:   model.TaskTypeSBI,
			shouldFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, exists := registry.GetStrategy(tt.taskType)
			if exists != tt.shouldFind {
				t.Errorf("Expected exists=%v, got %v", tt.shouldFind, exists)
			}

			if tt.shouldFind && strategy == nil {
				t.Error("Expected non-nil strategy when exists=true")
			}

			if !tt.shouldFind && strategy != nil {
				t.Error("Expected nil strategy when exists=false")
			}
		})
	}
}

// TestStrategyRegistry_ExecuteImplementation tests executing implementation via registry
func TestStrategyRegistry_ExecuteImplementation(t *testing.T) {
	registry := NewStrategyRegistry()
	ctx := context.Background()

	// Create a mock strategy with custom execution
	expectedResult := &ImplementationResult{
		Success:  true,
		Message:  "Implementation successful",
		NextStep: model.StepReview,
	}

	mockStrategy := &MockStrategy{
		name:      "TestStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeEPIC },
		executeFunc: func(ctx context.Context, t task.Task) (*ImplementationResult, error) {
			return expectedResult, nil
		},
	}

	registry.Register(model.TaskTypeEPIC, mockStrategy)

	// Create test task
	metadata := epic.EPICMetadata{}
	epicTask, err := epic.NewEPIC("Test EPIC", "Test description", metadata)
	if err != nil {
		t.Fatalf("Failed to create EPIC: %v", err)
	}

	// Execute implementation
	result, err := registry.ExecuteImplementation(ctx, epicTask)

	// Verify result
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result != expectedResult {
		t.Error("Expected result to match expected result")
	}

	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Message != "Implementation successful" {
		t.Errorf("Expected message 'Implementation successful', got '%s'", result.Message)
	}

	if result.NextStep != model.StepReview {
		t.Errorf("Expected NextStep to be Review, got %v", result.NextStep)
	}
}

// TestStrategyRegistry_ExecuteImplementation_NoStrategy tests execution when no strategy is found
func TestStrategyRegistry_ExecuteImplementation_NoStrategy(t *testing.T) {
	registry := NewStrategyRegistry()
	ctx := context.Background()

	// Create task but don't register a strategy
	metadata := sbi.SBIMetadata{}
	sbiTask, err := sbi.NewSBI("Test SBI", "Test description", nil, metadata)
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	// Execute implementation
	result, err := registry.ExecuteImplementation(ctx, sbiTask)

	// Should return error
	if err == nil {
		t.Fatal("Expected error when no strategy is found")
	}

	// Should be StrategyNotFoundError
	var strategyErr *StrategyNotFoundError
	if !errors.As(err, &strategyErr) {
		t.Errorf("Expected StrategyNotFoundError, got %T", err)
	}

	if strategyErr.TaskType != model.TaskTypeSBI {
		t.Errorf("Expected error for TaskTypeSBI, got %v", strategyErr.TaskType)
	}

	// Result should be nil
	if result != nil {
		t.Error("Expected nil result when strategy not found")
	}
}

// TestStrategyRegistry_ExecuteImplementation_StrategyError tests execution when strategy returns error
func TestStrategyRegistry_ExecuteImplementation_StrategyError(t *testing.T) {
	registry := NewStrategyRegistry()
	ctx := context.Background()

	// Create a mock strategy that returns an error
	expectedError := errors.New("strategy execution failed")
	mockStrategy := &MockStrategy{
		name:      "ErrorStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeSBI },
		executeFunc: func(ctx context.Context, t task.Task) (*ImplementationResult, error) {
			return &ImplementationResult{
				Success: false,
				Message: "Execution failed",
			}, expectedError
		},
	}

	registry.Register(model.TaskTypeSBI, mockStrategy)

	// Create test task
	parentID := model.NewTaskID()
	metadata := sbi.SBIMetadata{}
	sbiTask, err := sbi.NewSBI("Test SBI", "Test description", &parentID, metadata)
	if err != nil {
		t.Fatalf("Failed to create SBI: %v", err)
	}

	// Execute implementation
	result, err := registry.ExecuteImplementation(ctx, sbiTask)

	// Should return error from strategy
	if err == nil {
		t.Fatal("Expected error from strategy execution")
	}

	if err != expectedError {
		t.Errorf("Expected specific error, got %v", err)
	}

	// Result should still be returned
	if result == nil {
		t.Fatal("Expected non-nil result even with error")
	}

	if result.Success {
		t.Error("Expected Success to be false")
	}

	if result.Message != "Execution failed" {
		t.Errorf("Expected message 'Execution failed', got '%s'", result.Message)
	}
}

// TestStrategyRegistry_MultipleExecutions tests multiple sequential executions
func TestStrategyRegistry_MultipleExecutions(t *testing.T) {
	registry := NewStrategyRegistry()
	ctx := context.Background()

	// Create strategies for different task types
	callCounts := make(map[model.TaskType]int)

	epicStrategy := &MockStrategy{
		name:      "EPICStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeEPIC },
		executeFunc: func(ctx context.Context, t task.Task) (*ImplementationResult, error) {
			callCounts[model.TaskTypeEPIC]++
			return &ImplementationResult{Success: true}, nil
		},
	}

	sbiStrategy := &MockStrategy{
		name:      "SBIStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeSBI },
		executeFunc: func(ctx context.Context, t task.Task) (*ImplementationResult, error) {
			callCounts[model.TaskTypeSBI]++
			return &ImplementationResult{Success: true}, nil
		},
	}

	registry.Register(model.TaskTypeEPIC, epicStrategy)
	registry.Register(model.TaskTypeSBI, sbiStrategy)

	// Execute multiple times for each task type
	metadata := epic.EPICMetadata{}
	epicTask, _ := epic.NewEPIC("Test EPIC", "Description", metadata)
	for i := 0; i < 3; i++ {
		_, err := registry.ExecuteImplementation(ctx, epicTask)
		if err != nil {
			t.Fatalf("EPIC execution %d failed: %v", i+1, err)
		}
	}

	parentID := model.NewTaskID()
	sbiMeta := sbi.SBIMetadata{}
	sbiTask, _ := sbi.NewSBI("Test SBI", "Description", &parentID, sbiMeta)
	for i := 0; i < 5; i++ {
		_, err := registry.ExecuteImplementation(ctx, sbiTask)
		if err != nil {
			t.Fatalf("SBI execution %d failed: %v", i+1, err)
		}
	}

	// Verify call counts
	if callCounts[model.TaskTypeEPIC] != 3 {
		t.Errorf("Expected 3 EPIC executions, got %d", callCounts[model.TaskTypeEPIC])
	}
	if callCounts[model.TaskTypeSBI] != 5 {
		t.Errorf("Expected 5 SBI executions, got %d", callCounts[model.TaskTypeSBI])
	}
}

// TestStrategyNotFoundError tests the StrategyNotFoundError type
func TestStrategyNotFoundError(t *testing.T) {
	tests := []struct {
		name        string
		taskType    model.TaskType
		expectedMsg string
	}{
		{
			name:        "EPIC task type",
			taskType:    model.TaskTypeEPIC,
			expectedMsg: "no implementation strategy found for task type: EPIC",
		},
		{
			name:        "PBI task type",
			taskType:    model.TaskTypePBI,
			expectedMsg: "no implementation strategy found for task type: PBI",
		},
		{
			name:        "SBI task type",
			taskType:    model.TaskTypeSBI,
			expectedMsg: "no implementation strategy found for task type: SBI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &StrategyNotFoundError{TaskType: tt.taskType}

			if err.Error() != tt.expectedMsg {
				t.Errorf("Expected error message '%s', got '%s'", tt.expectedMsg, err.Error())
			}

			// Verify it implements error interface
			var _ error = err
		})
	}
}

// TestImplementationResult_Structure tests the ImplementationResult structure
func TestImplementationResult_Structure(t *testing.T) {
	artifacts := []Artifact{
		{
			Path:        "test.go",
			Content:     "package test",
			Type:        ArtifactTypeCode,
			Description: "Test file",
		},
		{
			Path:        "README.md",
			Content:     "# Test",
			Type:        ArtifactTypeDocumentation,
			Description: "Documentation",
		},
	}

	metadata := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	childIDs := []model.TaskID{
		model.NewTaskID(),
		model.NewTaskID(),
	}

	result := &ImplementationResult{
		Success:      true,
		Message:      "Test message",
		Artifacts:    artifacts,
		NextStep:     model.StepReview,
		ChildTaskIDs: childIDs,
		Metadata:     metadata,
	}

	// Verify all fields are accessible
	if !result.Success {
		t.Error("Expected Success to be true")
	}

	if result.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", result.Message)
	}

	if len(result.Artifacts) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(result.Artifacts))
	}

	if result.NextStep != model.StepReview {
		t.Errorf("Expected NextStep to be Review, got %v", result.NextStep)
	}

	if len(result.ChildTaskIDs) != 2 {
		t.Errorf("Expected 2 child task IDs, got %d", len(result.ChildTaskIDs))
	}

	if len(result.Metadata) != 3 {
		t.Errorf("Expected 3 metadata entries, got %d", len(result.Metadata))
	}
}

// TestArtifact_Types tests the Artifact structure and types
func TestArtifact_Types(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		content      string
		artifactType ArtifactType
		description  string
	}{
		{
			name:         "Code artifact",
			path:         "main.go",
			content:      "package main",
			artifactType: ArtifactTypeCode,
			description:  "Main file",
		},
		{
			name:         "Documentation artifact",
			path:         "docs/api.md",
			content:      "# API Documentation",
			artifactType: ArtifactTypeDocumentation,
			description:  "API docs",
		},
		{
			name:         "Test artifact",
			path:         "main_test.go",
			content:      "package main\nimport \"testing\"",
			artifactType: ArtifactTypeTest,
			description:  "Test file",
		},
		{
			name:         "Config artifact",
			path:         "config.yaml",
			content:      "key: value",
			artifactType: ArtifactTypeConfig,
			description:  "Configuration",
		},
		{
			name:         "Task artifact",
			path:         "tasks/task1.md",
			content:      "# Task 1",
			artifactType: ArtifactTypeTask,
			description:  "Task definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			artifact := Artifact{
				Path:        tt.path,
				Content:     tt.content,
				Type:        tt.artifactType,
				Description: tt.description,
			}

			if artifact.Path != tt.path {
				t.Errorf("Expected path '%s', got '%s'", tt.path, artifact.Path)
			}

			if artifact.Content != tt.content {
				t.Errorf("Expected content '%s', got '%s'", tt.content, artifact.Content)
			}

			if artifact.Type != tt.artifactType {
				t.Errorf("Expected type '%s', got '%s'", tt.artifactType, artifact.Type)
			}

			if artifact.Description != tt.description {
				t.Errorf("Expected description '%s', got '%s'", tt.description, artifact.Description)
			}
		})
	}
}

// TestArtifactType_Constants tests that artifact type constants are correctly defined
func TestArtifactType_Constants(t *testing.T) {
	if ArtifactTypeCode != "CODE" {
		t.Errorf("Expected ArtifactTypeCode to be 'CODE', got '%s'", ArtifactTypeCode)
	}

	if ArtifactTypeDocumentation != "DOCUMENTATION" {
		t.Errorf("Expected ArtifactTypeDocumentation to be 'DOCUMENTATION', got '%s'", ArtifactTypeDocumentation)
	}

	if ArtifactTypeTest != "TEST" {
		t.Errorf("Expected ArtifactTypeTest to be 'TEST', got '%s'", ArtifactTypeTest)
	}

	if ArtifactTypeConfig != "CONFIG" {
		t.Errorf("Expected ArtifactTypeConfig to be 'CONFIG', got '%s'", ArtifactTypeConfig)
	}

	if ArtifactTypeTask != "TASK" {
		t.Errorf("Expected ArtifactTypeTask to be 'TASK', got '%s'", ArtifactTypeTask)
	}
}

// TestStrategyRegistry_ConcurrentAccess tests concurrent access to registry
func TestStrategyRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewStrategyRegistry()
	ctx := context.Background()

	// Register strategies
	for _, taskType := range []model.TaskType{model.TaskTypeEPIC, model.TaskTypeSBI} {
		tt := taskType // capture loop variable
		strategy := &MockStrategy{
			name:      string(taskType) + "Strategy",
			canHandle: func(t model.TaskType) bool { return t == tt },
			executeFunc: func(ctx context.Context, t task.Task) (*ImplementationResult, error) {
				return &ImplementationResult{Success: true}, nil
			},
		}
		registry.Register(taskType, strategy)
	}

	// Create test tasks
	epicTask, _ := epic.NewEPIC("EPIC", "Description", epic.EPICMetadata{})
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("SBI", "Description", &parentID, sbi.SBIMetadata{})

	// Execute concurrently (note: this is a basic test, not using goroutines)
	tasks := []task.Task{epicTask, sbiTask}
	for _, tsk := range tasks {
		_, err := registry.ExecuteImplementation(ctx, tsk)
		if err != nil {
			t.Errorf("Execution failed: %v", err)
		}
	}

	// Verify all strategies are still registered
	if len(registry.strategies) != 2 {
		t.Errorf("Expected 2 strategies, got %d", len(registry.strategies))
	}
}

// TestStrategyRegistry_TrueConcurrentAccess tests true concurrent access with goroutines
func TestStrategyRegistry_TrueConcurrentAccess(t *testing.T) {
	registry := NewStrategyRegistry()
	ctx := context.Background()

	// Track execution counts with atomic counters
	var epicCount, sbiCount atomic.Int32

	// Register strategies with execution counting
	epicStrategy := &MockStrategy{
		name:      "EPICStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeEPIC },
		executeFunc: func(ctx context.Context, t task.Task) (*ImplementationResult, error) {
			epicCount.Add(1)
			return &ImplementationResult{Success: true}, nil
		},
	}

	sbiStrategy := &MockStrategy{
		name:      "SBIStrategy",
		canHandle: func(tt model.TaskType) bool { return tt == model.TaskTypeSBI },
		executeFunc: func(ctx context.Context, t task.Task) (*ImplementationResult, error) {
			sbiCount.Add(1)
			return &ImplementationResult{Success: true}, nil
		},
	}

	registry.Register(model.TaskTypeEPIC, epicStrategy)
	registry.Register(model.TaskTypeSBI, sbiStrategy)

	// Create test tasks
	epicTask, _ := epic.NewEPIC("EPIC", "Description", epic.EPICMetadata{})
	parentID := model.NewTaskID()
	sbiTask, _ := sbi.NewSBI("SBI", "Description", &parentID, sbi.SBIMetadata{})

	// Execute concurrently with multiple goroutines
	const numGoroutines = 50
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*2)

	// Launch goroutines for EPIC tasks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := registry.ExecuteImplementation(ctx, epicTask)
			if err != nil {
				errors <- err
			}
		}()
	}

	// Launch goroutines for SBI tasks
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := registry.ExecuteImplementation(ctx, sbiTask)
			if err != nil {
				errors <- err
			}
		}()
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent execution failed: %v", err)
	}

	// Verify execution counts
	if epicCount.Load() != numGoroutines {
		t.Errorf("Expected %d EPIC executions, got %d", numGoroutines, epicCount.Load())
	}

	if sbiCount.Load() != numGoroutines {
		t.Errorf("Expected %d SBI executions, got %d", numGoroutines, sbiCount.Load())
	}

	// Verify registry integrity
	if len(registry.strategies) != 2 {
		t.Errorf("Expected 2 strategies after concurrent access, got %d", len(registry.strategies))
	}

	// Verify strategies are still retrievable
	strategy, exists := registry.GetStrategy(model.TaskTypeEPIC)
	if !exists || strategy == nil {
		t.Error("EPIC strategy should still be registered after concurrent access")
	}

	strategy, exists = registry.GetStrategy(model.TaskTypeSBI)
	if !exists || strategy == nil {
		t.Error("SBI strategy should still be registered after concurrent access")
	}
}
