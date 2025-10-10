package workflow_sbi

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/application/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/sbi"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
)

// createTestContainer creates a test container with in-memory database
func createTestContainer(t *testing.T) *di.Container {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	config := di.Config{
		DBPath:                dbPath,
		AgentType:             "gemini-cli", // Use mock agent for tests
		StorageType:           "mock",
		LockHeartbeatInterval: 100 * time.Millisecond,
		LockCleanupInterval:   200 * time.Millisecond,
	}

	container, err := di.NewContainer(config)
	require.NoError(t, err)

	// Start container services
	ctx := context.Background()
	err = container.Start(ctx)
	require.NoError(t, err)

	return container
}

// createTestSBI creates a test SBI entity
func createTestSBI(id string, status model.Status) *sbi.SBI {
	title := fmt.Sprintf("Test SBI %s", id)
	description := fmt.Sprintf("Description for %s", id)

	metadata := sbi.SBIMetadata{
		EstimatedHours: 1.0,
		Priority:       0,
		Sequence:       1,
		RegisteredAt:   time.Now(),
		Labels:         []string{},
		AssignedAgent:  "gemini-cli",
		FilePaths:      []string{},
	}

	s, _ := sbi.NewSBI(title, description, nil, metadata)

	// Set status after creation using reconstruction
	taskID, _ := model.NewTaskIDFromString(id)
	s = sbi.ReconstructSBI(
		taskID,
		title,
		description,
		status,
		model.StepPick, // Default step
		nil,
		metadata,
		s.ExecutionState(),
		time.Now(),
		time.Now(),
	)

	return s
}

// Test 1: 並行数制御テスト
func TestParallelSBIWorkflowRunner_ConcurrencyControl(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup container
	container := createTestContainer(t)
	defer container.Close()

	// Create and save 5 test SBIs
	ctx := context.Background()
	sbiRepo := container.GetSBIRepository()

	for i := 1; i <= 5; i++ {
		s := createTestSBI(fmt.Sprintf("SBI-%03d", i), model.StatusPending)
		err := sbiRepo.Save(ctx, s)
		require.NoError(t, err)
	}

	// Track concurrent executions
	var currentConcurrency int32
	var maxConcurrency int32
	var executionMutex sync.Mutex
	var executedSBIs []string

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		// Increment current concurrency
		current := atomic.AddInt32(&currentConcurrency, 1)
		defer atomic.AddInt32(&currentConcurrency, -1)

		// Update max concurrency
		for {
			max := atomic.LoadInt32(&maxConcurrency)
			if current <= max || atomic.CompareAndSwapInt32(&maxConcurrency, max, current) {
				break
			}
		}

		// Track executed SBIs
		executionMutex.Lock()
		executedSBIs = append(executedSBIs, sbiID)
		executionMutex.Unlock()

		// Simulate work
		time.Sleep(50 * time.Millisecond)
		return nil
	}

	// Create runner with max concurrency = 3
	maxParallel := 3
	runner := NewParallelSBIWorkflowRunner(container, maxParallel, executeTurn)

	// Run the workflow
	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)
	require.NoError(t, err)

	// Verify max concurrency was limited to 3
	assert.LessOrEqual(t, int(atomic.LoadInt32(&maxConcurrency)), maxParallel,
		"Max concurrency should not exceed %d", maxParallel)

	// Verify SBIs were executed (up to maxParallel in single run)
	// Note: Run() fetches up to maxParallel SBIs per invocation
	// WorkflowManager calls Run() periodically to process all tasks
	executionMutex.Lock()
	assert.Len(t, executedSBIs, maxParallel, "%d SBIs should have been executed in single Run()", maxParallel)
	executionMutex.Unlock()
}

