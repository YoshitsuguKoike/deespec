package workflow

import "time"

// WorkflowConfig holds configuration for a specific workflow
type WorkflowConfig struct {
	Name      string                 `yaml:"name"`
	Enabled   bool                   `yaml:"enabled"`
	Interval  time.Duration          `yaml:"interval"`
	AutoFB    bool                   `yaml:"auto_fb"`
	ExtraArgs map[string]interface{} `yaml:"extra_args"`
}

// ManagerConfig holds configuration for the workflow manager
type ManagerConfig struct {
	DefaultInterval time.Duration
	MaxConcurrent   int
	Workflows       []WorkflowConfig
}
