package cli

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

// MockWorkflowRunner implements WorkflowRunner for testing
type MockWorkflowRunner struct {
	name        string
	description string
	enabled     bool
	runCount    int
	runDuration time.Duration
	shouldError bool
	mutex       sync.Mutex
}

func NewMockWorkflowRunner(name string) *MockWorkflowRunner {
	return &MockWorkflowRunner{
		name:        name,
		description: fmt.Sprintf("Mock workflow: %s", name),
		enabled:     true,
		runDuration: 10 * time.Millisecond,
	}
}

func (mwr *MockWorkflowRunner) Name() string {
	return mwr.name
}

func (mwr *MockWorkflowRunner) Description() string {
	return mwr.description
}

func (mwr *MockWorkflowRunner) IsEnabled() bool {
	return mwr.enabled
}

func (mwr *MockWorkflowRunner) Run(ctx context.Context, config WorkflowConfig) error {
	mwr.mutex.Lock()
	defer mwr.mutex.Unlock()

	// Check for cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Simulate work
	time.Sleep(mwr.runDuration)
	mwr.runCount++

	if mwr.shouldError {
		return fmt.Errorf("mock error from %s", mwr.name)
	}

	return nil
}

func (mwr *MockWorkflowRunner) SetEnabled(enabled bool) {
	mwr.mutex.Lock()
	defer mwr.mutex.Unlock()
	mwr.enabled = enabled
}

func (mwr *MockWorkflowRunner) SetShouldError(shouldError bool) {
	mwr.mutex.Lock()
	defer mwr.mutex.Unlock()
	mwr.shouldError = shouldError
}

func (mwr *MockWorkflowRunner) GetRunCount() int {
	mwr.mutex.Lock()
	defer mwr.mutex.Unlock()
	return mwr.runCount
}

func TestWorkflowManager_NewWorkflowManager(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	if wm == nil {
		t.Fatal("NewWorkflowManager returned nil")
	}

	if wm.workflows == nil {
		t.Error("workflows map is nil")
	}

	if wm.configs == nil {
		t.Error("configs map is nil")
	}

	if wm.stats == nil {
		t.Error("stats map is nil")
	}

	if wm.ctx == nil {
		t.Error("context is nil")
	}
}

func TestWorkflowManager_RegisterWorkflow(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	runner := NewMockWorkflowRunner("test-workflow")
	config := WorkflowConfig{
		Name:     "test-workflow",
		Enabled:  true,
		Interval: 100 * time.Millisecond,
	}

	err := wm.RegisterWorkflow(runner, config)
	if err != nil {
		t.Fatalf("Failed to register workflow: %v", err)
	}

	// Test duplicate registration
	err = wm.RegisterWorkflow(runner, config)
	if err == nil {
		t.Error("Expected error for duplicate workflow registration")
	}

	// Verify registration
	names := wm.GetWorkflowNames()
	if len(names) != 1 || names[0] != "test-workflow" {
		t.Errorf("Expected workflow names [test-workflow], got %v", names)
	}
}

func TestWorkflowManager_GetEnabledWorkflows(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	// Register enabled workflow
	enabledRunner := NewMockWorkflowRunner("enabled-workflow")
	enabledConfig := WorkflowConfig{
		Name:    "enabled-workflow",
		Enabled: true,
	}
	wm.RegisterWorkflow(enabledRunner, enabledConfig)

	// Register disabled workflow (config disabled)
	disabledRunner := NewMockWorkflowRunner("disabled-workflow")
	disabledConfig := WorkflowConfig{
		Name:    "disabled-workflow",
		Enabled: false,
	}
	wm.RegisterWorkflow(disabledRunner, disabledConfig)

	// Register runner-disabled workflow (runner disabled)
	runnerDisabledRunner := NewMockWorkflowRunner("runner-disabled-workflow")
	runnerDisabledRunner.SetEnabled(false)
	runnerDisabledConfig := WorkflowConfig{
		Name:    "runner-disabled-workflow",
		Enabled: true,
	}
	wm.RegisterWorkflow(runnerDisabledRunner, runnerDisabledConfig)

	enabled := wm.GetEnabledWorkflows()
	if len(enabled) != 1 || enabled[0] != "enabled-workflow" {
		t.Errorf("Expected enabled workflows [enabled-workflow], got %v", enabled)
	}
}