// Test 2: ロック競合テスト
func TestParallelSBIWorkflowRunner_LockContention(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup container
	container := createTestContainer(t)
	defer container.Close()

	// Create and save 3 test SBIs
	ctx := context.Background()
	sbiRepo := container.GetSBIRepository()

	for i := 1; i <= 3; i++ {
		s := createTestSBI(fmt.Sprintf("SBI-%03d", i), model.StatusPending)
		err := sbiRepo.Save(ctx, s)
		require.NoError(t, err)
	}

	// Track execution count per SBI
	var executionMutex sync.Mutex
	executionCounts := make(map[string]int)

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		// Track execution
		executionMutex.Lock()
		executionCounts[sbiID]++
		executionMutex.Unlock()

		// Simulate work
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Create runner
	runner := NewParallelSBIWorkflowRunner(container, 3, executeTurn)

	// Run the workflow
	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)
	require.NoError(t, err)

	// Verify each SBI was executed exactly once
	executionMutex.Lock()
	for sbiID, count := range executionCounts {
		assert.Equal(t, 1, count, "SBI %s should have been executed exactly once, got %d", sbiID, count)
	}
	executionMutex.Unlock()
}

// Test 3: エラーハンドリングテスト
func TestParallelSBIWorkflowRunner_ErrorHandling(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup container
	container := createTestContainer(t)
	defer container.Close()

	// Create and save 5 test SBIs
	ctx := context.Background()
	sbiRepo := container.GetSBIRepository()

	for i := 1; i <= 5; i++ {
		s := createTestSBI(fmt.Sprintf("SBI-%03d", i), model.StatusPending)
		err := sbiRepo.Save(ctx, s)
		require.NoError(t, err)
	}

	// executeTurn that fails for specific SBIs
	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		// Fail for SBI-002 and SBI-004
		if sbiID == "SBI-002" || sbiID == "SBI-004" {
			return fmt.Errorf("intentional error for %s", sbiID)
		}
		return nil
	}

	// Create runner
	runner := NewParallelSBIWorkflowRunner(container, 3, executeTurn)

	// Run the workflow
	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)

	// Verify error was returned
	require.Error(t, err, "Should return error when executions fail")
	assert.Contains(t, err.Error(), "parallel execution errors",
		"Error should mention parallel execution errors")
}

// Test 4: コンテキストキャンセルテスト
func TestParallelSBIWorkflowRunner_ContextCancellation(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup container
	container := createTestContainer(t)
	defer container.Close()

	// Create context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// executeTurn function (won't be called due to cancelled context)
	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	// Create runner
	runner := NewParallelSBIWorkflowRunner(container, 3, executeTurn)

	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)

	// Verify context cancellation was handled
	assert.Equal(t, context.Canceled, err, "Should return context.Canceled error")
}

// Test 5: バリデーションテスト
func TestParallelSBIWorkflowRunner_Validate(t *testing.T) {
	defer goleak.VerifyNone(t)

	container := createTestContainer(t)
	defer container.Close()

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		return nil
	}

	tests := []struct {
		name        string
		maxParallel int
		container   *di.Container
		executeTurn ExecuteTurnFunc
		wantErr     bool
		errContains string
	}{
		{
			name:        "Valid configuration",
			maxParallel: 3,
			container:   container,
			executeTurn: executeTurn,
			wantErr:     false,
		},
		// Note: maxParallel=0 is automatically corrected to 1 in NewParallelSBIWorkflowRunner
		// so it won't fail validation. Test with negative value instead.
		{
			name:        "Invalid maxParallel (-1)",
			maxParallel: -1,
			container:   container,
			executeTurn: executeTurn,
			wantErr:     false, // Corrected to 1 in constructor
		},
		{
			name:        "Nil container",
			maxParallel: 3,
			container:   nil,
			executeTurn: executeTurn,
			wantErr:     true,
			errContains: "container is nil",
		},
		{
			name:        "Nil executeTurn function",
			maxParallel: 3,
			container:   container,
			executeTurn: nil,
			wantErr:     true,
			errContains: "executeTurn function is nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewParallelSBIWorkflowRunner(tt.container, tt.maxParallel, tt.executeTurn)
			err := runner.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Test 6: Name, Description, IsEnabled tests
func TestParallelSBIWorkflowRunner_BasicMethods(t *testing.T) {
	defer goleak.VerifyNone(t)

	container := createTestContainer(t)
	defer container.Close()

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		return nil
	}

	runner := NewParallelSBIWorkflowRunner(container, 3, executeTurn)

	// Test Name
	assert.Equal(t, "sbi-parallel", runner.Name())

	// Test Description
	assert.Contains(t, runner.Description(), "Parallel SBI workflow")
	assert.Contains(t, runner.Description(), "max: 3")

	// Test IsEnabled / SetEnabled
	assert.True(t, runner.IsEnabled(), "Runner should be enabled by default")

	runner.SetEnabled(false)
	assert.False(t, runner.IsEnabled(), "Runner should be disabled after SetEnabled(false)")

	runner.SetEnabled(true)
	assert.True(t, runner.IsEnabled(), "Runner should be enabled after SetEnabled(true)")
}

// Test 7: No tasks available
func TestParallelSBIWorkflowRunner_NoTasksAvailable(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup container with empty repository
	container := createTestContainer(t)
	defer container.Close()

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		t.Fatal("executeTurn should not be called when no tasks are available")
		return nil
	}

	// Create runner
	runner := NewParallelSBIWorkflowRunner(container, 3, executeTurn)

	// Run the workflow
	ctx := context.Background()
	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)

	// Verify no error when no tasks available
	require.NoError(t, err, "Should not error when no tasks are available")
}

