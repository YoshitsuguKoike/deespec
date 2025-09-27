package workflow

import "regexp"

// AllowedAgents defines the allowed agent types for workflow steps
var AllowedAgents = []string{"claude_cli", "system"}

// allowedAgentsSet for quick lookup
var allowedAgentsSet = map[string]struct{}{
	"claude_cli": {},
	"system":     {},
}

// DefaultDecisionRegex is the default pattern for review decisions
const DefaultDecisionRegex = `^DECISION:\s+(OK|NEEDS_CHANGES)\s*$`

// DefaultMaxPromptKB is the default maximum prompt size in KB
const DefaultMaxPromptKB = 64

// MaxPromptKBUpper is the upper limit for prompt size in KB
const MaxPromptKBUpper = 512

// IsAllowedAgent checks if an agent is in the allowed set
func IsAllowedAgent(agent string) bool {
	_, ok := allowedAgentsSet[agent]
	return ok
}

// Constraints represents the workflow constraints
type Constraints struct {
	MaxPromptKB int `yaml:"max_prompt_kb"`
}

// Decision represents the decision configuration for a workflow step
type Decision struct {
	Regex string `yaml:"regex"`
}

// Step represents a single step in the workflow
type Step struct {
	ID                 string         `yaml:"id"`
	Agent              string         `yaml:"agent"`
	PromptPath         string         `yaml:"prompt_path"`
	Decision           *Decision      `yaml:"decision,omitempty"`
	ResolvedPromptPath string         `yaml:"-"` // Internal: absolute path resolved from prompt_path
	CompiledDecision   *regexp.Regexp `yaml:"-"` // Internal: compiled regex for review decision
}

// Workflow represents the complete workflow configuration
type Workflow struct {
	Name        string            `yaml:"name"`
	Steps       []Step            `yaml:"steps"`
	Vars        map[string]string `yaml:"vars,omitempty"`        // Optional variables for prompt expansion
	Constraints Constraints       `yaml:"constraints,omitempty"` // Optional constraints for workflow execution
}
