package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// ClaudeCodeGateway implements AgentGateway for Claude Code API
type ClaudeCodeGateway struct {
	apiKey     string
	apiURL     string
	httpClient *http.Client
	model      string
}

// NewClaudeCodeGateway creates a new Claude Code gateway
func NewClaudeCodeGateway(apiKey string) *ClaudeCodeGateway {
	return &ClaudeCodeGateway{
		apiKey: apiKey,
		apiURL: "https://api.anthropic.com/v1/messages",
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		model: "claude-3-5-sonnet-20241022",
	}
}

// Execute runs the Claude Code agent with given request
func (g *ClaudeCodeGateway) Execute(ctx context.Context, req output.AgentRequest) (*output.AgentResponse, error) {
	start := time.Now()

	// Build Claude API request
	claudeReq := ClaudeRequest{
		Model:       g.model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Messages: []Message{
			{
				Role:    "user",
				Content: req.Prompt,
			},
		},
	}

	// Call Claude API
	resp, err := g.callClaudeAPI(ctx, claudeReq)
	if err != nil {
		return nil, fmt.Errorf("Claude API call failed: %w", err)
	}

	// Extract text content from response
	var outputText string
	if len(resp.Content) > 0 {
		outputText = resp.Content[0].Text
	}

	// Build agent response
	return &output.AgentResponse{
		Output:     outputText,
		ExitCode:   0,
		Duration:   time.Since(start),
		TokensUsed: resp.Usage.InputTokens + resp.Usage.OutputTokens,
		AgentType:  "claude-code",
		Metadata: map[string]string{
			"model":         g.model,
			"stop_reason":   resp.StopReason,
			"input_tokens":  fmt.Sprintf("%d", resp.Usage.InputTokens),
			"output_tokens": fmt.Sprintf("%d", resp.Usage.OutputTokens),
		},
	}, nil
}

// GetCapability returns Claude Code's capabilities
func (g *ClaudeCodeGateway) GetCapability() output.AgentCapability {
	return output.AgentCapability{
		SupportsCodeGeneration: true,
		SupportsReview:         true,
		SupportsTest:           true,
		MaxPromptSize:          200000, // 200k tokens
		ConcurrentTasks:        5,
		AgentType:              "claude-code",
	}
}

// HealthCheck verifies if Claude API is accessible
func (g *ClaudeCodeGateway) HealthCheck(ctx context.Context) error {
	// Simple ping request
	req := ClaudeRequest{
		Model:     g.model,
		MaxTokens: 10,
		Messages: []Message{
			{Role: "user", Content: "ping"},
		},
	}

	_, err := g.callClaudeAPI(ctx, req)
	return err
}

// callClaudeAPI makes an HTTP request to Claude API
func (g *ClaudeCodeGateway) callClaudeAPI(ctx context.Context, req ClaudeRequest) (*ClaudeResponse, error) {
	// Marshal request body
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", g.apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", g.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	// Execute request
	httpResp, err := g.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer httpResp.Body.Close()

	// Parse response
	var claudeResp ClaudeResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&claudeResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Check for API errors
	if httpResp.StatusCode != http.StatusOK {
		if claudeResp.Error.Message != "" {
			return nil, fmt.Errorf("API error (%d): %s - %s", httpResp.StatusCode, claudeResp.Error.Type, claudeResp.Error.Message)
		}
		return nil, fmt.Errorf("API error: status %d", httpResp.StatusCode)
	}

	return &claudeResp, nil
}

// Claude API request/response types
type ClaudeRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	ID         string          `json:"id"`
	Type       string          `json:"type"`
	Role       string          `json:"role"`
	Content    []ContentBlock  `json:"content"`
	Model      string          `json:"model"`
	StopReason string          `json:"stop_reason"`
	Usage      Usage           `json:"usage"`
	Error      ClaudeErrorResp `json:"error,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ClaudeErrorResp struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}
