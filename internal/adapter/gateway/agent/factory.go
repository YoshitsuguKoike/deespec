package agent

import (
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// NewAgentGateway creates an agent gateway based on agent type
// Supported types: claude-code, claude-code-cli, gemini-cli, codex
// Note: User is responsible for ensuring the agent is available (e.g., claude CLI installed)
func NewAgentGateway(agentType string) (output.AgentGateway, error) {
	switch agentType {
	case "claude-code":
		// API version (requires ANTHROPIC_API_KEY)
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set for claude-code")
		}
		return NewClaudeCodeGateway(apiKey), nil

	case "claude-code-cli":
		// CLI version (assumes `claude` command is available)
		return NewClaudeCodeCLIGateway(), nil

	case "gemini-cli":
		return NewGeminiMockGateway(), nil

	case "codex":
		return NewCodexMockGateway(), nil

	default:
		return nil, fmt.Errorf("unknown agent type: %s (supported: claude-code, claude-code-cli, gemini-cli, codex)", agentType)
	}
}

// GetAvailableAgents returns a list of available agent types
func GetAvailableAgents() []string {
	agents := []string{}

	// Check if Claude Code is available
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		agents = append(agents, "claude-code")
	}

	// Mock agents are always available
	agents = append(agents, "gemini-cli", "codex")

	return agents
}

// GetDefaultAgent returns the default agent type to use
func GetDefaultAgent() string {
	// Default to Claude Code CLI (assumes user has it installed)
	return "claude-code-cli"
}
