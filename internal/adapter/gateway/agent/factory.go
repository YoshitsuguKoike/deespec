package agent

import (
	"fmt"
	"os"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// NewAgentGateway creates an agent gateway based on agent type
// Supported types: claude-code, gemini-cli, codex
func NewAgentGateway(agentType string) (output.AgentGateway, error) {
	switch agentType {
	case "claude-code":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable not set")
		}
		return NewClaudeCodeGateway(apiKey), nil

	case "gemini-cli":
		return NewGeminiMockGateway(), nil

	case "codex":
		return NewCodexMockGateway(), nil

	default:
		return nil, fmt.Errorf("unknown agent type: %s (supported: claude-code, gemini-cli, codex)", agentType)
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
	// Prefer Claude Code if API key is available
	if os.Getenv("ANTHROPIC_API_KEY") != "" {
		return "claude-code"
	}

	// Fallback to mock agents
	return "gemini-cli"
}
