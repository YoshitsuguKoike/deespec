package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// GeminiMockGateway is a mock implementation of AgentGateway for Gemini CLI
// This will be replaced with actual Gemini CLI integration in the future
type GeminiMockGateway struct{}

// NewGeminiMockGateway creates a new Gemini mock gateway
func NewGeminiMockGateway() *GeminiMockGateway {
	return &GeminiMockGateway{}
}

// Execute simulates Gemini CLI execution
func (g *GeminiMockGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Generate mock response
	promptPreview := req.Prompt
	if len(promptPreview) > 50 {
		promptPreview = promptPreview[:50] + "..."
	}

	mockOutput := fmt.Sprintf("[Gemini Mock] Response for: %s", promptPreview)

	return &output.AgentResponse{
		Output:     mockOutput,
		ExitCode:   0,
		Duration:   100 * time.Millisecond,
		TokensUsed: len(req.Prompt) / 4, // Rough estimate
		AgentType:  "gemini-cli",
		Metadata: map[string]string{
			"mock": "true",
			"note": "Gemini CLI integration pending",
		},
	}, nil
}

// GetCapability returns Gemini's mock capabilities
func (g *GeminiMockGateway) GetCapability() output.AgentCapability {
	return output.AgentCapability{
		SupportsCodeGeneration: true,
		SupportsReview:         true,
		SupportsTest:           true,
		MaxPromptSize:          32000,
		ConcurrentTasks:        3,
		AgentType:              "gemini-cli",
	}
}

// HealthCheck always returns success for mock
func (g *GeminiMockGateway) HealthCheck(ctx context.Context) error {
	return nil
}
