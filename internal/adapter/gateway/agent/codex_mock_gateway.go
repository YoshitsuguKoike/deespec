package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// CodexMockGateway is a mock implementation of AgentGateway for OpenAI Codex
// This will be replaced with actual Codex integration in the future
type CodexMockGateway struct{}

// NewCodexMockGateway creates a new Codex mock gateway
func NewCodexMockGateway() *CodexMockGateway {
	return &CodexMockGateway{}
}

// Execute simulates Codex execution
func (g *CodexMockGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
	// Simulate processing time
	time.Sleep(150 * time.Millisecond)

	// Generate mock response
	promptPreview := req.Prompt
	if len(promptPreview) > 50 {
		promptPreview = promptPreview[:50] + "..."
	}

	mockOutput := fmt.Sprintf("[Codex Mock] Response for: %s", promptPreview)

	return &output.AgentResponse{
		Output:     mockOutput,
		ExitCode:   0,
		Duration:   150 * time.Millisecond,
		TokensUsed: len(req.Prompt) / 4, // Rough estimate
		AgentType:  "codex",
		Metadata: map[string]string{
			"mock": "true",
			"note": "Codex integration pending",
		},
	}, nil
}

// GetCapability returns Codex's mock capabilities
func (g *CodexMockGateway) GetCapability() output.AgentCapability {
	return output.AgentCapability{
		SupportsCodeGeneration: true,
		SupportsReview:         false,
		SupportsTest:           true,
		MaxPromptSize:          8000,
		ConcurrentTasks:        5,
		AgentType:              "codex",
	}
}

// HealthCheck always returns success for mock
func (g *CodexMockGateway) HealthCheck(ctx context.Context) error {
	return nil
}
