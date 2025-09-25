package workflow

// AllowedAgents defines the allowed agent types for workflow steps
var AllowedAgents = []string{"claude_cli", "system"}

// allowedAgentsSet for quick lookup
var allowedAgentsSet = map[string]struct{}{
	"claude_cli": {},
	"system":     {},
}

// IsAllowedAgent checks if an agent is in the allowed set
func IsAllowedAgent(agent string) bool {
	_, ok := allowedAgentsSet[agent]
	return ok
}

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
	Name  string            `yaml:"name"`
	Steps []Step            `yaml:"steps"`
	Vars  map[string]string `yaml:"vars,omitempty"` // Optional variables for prompt expansion
}