// Test 8: Large number of SBIs (load test)
func TestParallelSBIWorkflowRunner_LoadTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
	}

	defer goleak.VerifyNone(t)

	// Setup container
	container := createTestContainer(t)
	defer container.Close()

	// Create and save 20 test SBIs
	ctx := context.Background()
	sbiRepo := container.GetSBIRepository()

	for i := 1; i <= 20; i++ {
		s := createTestSBI(fmt.Sprintf("SBI-%03d", i), model.StatusPending)
		err := sbiRepo.Save(ctx, s)
		require.NoError(t, err)
	}

	// Track execution count
	var executionCount int32

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		atomic.AddInt32(&executionCount, 1)
		time.Sleep(10 * time.Millisecond) // Simulate work
		return nil
	}

	// Create runner with higher concurrency
	runner := NewParallelSBIWorkflowRunner(container, 5, executeTurn)

	// Measure execution time
	startTime := time.Now()

	// Run the workflow
	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)
	require.NoError(t, err)

	elapsedTime := time.Since(startTime)

	// Verify SBIs were executed (up to maxParallel=5 in single run)
	// Note: Run() processes up to maxParallel tasks per invocation
	// To process all 20 SBIs, would need multiple Run() calls
	assert.Equal(t, int32(5), atomic.LoadInt32(&executionCount),
		"5 SBIs should have been executed in single Run() with maxParallel=5")

	// Verify parallel execution was faster than sequential for the 5 SBIs
	// Sequential: 5 * 10ms = 50ms
	// Parallel (5 concurrent): ~1 batch * 10ms = ~10ms
	// Allow overhead
	assert.Less(t, elapsedTime, 100*time.Millisecond,
		"Parallel execution should be faster than sequential")

	t.Logf("Load test completed: %d SBIs in %v (avg: %v per SBI)",
		atomic.LoadInt32(&executionCount), elapsedTime, elapsedTime/time.Duration(atomic.LoadInt32(&executionCount)))
}

