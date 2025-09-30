package cli

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestSBIWorkflowRunner_NewSBIWorkflowRunner(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	runner := NewSBIWorkflowRunner()
	if runner == nil {
		t.Fatal("NewSBIWorkflowRunner returned nil")
	}

	if !runner.IsEnabled() {
		t.Error("Expected runner to be enabled by default")
	}
}

func TestSBIWorkflowRunner_Name(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	runner := NewSBIWorkflowRunner()
	name := runner.Name()

	expected := "sbi"
	if name != expected {
		t.Errorf("Expected name '%s', got '%s'", expected, name)
	}
}

func TestSBIWorkflowRunner_Description(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	runner := NewSBIWorkflowRunner()
	description := runner.Description()

	if description == "" {
		t.Error("Description should not be empty")
	}

	if description != "Spec Backlog Item processing workflow" {
		t.Errorf("Unexpected description: %s", description)
	}
}

func TestSBIWorkflowRunner_IsEnabled(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	runner := NewSBIWorkflowRunner()

	// Test default enabled state
	if !runner.IsEnabled() {
		t.Error("Expected runner to be enabled by default")
	}

	// Test setting enabled/disabled
	runner.SetEnabled(false)
	if runner.IsEnabled() {
		t.Error("Expected runner to be disabled after SetEnabled(false)")
	}

	runner.SetEnabled(true)
	if !runner.IsEnabled() {
		t.Error("Expected runner to be enabled after SetEnabled(true)")
	}
}

func TestSBIWorkflowRunner_Validate(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	runner := NewSBIWorkflowRunner()
	err := runner.Validate()

	if err != nil {
		t.Errorf("Validation failed: %v", err)
	}
}

func TestSBIWorkflowRunner_Run_Context_Cancellation(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	runner := NewSBIWorkflowRunner()

	// Test context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := WorkflowConfig{
		Name:     "sbi",
		AutoFB:   false,
		Interval: 1 * time.Second,
	}

	err := runner.Run(ctx, config)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func TestSBIWorkflowRunner_Run_Context_Timeout(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	runner := NewSBIWorkflowRunner()

	// Test context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure context times out

	config := WorkflowConfig{
		Name:     "sbi",
		AutoFB:   false,
		Interval: 1 * time.Second,
	}

	err := runner.Run(ctx, config)
	if err != context.DeadlineExceeded {
		t.Errorf("Expected context.DeadlineExceeded error, got %v", err)
	}
}

func TestSBIWorkflowRunner_Run_AutoFB_Config(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	// Skip this test as it requires full app context which isn't available in unit tests
	t.Skip("Skipping integration test that requires full app context")

	runner := NewSBIWorkflowRunner()
	ctx := context.Background()

	// Test with AutoFB enabled in config
	config := WorkflowConfig{
		Name:     "sbi",
		AutoFB:   true,
		Interval: 1 * time.Second,
	}

	// This test assumes runOnce is available and testable
	// In a real scenario, you might want to mock this dependency
	err := runner.Run(ctx, config)
	// We can't easily test the success case without mocking runOnce
	// So we just verify it doesn't panic and returns some result
	_ = err // Suppress unused variable warning
}

func TestSBIWorkflowRunner_Run_AutoFB_GlobalConfig(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	// Skip this test as it requires full app context which isn't available in unit tests
	t.Skip("Skipping integration test that requires full app context")

	runner := NewSBIWorkflowRunner()
	ctx := context.Background()

	// Test with AutoFB disabled in config but enabled globally
	config := WorkflowConfig{
		Name:     "sbi",
		AutoFB:   false,
		Interval: 1 * time.Second,
	}

	// Note: Testing global config requires access to globalConfig variable
	// This test assumes the global config behavior is tested elsewhere
	err := runner.Run(ctx, config)
	_ = err // Suppress unused variable warning
}

func TestSBIWorkflowRunner_Integration_WithManager(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	// Integration test with WorkflowManager
	wm := NewWorkflowManager()
	defer wm.Stop()

	runner := NewSBIWorkflowRunner()
	config := WorkflowConfig{
		Name:     "sbi",
		Enabled:  true,
		Interval: 100 * time.Millisecond,
		AutoFB:   false,
	}

	err := wm.RegisterWorkflow(runner, config)
	if err != nil {
		t.Fatalf("Failed to register SBI workflow: %v", err)
	}

	// Verify registration
	names := wm.GetWorkflowNames()
	if len(names) != 1 || names[0] != "sbi" {
		t.Errorf("Expected workflow names [sbi], got %v", names)
	}

	enabled := wm.GetEnabledWorkflows()
	if len(enabled) != 1 || enabled[0] != "sbi" {
		t.Errorf("Expected enabled workflows [sbi], got %v", enabled)
	}

	// Skip actually running the workflow in unit tests as it requires full app context
	if testing.Short() {
		t.Skip("Skipping workflow execution in short mode")
	}

	// Test starting the workflow (will fail due to runOnce dependency)
	// But we can verify the manager accepts it
	err = wm.RunWorkflow("sbi")
	if err != nil {
		// This is expected to fail in test environment due to missing dependencies
		t.Logf("SBI workflow start failed as expected: %v", err)
	}

	// Give it a brief moment to attempt execution
	time.Sleep(50 * time.Millisecond)

	// Check stats
	stats := wm.GetStats()
	sbiStats := stats["sbi"]
	if sbiStats == nil {
		t.Error("No stats found for SBI workflow")
	}
}
