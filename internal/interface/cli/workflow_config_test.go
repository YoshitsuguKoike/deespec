package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/goleak"
	"gopkg.in/yaml.v3"
)

func TestLoadWorkflowConfig_DefaultConfig(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	// Test loading non-existent config file (should return default)
	config, err := LoadWorkflowConfig("non/existent/path.yaml")
	if err != nil {
		t.Fatalf("Expected no error for non-existent config, got: %v", err)
	}

	if config == nil {
		t.Fatal("Config should not be nil")
	}

	if config.Version != "1.0" {
		t.Errorf("Expected version '1.0', got '%s'", config.Version)
	}

	if config.Run.DefaultInterval != "5s" {
		t.Errorf("Expected default interval '5s', got '%s'", config.Run.DefaultInterval)
	}

	if config.Run.MaxConcurrent != 10 {
		t.Errorf("Expected max concurrent 10, got %d", config.Run.MaxConcurrent)
	}

	if !config.Run.AutoRestart {
		t.Error("Expected auto restart to be true")
	}

	// Check default SBI workflow
	sbiWorkflow, exists := config.Workflows["sbi"]
	if !exists {
		t.Fatal("Default SBI workflow should exist")
	}

	if sbiWorkflow.Name != "sbi" {
		t.Errorf("Expected SBI workflow name 'sbi', got '%s'", sbiWorkflow.Name)
	}

	if !sbiWorkflow.Enabled {
		t.Error("Expected SBI workflow to be enabled")
	}

	if sbiWorkflow.Interval != 5*time.Second {
		t.Errorf("Expected SBI interval 5s, got %v", sbiWorkflow.Interval)
	}
}