// Test 9: File conflict detection test
func TestParallelSBIWorkflowRunner_FileConflictDetection(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup container
	container := createTestContainer(t)
	defer container.Close()

	ctx := context.Background()
	sbiRepo := container.GetSBIRepository()

	// Create SBIs with overlapping file paths
	// SBI-001: file1.go, file2.go
	// SBI-002: file3.go, file4.go (no conflict with SBI-001)
	// SBI-003: file2.go, file5.go (conflicts with SBI-001 on file2.go)
	// SBI-004: file6.go (no conflict)

	sbi1 := createTestSBIWithFiles("SBI-001", []string{"file1.go", "file2.go"})
	sbi2 := createTestSBIWithFiles("SBI-002", []string{"file3.go", "file4.go"})
	sbi3 := createTestSBIWithFiles("SBI-003", []string{"file2.go", "file5.go"})
	sbi4 := createTestSBIWithFiles("SBI-004", []string{"file6.go"})

	// Save all SBIs
	require.NoError(t, sbiRepo.Save(ctx, sbi1))
	require.NoError(t, sbiRepo.Save(ctx, sbi2))
	require.NoError(t, sbiRepo.Save(ctx, sbi3))
	require.NoError(t, sbiRepo.Save(ctx, sbi4))

	// Track concurrent execution of conflicting files
	var mu sync.Mutex
	activeFiles := make(map[string]bool) // Track which files are currently being processed
	maxConcurrentConflicts := 0          // Should stay 0 (no conflicts should execute concurrently)
	executionOrder := []string{}

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		// Get SBI to check its files
		s, err := sbiRepo.Find(ctx, repository.SBIID(sbiID))
		if err != nil {
			return err
		}

		mu.Lock()
		// Check if any of this SBI's files are currently active
		conflictCount := 0
		for _, filePath := range s.Metadata().FilePaths {
			if activeFiles[filePath] {
				conflictCount++
			}
		}
		if conflictCount > maxConcurrentConflicts {
			maxConcurrentConflicts = conflictCount
		}

		// Mark files as active
		for _, filePath := range s.Metadata().FilePaths {
			activeFiles[filePath] = true
		}
		executionOrder = append(executionOrder, sbiID)
		mu.Unlock()

		// Simulate work
		time.Sleep(100 * time.Millisecond)

		// Unmark files
		mu.Lock()
		for _, filePath := range s.Metadata().FilePaths {
			delete(activeFiles, filePath)
		}
		mu.Unlock()

		return nil
	}

	// Create runner with max concurrency = 4 (enough to run all non-conflicting SBIs)
	runner := NewParallelSBIWorkflowRunner(container, 4, executeTurn)

	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)
	require.NoError(t, err)

	// Verify no concurrent conflicts occurred
	assert.Equal(t, 0, maxConcurrentConflicts,
		"No files should be processed concurrently by different SBIs")

	// Verify that non-conflicting SBIs executed
	// In one Run() call with maxParallel=4, we can execute up to 4 SBIs
	// But SBI-003 conflicts with SBI-001 (both use file2.go), so it should be skipped
	// So we expect 3 SBIs to execute: SBI-001, SBI-002, SBI-004
	mu.Lock()
	assert.Len(t, executionOrder, 3, "3 non-conflicting SBIs should execute")
	// SBI-003 should not be in the execution order if it was skipped due to conflict
	assert.NotContains(t, executionOrder, "SBI-003", "SBI-003 should be skipped due to file conflict with SBI-001")
	mu.Unlock()

	t.Logf("Execution order: %v", executionOrder)
}

// createTestSBIWithFiles creates a test SBI with specific file paths
func createTestSBIWithFiles(id string, filePaths []string) *sbi.SBI {
	title := fmt.Sprintf("Test SBI %s", id)
	description := fmt.Sprintf("Description for %s", id)

	metadata := sbi.SBIMetadata{
		EstimatedHours: 1.0,
		Priority:       0,
		Sequence:       1,
		RegisteredAt:   time.Now(),
		Labels:         []string{},
		AssignedAgent:  "gemini-cli",
		FilePaths:      filePaths,
	}

	s, _ := sbi.NewSBI(title, description, nil, metadata)

	// Set specific ID using reconstruction
	taskID, _ := model.NewTaskIDFromString(id)
	s = sbi.ReconstructSBI(
		taskID,
		title,
		description,
		model.StatusPending,
		model.StepPick,
		nil,
		metadata,
		s.ExecutionState(),
		time.Now(),
		time.Now(),
	)

	return s
}

