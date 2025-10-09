package workflow_config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/application/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/workflow_sbi"
	"gopkg.in/yaml.v3"
)

// WorkflowConfiguration represents the full configuration file structure
type WorkflowConfiguration struct {
	Version   string                             `yaml:"version"`
	Run       RunConfiguration                   `yaml:"run"`
	Workflows map[string]workflow.WorkflowConfig `yaml:"workflows"`
}

// RunConfiguration holds global run settings
type RunConfiguration struct {
	DefaultInterval string `yaml:"default_interval"`
	MaxConcurrent   int    `yaml:"max_concurrent"`
	AutoRestart     bool   `yaml:"auto_restart"`
}

// LoadWorkflowConfig loads workflow configuration from file
func LoadWorkflowConfig(configPath string) (*WorkflowConfiguration, error) {
	// Default configuration
	config := &WorkflowConfiguration{
		Version: "1.0",
		Run: RunConfiguration{
			DefaultInterval: "5s",
			MaxConcurrent:   10,
			AutoRestart:     true,
		},
		Workflows: map[string]workflow.WorkflowConfig{
			"sbi": {
				Name:     "sbi",
				Enabled:  true,
				Interval: 5 * time.Second,
				AutoFB:   false,
				ExtraArgs: map[string]interface{}{
					"description": "Spec Backlog Item processing",
				},
			},
		},
	}

	// If config file doesn't exist, return default config
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil
	}

	// Read configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// Parse YAML
	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	// Post-process configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %v", err)
	}

	if err := config.ProcessIntervals(); err != nil {
		return nil, fmt.Errorf("failed to process intervals: %v", err)
	}

	return config, nil
}

// SaveWorkflowConfig saves workflow configuration to file
func SaveWorkflowConfig(config *WorkflowConfiguration, configPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Convert to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (wc *WorkflowConfiguration) Validate() error {
	if wc.Version == "" {
		wc.Version = "1.0"
	}

	if wc.Run.MaxConcurrent <= 0 {
		wc.Run.MaxConcurrent = 10
	}

	if wc.Run.DefaultInterval == "" {
		wc.Run.DefaultInterval = "5s"
	}

	// Validate each workflow config
	for name, workflow := range wc.Workflows {
		if workflow.Name == "" {
			workflow.Name = name
			wc.Workflows[name] = workflow
		}
		if workflow.Name != name {
			return fmt.Errorf("workflow name mismatch: key=%s, name=%s", name, workflow.Name)
		}
	}

	return nil
}

// ProcessIntervals converts string intervals to time.Duration
func (wc *WorkflowConfiguration) ProcessIntervals() error {
	// Process default interval
	if wc.Run.DefaultInterval != "" {
		defaultInterval, err := time.ParseDuration(wc.Run.DefaultInterval)
		if err != nil {
			return fmt.Errorf("invalid default interval: %v", err)
		}

		// Apply to workflows that don't have intervals set
		for name, workflow := range wc.Workflows {
			if workflow.Interval == 0 {
				workflow.Interval = defaultInterval
				wc.Workflows[name] = workflow
			}
		}
	}

	return nil
}

// GetDefaultConfigPath returns the default configuration file path
func GetDefaultConfigPath() string {
	return filepath.Join(".deespec", "workflow.yaml")
}

// CreateManagerFromConfig creates a workflow manager from configuration
func CreateManagerFromConfig(configPath string) (*workflow.WorkflowManager, error) {
	config, err := LoadWorkflowConfig(configPath)
	if err != nil {
		return nil, err
	}

	manager := workflow.NewWorkflowManager(common.Info, common.Warn, common.Debug)

	// Register workflows based on configuration
	for name, workflowConfig := range config.Workflows {
		var runner workflow.WorkflowRunner

		switch name {
		case "sbi":
			runner = workflow_sbi.NewSBIWorkflowRunner()
		default:
			common.Warn("Unknown workflow type: %s, skipping\n", name)
			continue
		}

		if err := manager.RegisterWorkflow(runner, workflowConfig); err != nil {
			return nil, fmt.Errorf("failed to register workflow %s: %v", name, err)
		}
	}

	return manager, nil
}

// GenerateExampleConfig creates an example configuration file
func GenerateExampleConfig(configPath string) error {
	config := &WorkflowConfiguration{
		Version: "1.0",
		Run: RunConfiguration{
			DefaultInterval: "5s",
			MaxConcurrent:   5,
			AutoRestart:     true,
		},
		Workflows: map[string]workflow.WorkflowConfig{
			"sbi": {
				Name:     "sbi",
				Enabled:  true,
				Interval: 5 * time.Second,
				AutoFB:   false,
				ExtraArgs: map[string]interface{}{
					"description": "Spec Backlog Item processing workflow",
					"priority":    1,
				},
			},
			"pbi": {
				Name:     "pbi",
				Enabled:  false, // Disabled by default as PBI is not implemented yet
				Interval: 10 * time.Second,
				AutoFB:   false,
				ExtraArgs: map[string]interface{}{
					"description": "Product Backlog Item processing workflow (future)",
					"priority":    2,
				},
			},
		},
	}

	return SaveWorkflowConfig(config, configPath)
}

// WorkflowConfigBuilder helps build workflow configurations
type WorkflowConfigBuilder struct {
	config workflow.WorkflowConfig
}

// NewWorkflowConfigBuilder creates a new builder
func NewWorkflowConfigBuilder(name string) *WorkflowConfigBuilder {
	return &WorkflowConfigBuilder{
		config: workflow.WorkflowConfig{
			Name:      name,
			Enabled:   true,
			Interval:  service.DefaultRunInterval,
			AutoFB:    false,
			ExtraArgs: make(map[string]interface{}),
		},
	}
}

// WithEnabled sets the enabled state
func (wcb *WorkflowConfigBuilder) WithEnabled(enabled bool) *WorkflowConfigBuilder {
	wcb.config.Enabled = enabled
	return wcb
}

// WithInterval sets the execution interval
func (wcb *WorkflowConfigBuilder) WithInterval(interval time.Duration) *WorkflowConfigBuilder {
	wcb.config.Interval = interval
	return wcb
}

// WithAutoFB sets the auto-FB flag
func (wcb *WorkflowConfigBuilder) WithAutoFB(autoFB bool) *WorkflowConfigBuilder {
	wcb.config.AutoFB = autoFB
	return wcb
}

// WithExtraArg adds an extra argument
func (wcb *WorkflowConfigBuilder) WithExtraArg(key string, value interface{}) *WorkflowConfigBuilder {
	wcb.config.ExtraArgs[key] = value
	return wcb
}

// Build returns the built configuration
func (wcb *WorkflowConfigBuilder) Build() workflow.WorkflowConfig {
	return wcb.config
}
