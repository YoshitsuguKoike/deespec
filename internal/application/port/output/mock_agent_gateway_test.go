package output_test

import (
	"context"
	"fmt"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// MockAgentGateway is a mock implementation of AgentGateway for testing
type MockAgentGateway struct {
	ExecuteFunc      func(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error)
	HealthCheckFunc  func(ctx context.Context) error
	capability       output.AgentCapability
	executionHistory []output.AgentRequest
}

// NewMockAgentGateway creates a new mock agent gateway
func NewMockAgentGateway(agentType string) *MockAgentGateway {
	return &MockAgentGateway{
		capability: output.AgentCapability{
			SupportsCodeGeneration: true,
			SupportsReview:         true,
			SupportsTest:           true,
			MaxPromptSize:          200000,
			ConcurrentTasks:        5,
			AgentType:              agentType,
		},
		executionHistory: []output.AgentRequest{},
	}
}

// Execute executes the agent (mock implementation)
func (m *MockAgentGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
	m.executionHistory = append(m.executionHistory, req)

	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, req)
	}

	// Default mock response
	return &output.AgentResponse{
		Output:     fmt.Sprintf("Mock response for prompt: %s", req.Prompt[:min(50, len(req.Prompt))]),
		ExitCode:   0,
		Duration:   100 * time.Millisecond,
		TokensUsed: 100,
		AgentType:  m.capability.AgentType,
		Metadata:   map[string]string{"mock": "true"},
	}, nil
}

// GetCapability returns the agent capability
func (m *MockAgentGateway) GetCapability() output.AgentCapability {
	return m.capability
}

// HealthCheck performs a health check
func (m *MockAgentGateway) HealthCheck(ctx context.Context) error {
	if m.HealthCheckFunc != nil {
		return m.HealthCheckFunc(ctx)
	}
	return nil
}

// GetExecutionHistory returns the history of execute calls
func (m *MockAgentGateway) GetExecutionHistory() []output.AgentRequest {
	return m.executionHistory
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
