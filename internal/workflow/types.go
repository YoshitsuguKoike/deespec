package workflow

// Decision represents the decision configuration for a workflow step
type Decision struct {
	Regex string `yaml:"regex"`
}

// Step represents a single step in the workflow
type Step struct {
	ID                string    `yaml:"id"`
	Agent             string    `yaml:"agent"`
	PromptPath        string    `yaml:"prompt_path"`
	Decision          *Decision `yaml:"decision,omitempty"`
	ResolvedPromptPath string   `yaml:"-"` // Internal: absolute path resolved from prompt_path
}

// Workflow represents the complete workflow configuration
type Workflow struct {
	Name  string `yaml:"name"`
	Steps []Step `yaml:"steps"`
}