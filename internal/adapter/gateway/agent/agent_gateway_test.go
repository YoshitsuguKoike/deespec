package agent_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/adapter/gateway/agent"
	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

func TestGeminiMockGateway(t *testing.T) {
	gateway := agent.NewGeminiMockGateway()

	// Test Execute
	req := output.AgentRequest{
		Prompt:    "Write a function to calculate fibonacci",
		MaxTokens: 1000,
		Timeout:   time.Minute,
	}

	resp, err := gateway.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.AgentType != "gemini-cli" {
		t.Errorf("AgentType = %s, want gemini-cli", resp.AgentType)
	}

	if resp.Output == "" {
		t.Error("Output should not be empty")
	}

	// Test GetCapability
	cap := gateway.GetCapability()
	if cap.AgentType != "gemini-cli" {
		t.Errorf("Capability AgentType = %s, want gemini-cli", cap.AgentType)
	}

	if !cap.SupportsCodeGeneration {
		t.Error("Should support code generation")
	}

	// Test HealthCheck
	if err := gateway.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}
}

func TestCodexMockGateway(t *testing.T) {
	gateway := agent.NewCodexMockGateway()

	// Test Execute
	req := output.AgentRequest{
		Prompt:    "Generate unit tests",
		MaxTokens: 500,
		Timeout:   time.Minute,
	}

	resp, err := gateway.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.AgentType != "codex" {
		t.Errorf("AgentType = %s, want codex", resp.AgentType)
	}

	if resp.Output == "" {
		t.Error("Output should not be empty")
	}

	// Test GetCapability
	cap := gateway.GetCapability()
	if cap.AgentType != "codex" {
		t.Errorf("Capability AgentType = %s, want codex", cap.AgentType)
	}

	if !cap.SupportsCodeGeneration {
		t.Error("Should support code generation")
	}

	// Test HealthCheck
	if err := gateway.HealthCheck(context.Background()); err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}
}

// TestClaudeCodeGateway tests Claude Code API integration
// This test requires ANTHROPIC_API_KEY environment variable
func TestClaudeCodeGateway(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping Claude Code test: ANTHROPIC_API_KEY not set")
	}

	gateway := agent.NewClaudeCodeGateway(apiKey)

	// Test GetCapability
	cap := gateway.GetCapability()
	if cap.AgentType != "claude-code" {
		t.Errorf("Capability AgentType = %s, want claude-code", cap.AgentType)
	}

	if !cap.SupportsCodeGeneration {
		t.Error("Should support code generation")
	}

	// Test HealthCheck
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := gateway.HealthCheck(ctx); err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}

	// Test Execute (simple prompt)
	req := output.AgentRequest{
		Prompt:      "Say 'Hello' in one word",
		MaxTokens:   10,
		Temperature: 0.0,
		Timeout:     30 * time.Second,
	}

	resp, err := gateway.Execute(ctx, req)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if resp.AgentType != "claude-code" {
		t.Errorf("AgentType = %s, want claude-code", resp.AgentType)
	}

	if resp.Output == "" {
		t.Error("Output should not be empty")
	}

	if resp.TokensUsed == 0 {
		t.Error("TokensUsed should be > 0")
	}

	t.Logf("Claude response: %s (tokens: %d, duration: %v)", resp.Output, resp.TokensUsed, resp.Duration)
}

func TestNewAgentGateway(t *testing.T) {
	tests := []struct {
		name      string
		agentType string
		wantErr   bool
		setup     func()
		cleanup   func()
	}{
		{
			name:      "gemini-cli",
			agentType: "gemini-cli",
			wantErr:   false,
		},
		{
			name:      "codex",
			agentType: "codex",
			wantErr:   false,
		},
		{
			name:      "unknown agent",
			agentType: "unknown",
			wantErr:   true,
		},
		{
			name:      "claude-code without API key",
			agentType: "claude-code",
			wantErr:   true,
			setup: func() {
				os.Unsetenv("ANTHROPIC_API_KEY")
			},
		},
		{
			name:      "claude-code with API key",
			agentType: "claude-code",
			wantErr:   false,
			setup: func() {
				os.Setenv("ANTHROPIC_API_KEY", "test-key")
			},
			cleanup: func() {
				os.Unsetenv("ANTHROPIC_API_KEY")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.cleanup != nil {
				defer tt.cleanup()
			}

			gateway, err := agent.NewAgentGateway(tt.agentType)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewAgentGateway() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && gateway == nil {
				t.Error("Gateway should not be nil when no error expected")
			}
		})
	}
}