func TestLoadWorkflowConfig_ValidFile(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	configData := `
version: "1.5"
run:
  default_interval: "10s"
  max_concurrent: 5
  auto_restart: false
workflows:
  sbi:
    name: "sbi"
    enabled: true
    interval: 15s
    auto_fb: true
    extra_args:
      description: "Test SBI workflow"
      priority: 1
  pbi:
    name: "pbi"
    enabled: false
    interval: 30s
    auto_fb: false
`

	err := os.WriteFile(configPath, []byte(configData), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	config, err := LoadWorkflowConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded values
	if config.Version != "1.5" {
		t.Errorf("Expected version '1.5', got '%s'", config.Version)
	}

	if config.Run.DefaultInterval != "10s" {
		t.Errorf("Expected default interval '10s', got '%s'", config.Run.DefaultInterval)
	}

	if config.Run.MaxConcurrent != 5 {
		t.Errorf("Expected max concurrent 5, got %d", config.Run.MaxConcurrent)
	}

	if config.Run.AutoRestart {
		t.Error("Expected auto restart to be false")
	}

	// Check SBI workflow
	sbi := config.Workflows["sbi"]
	if sbi.Interval != 15*time.Second {
		t.Errorf("Expected SBI interval 15s, got %v", sbi.Interval)
	}

	if !sbi.AutoFB {
		t.Error("Expected SBI AutoFB to be true")
	}

	// Check PBI workflow
	pbi := config.Workflows["pbi"]
	if pbi.Enabled {
		t.Error("Expected PBI to be disabled")
	}

	if pbi.Interval != 30*time.Second {
		t.Errorf("Expected PBI interval 30s, got %v", pbi.Interval)
	}
}

func TestLoadWorkflowConfig_InvalidYAML(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `
version: "1.0"
run:
  default_interval: "5s"
  invalid_yaml_structure: [
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err = LoadWorkflowConfig(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestSaveWorkflowConfig(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "save_test.yaml")

	config := &WorkflowConfiguration{
		Version: "1.0",
		Run: RunConfiguration{
			DefaultInterval: "5s",
			MaxConcurrent:   10,
			AutoRestart:     true,
		},
		Workflows: map[string]WorkflowConfig{
			"test": {
				Name:     "test",
				Enabled:  true,
				Interval: 10 * time.Second,
				AutoFB:   false,
				ExtraArgs: map[string]interface{}{
					"description": "Test workflow",
				},
			},
		},
	}

	err := SaveWorkflowConfig(config, configPath)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load back and verify
	loadedConfig, err := LoadWorkflowConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loadedConfig.Version != config.Version {
		t.Errorf("Version mismatch: expected %s, got %s", config.Version, loadedConfig.Version)
	}

	testWorkflow := loadedConfig.Workflows["test"]
	if testWorkflow.Name != "test" {
		t.Errorf("Workflow name mismatch: expected 'test', got '%s'", testWorkflow.Name)
	}
}

func TestWorkflowConfiguration_Validate(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	// Test validation with missing version
	config := &WorkflowConfiguration{
		Run: RunConfiguration{
			MaxConcurrent: 5,
		},
		Workflows: map[string]WorkflowConfig{
			"test": {
				Name: "test",
			},
		},
	}

	err := config.Validate()
	if err != nil {
		t.Fatalf("Validation failed: %v", err)
	}

	// Check defaults were applied
	if config.Version != "1.0" {
		t.Errorf("Expected default version '1.0', got '%s'", config.Version)
	}

	if config.Run.DefaultInterval != "5s" {
		t.Errorf("Expected default interval '5s', got '%s'", config.Run.DefaultInterval)
	}

	// Test validation with workflow name mismatch
	config.Workflows["mismatch"] = WorkflowConfig{
		Name: "different-name",
	}

	err = config.Validate()
	if err == nil {
		t.Error("Expected validation error for workflow name mismatch")
	}
}

func TestWorkflowConfiguration_ProcessIntervals(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	config := &WorkflowConfiguration{
		Run: RunConfiguration{
			DefaultInterval: "10s",
		},
		Workflows: map[string]WorkflowConfig{
			"no-interval": {
				Name:     "no-interval",
				Interval: 0, // No interval set
			},
			"with-interval": {
				Name:     "with-interval",
				Interval: 5 * time.Second, // Already has interval
			},
		},
	}

	err := config.ProcessIntervals()
	if err != nil {
		t.Fatalf("ProcessIntervals failed: %v", err)
	}

	// Check that default interval was applied to workflow without interval
	noIntervalWorkflow := config.Workflows["no-interval"]
	if noIntervalWorkflow.Interval != 10*time.Second {
		t.Errorf("Expected default interval 10s, got %v", noIntervalWorkflow.Interval)
	}

	// Check that existing interval was preserved
	withIntervalWorkflow := config.Workflows["with-interval"]
	if withIntervalWorkflow.Interval != 5*time.Second {
		t.Errorf("Expected existing interval 5s, got %v", withIntervalWorkflow.Interval)
	}
}

func TestWorkflowConfiguration_ProcessIntervals_InvalidDefault(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	config := &WorkflowConfiguration{
		Run: RunConfiguration{
			DefaultInterval: "invalid-duration",
		},
		Workflows: map[string]WorkflowConfig{},
	}

	err := config.ProcessIntervals()
	if err == nil {
		t.Error("Expected error for invalid default interval")
	}
}

func TestGetDefaultConfigPath(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	path := GetDefaultConfigPath()
	expected := filepath.Join(".deespec", "workflow.yaml")

	if path != expected {
		t.Errorf("Expected path '%s', got '%s'", expected, path)
	}
}

func TestGenerateExampleConfig(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "example.yaml")

	err := GenerateExampleConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to generate example config: %v", err)
	}

	// Verify file was created and is valid YAML
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read generated config: %v", err)
	}

	var config WorkflowConfiguration
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		t.Fatalf("Generated config is not valid YAML: %v", err)
	}

	// Verify it contains expected workflows
	if _, exists := config.Workflows["sbi"]; !exists {
		t.Error("Example config should contain SBI workflow")
	}

	if _, exists := config.Workflows["pbi"]; !exists {
		t.Error("Example config should contain PBI workflow")
	}

	// Verify PBI is disabled by default
	pbi := config.Workflows["pbi"]
	if pbi.Enabled {
		t.Error("PBI workflow should be disabled in example config")
	}
}

func TestCreateManagerFromConfig(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "manager_test.yaml")

	// Create test config
	config := &WorkflowConfiguration{
		Version: "1.0",
		Run: RunConfiguration{
			DefaultInterval: "5s",
			MaxConcurrent:   5,
		},
		Workflows: map[string]WorkflowConfig{
			"sbi": {
				Name:     "sbi",
				Enabled:  true,
				Interval: 5 * time.Second,
			},
		},
	}

	err := SaveWorkflowConfig(config, configPath)
	if err != nil {
		t.Fatalf("Failed to save test config: %v", err)
	}

	// Create manager from config
	manager, err := CreateManagerFromConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to create manager from config: %v", err)
	}
	defer manager.Stop()

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	// Verify SBI workflow was registered
	workflows := manager.GetWorkflowNames()
	if len(workflows) != 1 || workflows[0] != "sbi" {
		t.Errorf("Expected workflows [sbi], got %v", workflows)
	}
}

func TestWorkflowConfigBuilder(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreTopFunction("github.com/YoshitsuguKoike/deespec/internal/interface/cli.setupSignalHandler.func1"))

	builder := NewWorkflowConfigBuilder("test-workflow")
	config := builder.
		WithEnabled(false).
		WithInterval(30*time.Second).
		WithAutoFB(true).
		WithExtraArg("priority", 5).
		WithExtraArg("description", "Test workflow").
		Build()

	if config.Name != "test-workflow" {
		t.Errorf("Expected name 'test-workflow', got '%s'", config.Name)
	}

	if config.Enabled {
		t.Error("Expected workflow to be disabled")
	}

	if config.Interval != 30*time.Second {
		t.Errorf("Expected interval 30s, got %v", config.Interval)
	}

	if !config.AutoFB {
		t.Error("Expected AutoFB to be true")
	}

	if config.ExtraArgs["priority"] != 5 {
		t.Errorf("Expected priority 5, got %v", config.ExtraArgs["priority"])
	}

	if config.ExtraArgs["description"] != "Test workflow" {
		t.Errorf("Expected description 'Test workflow', got %v", config.ExtraArgs["description"])
	}
}
