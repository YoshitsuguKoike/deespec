package output

import (
	"context"
	"time"
)

// AgentGateway is the interface for AI agent execution
// This abstraction allows different AI backends (Claude, Gemini, Codex)
type AgentGateway interface {
	// Execute runs the agent with given request
	Execute(ctx context.Context, req AgentRequest) (*AgentResponse, error)

	// GetCapability returns the agent's capabilities
	GetCapability() AgentCapability

	// HealthCheck verifies if the agent is available
	HealthCheck(ctx context.Context) error
}

// AgentRequest represents a request to an AI agent
type AgentRequest struct {
	Prompt      string            // The prompt to send to the agent
	Timeout     time.Duration     // Execution timeout
	Context     map[string]string // Additional context information
	MaxTokens   int               // Maximum tokens to generate (if applicable)
	Temperature float64           // Temperature for generation (0.0-1.0)
}

// AgentResponse represents the response from an AI agent
type AgentResponse struct {
	Output     string            // Generated output
	ExitCode   int               // Exit code (for CLI-based agents)
	Duration   time.Duration     // Execution duration
	TokensUsed int               // Number of tokens used (if applicable)
	AgentType  string            // Type of agent that executed (claude/gemini/codex)
	Metadata   map[string]string // Additional metadata
}

// AgentCapability describes what an agent can do
type AgentCapability struct {
	SupportsCodeGeneration bool   // Can generate code
	SupportsReview         bool   // Can review code
	SupportsTest           bool   // Can generate tests
	MaxPromptSize          int    // Maximum prompt size in bytes
	ConcurrentTasks        int    // Number of concurrent tasks supported
	AgentType              string // Agent type identifier
}