// createTestSBIWithAgent creates a test SBI with specific agent assignment
func createTestSBIWithAgent(id string, agent string) *sbi.SBI {
	title := fmt.Sprintf("Test SBI %s", id)
	description := fmt.Sprintf("Description for %s", id)

	metadata := sbi.SBIMetadata{
		EstimatedHours: 1.0,
		Priority:       0,
		Sequence:       1,
		RegisteredAt:   time.Now(),
		Labels:         []string{},
		AssignedAgent:  agent,
		FilePaths:      []string{},
	}

	s, _ := sbi.NewSBI(title, description, nil, metadata)

	// Set specific ID using reconstruction
	taskID, _ := model.NewTaskIDFromString(id)
	s = sbi.ReconstructSBI(
		taskID,
		title,
		description,
		model.StatusPending,
		model.StepPick,
		nil,
		metadata,
		s.ExecutionState(),
		time.Now(),
		time.Now(),
	)

	return s
}

// Test 10: Agent pool integration test
func TestParallelSBIWorkflowRunner_AgentPoolControl(t *testing.T) {
	defer goleak.VerifyNone(t)

	// Setup container
	container := createTestContainer(t)
	defer container.Close()

	ctx := context.Background()
	sbiRepo := container.GetSBIRepository()

	// Create SBIs with different agent assignments
	// 3 for claude-code (max: 2), 2 for gemini-cli (max: 1)
	sbi1 := createTestSBIWithAgent("SBI-001", "claude-code")
	sbi2 := createTestSBIWithAgent("SBI-002", "claude-code")
	sbi3 := createTestSBIWithAgent("SBI-003", "claude-code")
	sbi4 := createTestSBIWithAgent("SBI-004", "gemini-cli")
	sbi5 := createTestSBIWithAgent("SBI-005", "gemini-cli")

	// Save all SBIs
	require.NoError(t, sbiRepo.Save(ctx, sbi1))
	require.NoError(t, sbiRepo.Save(ctx, sbi2))
	require.NoError(t, sbiRepo.Save(ctx, sbi3))
	require.NoError(t, sbiRepo.Save(ctx, sbi4))
	require.NoError(t, sbiRepo.Save(ctx, sbi5))

	// Create agent pool with limits
	agentPool := service.NewAgentPool()
	// Default limits: claude-code=2, gemini-cli=1

	// Track execution
	var mu sync.Mutex
	executedSBIs := []string{}
	agentCounts := make(map[string]int) // Track concurrent executions per agent

	executeTurn := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
		s, err := sbiRepo.Find(ctx, repository.SBIID(sbiID))
		if err != nil {
			return err
		}

		agent := s.Metadata().AssignedAgent

		mu.Lock()
		executedSBIs = append(executedSBIs, sbiID)
		agentCounts[agent]++
		mu.Unlock()

		// Simulate work
		time.Sleep(50 * time.Millisecond)

		return nil
	}

	// Create runner with agent pool
	runner := NewParallelSBIWorkflowRunnerWithAgentPool(container, 5, executeTurn, agentPool)

	config := workflow.WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 1 * time.Second,
		AutoFB:   false,
	}

	err := runner.Run(ctx, config)
	require.NoError(t, err)

	// Verify execution
	mu.Lock()
	defer mu.Unlock()

	// Should execute 3 SBIs total:
	// - 2 claude-code (up to limit)
	// - 1 gemini-cli (up to limit)
	assert.Len(t, executedSBIs, 3, "3 SBIs should execute based on agent limits")

	// Verify agent counts respect limits
	assert.LessOrEqual(t, agentCounts["claude-code"], 2, "claude-code should not exceed limit of 2")
	assert.LessOrEqual(t, agentCounts["gemini-cli"], 1, "gemini-cli should not exceed limit of 1")

	// Verify at least one of each agent type executed
	assert.Greater(t, agentCounts["claude-code"], 0, "At least one claude-code task should execute")
	assert.Greater(t, agentCounts["gemini-cli"], 0, "At least one gemini-cli task should execute")

	t.Logf("Executed SBIs: %v", executedSBIs)
	t.Logf("Agent counts: %v", agentCounts)
}
