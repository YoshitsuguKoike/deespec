package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
	"github.com/YoshitsuguKoike/deespec/internal/interface/external/claudecli"
)

// ClaudeCodeCLIGateway implements AgentGateway using Claude Code CLI
// This executes `claude --dangerously-skip-permissions -p "prompt"` directly
type ClaudeCodeCLIGateway struct {
	runner     *claudecli.Runner
	workingDir string // Working directory for claude execution
}

// NewClaudeCodeCLIGateway creates a new Claude Code CLI gateway
func NewClaudeCodeCLIGateway() *ClaudeCodeCLIGateway {
	// Get current working directory
	wd, err := os.Getwd()
	if err != nil {
		wd = "."
	}

	return &ClaudeCodeCLIGateway{
		runner: &claudecli.Runner{
			Bin:     "claude",
			Timeout: 10 * time.Minute,
		},
		workingDir: wd,
	}
}

// Execute runs Claude Code CLI with the given request
func (g *ClaudeCodeCLIGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
	start := time.Now()

	// Execute claude CLI command
	result, err := g.runner.Run(ctx, req.Prompt)
	if err != nil {
		return nil, fmt.Errorf("claude CLI execution failed: %w", err)
	}

	// Build agent response
	return &output.AgentResponse{
		Output:     result,
		ExitCode:   0,
		Duration:   time.Since(start),
		TokensUsed: 0, // CLI doesn't provide token count
		AgentType:  "claude-code-cli",
		Metadata: map[string]string{
			"working_dir": g.workingDir,
			"cli_version": "latest", // Could be enhanced to get actual version
		},
	}, nil
}

// ExecuteWithArtifact runs Claude Code CLI and instructs it to create an artifact file
// This method builds a prompt that asks Claude to write output to a specific file
func (g *ClaudeCodeCLIGateway) ExecuteWithArtifact(ctx context.Context, req output.AgentRequest, artifactPath string) (*output.AgentResponse, error) {
	// Ensure artifact directory exists
	artifactDir := filepath.Dir(artifactPath)
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifact directory: %w", err)
	}

	// Build enhanced prompt that instructs Claude to write to the artifact file
	enhancedPrompt := fmt.Sprintf(`%s

IMPORTANT: Please write your complete response to the file: %s

Use the Write tool to create this file with your full analysis, implementation, or review.
Make sure to include all relevant details, code changes, and explanations in the file.
`, req.Prompt, artifactPath)

	// Execute with enhanced prompt
	start := time.Now()
	result, err := g.runner.Run(ctx, enhancedPrompt)
	if err != nil {
		return nil, fmt.Errorf("claude CLI execution failed: %w", err)
	}

	// Check if artifact file was created
	if _, err := os.Stat(artifactPath); os.IsNotExist(err) {
		// Artifact wasn't created, write the result ourselves as fallback
		if err := os.WriteFile(artifactPath, []byte(result), 0644); err != nil {
			return nil, fmt.Errorf("failed to write artifact file: %w", err)
		}
	}

	// Build agent response
	return &output.AgentResponse{
		Output:     result,
		ExitCode:   0,
		Duration:   time.Since(start),
		TokensUsed: 0,
		AgentType:  "claude-code-cli",
		Metadata: map[string]string{
			"working_dir":   g.workingDir,
			"artifact_path": artifactPath,
			"artifact_size": fmt.Sprintf("%d", getFileSize(artifactPath)),
		},
	}, nil
}

// GetCapability returns Claude Code CLI's capabilities
func (g *ClaudeCodeCLIGateway) GetCapability() output.AgentCapability {
	return output.AgentCapability{
		SupportsCodeGeneration: true,
		SupportsReview:         true,
		SupportsTest:           true,
		MaxPromptSize:          200000, // 200k tokens
		ConcurrentTasks:        1,      // CLI runs one at a time
		AgentType:              "claude-code-cli",
	}
}

// HealthCheck verifies if claude CLI is available
func (g *ClaudeCodeCLIGateway) HealthCheck(ctx context.Context) error {
	// Simple test execution
	testReq := output.AgentRequest{
		Prompt:  "ping",
		Timeout: 10 * time.Second,
	}

	_, err := g.Execute(ctx, testReq)
	if err != nil {
		return fmt.Errorf("claude CLI health check failed: %w", err)
	}

	return nil
}

// Helper function to get file size
func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}