func TestWorkflowManager_RunWorkflow(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	runner := NewMockWorkflowRunner("test-workflow")
	config := WorkflowConfig{
		Name:     "test-workflow",
		Enabled:  true,
		Interval: 50 * time.Millisecond,
	}
	wm.RegisterWorkflow(runner, config)

	// Start workflow
	err := wm.RunWorkflow("test-workflow")
	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// Wait for a few executions
	time.Sleep(200 * time.Millisecond)

	// Stop and check
	wm.Stop()

	runCount := runner.GetRunCount()
	if runCount < 2 {
		t.Errorf("Expected at least 2 runs, got %d", runCount)
	}

	// Test running non-existent workflow
	err = wm.RunWorkflow("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent workflow")
	}
}

func TestWorkflowManager_RunAll(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	// Register multiple workflows
	runner1 := NewMockWorkflowRunner("workflow1")
	config1 := WorkflowConfig{
		Name:     "workflow1",
		Enabled:  true,
		Interval: 50 * time.Millisecond,
	}
	wm.RegisterWorkflow(runner1, config1)

	runner2 := NewMockWorkflowRunner("workflow2")
	config2 := WorkflowConfig{
		Name:     "workflow2",
		Enabled:  true,
		Interval: 50 * time.Millisecond,
	}
	wm.RegisterWorkflow(runner2, config2)

	// Start all workflows
	err := wm.RunAll()
	if err != nil {
		t.Fatalf("Failed to start all workflows: %v", err)
	}

	// Wait for executions
	time.Sleep(200 * time.Millisecond)

	// Stop and check
	wm.Stop()

	runCount1 := runner1.GetRunCount()
	runCount2 := runner2.GetRunCount()

	if runCount1 < 2 {
		t.Errorf("Workflow1 expected at least 2 runs, got %d", runCount1)
	}
	if runCount2 < 2 {
		t.Errorf("Workflow2 expected at least 2 runs, got %d", runCount2)
	}
}

func TestWorkflowManager_ErrorHandling(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	runner := NewMockWorkflowRunner("error-workflow")
	runner.SetShouldError(true)
	config := WorkflowConfig{
		Name:     "error-workflow",
		Enabled:  true,
		Interval: 50 * time.Millisecond,
	}
	wm.RegisterWorkflow(runner, config)

	// Start workflow
	err := wm.RunWorkflow("error-workflow")
	if err != nil {
		t.Fatalf("Failed to start workflow: %v", err)
	}

	// Wait for a few executions
	time.Sleep(200 * time.Millisecond)

	// Stop and check stats
	wm.Stop()

	stats := wm.GetStats()
	workflowStats := stats["error-workflow"]

	if workflowStats.FailedRuns == 0 {
		t.Error("Expected failed runs but got 0")
	}
	if workflowStats.LastError == nil {
		t.Error("Expected last error to be set")
	}
}

func TestWorkflowManager_Stats(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	runner := NewMockWorkflowRunner("stats-workflow")
	config := WorkflowConfig{
		Name:     "stats-workflow",
		Enabled:  true,
		Interval: 50 * time.Millisecond,
	}
	wm.RegisterWorkflow(runner, config)

	// Start workflow
	wm.RunWorkflow("stats-workflow")

	// Wait for executions
	time.Sleep(200 * time.Millisecond)

	// Stop and check stats
	wm.Stop()

	stats := wm.GetStats()
	workflowStats := stats["stats-workflow"]

	if workflowStats == nil {
		t.Fatal("No stats found for workflow")
	}

	if workflowStats.Name != "stats-workflow" {
		t.Errorf("Expected name 'stats-workflow', got '%s'", workflowStats.Name)
	}

	if workflowStats.TotalExecutions == 0 {
		t.Error("Expected total executions > 0")
	}

	if workflowStats.SuccessfulRuns == 0 {
		t.Error("Expected successful runs > 0")
	}

	if workflowStats.LastExecution.IsZero() {
		t.Error("Expected last execution time to be set")
	}
}

func TestWorkflowManager_ConcurrentAccess(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	wm := NewWorkflowManager()
	defer wm.Stop()

	// Test concurrent workflow registration
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			runner := NewMockWorkflowRunner(fmt.Sprintf("workflow-%d", id))
			config := WorkflowConfig{
				Name:    fmt.Sprintf("workflow-%d", id),
				Enabled: true,
			}
			wm.RegisterWorkflow(runner, config)
		}(i)
	}

	wg.Wait()

	// Check all workflows were registered
	names := wm.GetWorkflowNames()
	if len(names) != 10 {
		t.Errorf("Expected 10 workflows, got %d", len(names))
	}

	// Test concurrent stats access
	wm.RunAll()
	time.Sleep(100 * time.Millisecond)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stats := wm.GetStats()
			if len(stats) != 10 {
				t.Errorf("Expected 10 stats entries, got %d", len(stats))
			}
		}()
	}

	wg.Wait()
	wm.Stop()
}
