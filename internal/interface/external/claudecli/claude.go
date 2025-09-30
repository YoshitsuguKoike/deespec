package claudecli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Runner struct {
	Bin     string
	Timeout time.Duration
}

// ClaudeResponse represents the JSON response from claude
type ClaudeResponse struct {
	Type       string  `json:"type"`
	Subtype    string  `json:"subtype"`
	IsError    bool    `json:"is_error"`
	DurationMs int     `json:"duration_ms"`
	Result     string  `json:"result"`
	SessionID  string  `json:"session_id"`
	TotalCost  float64 `json:"total_cost_usd"`
	UUID       string  `json:"uuid"`
}

// RunOptions contains options for Claude Code execution
type RunOptions struct {
	AllowedTools    []string // Tools to allow (e.g., "Read", "Edit", "Bash")
	DisallowedTools []string // Tools to disallow
}

// StreamEvent represents a single streaming event from Claude
type StreamEvent struct {
	Type      string                 `json:"type"`
	Timestamp string                 `json:"timestamp"`
	Content   string                 `json:"content,omitempty"`
	Tool      string                 `json:"tool,omitempty"`
	Args      map[string]interface{} `json:"args,omitempty"`
	Result    string                 `json:"result,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// StreamContext contains context for streaming operations
type StreamContext struct {
	SBIDir    string                                   // .deespec/specs/sbi/SBI-ID
	StepName  string                                   // implement, review, etc.
	Turn      int                                      // Turn number
	LogWriter func(format string, args ...interface{}) // Optional logger
}

func (r Runner) Run(ctx context.Context, prompt string, extraArgs ...string) (string, error) {
	return r.RunWithOptions(ctx, prompt, nil, extraArgs...)
}

func (r Runner) RunWithOptions(ctx context.Context, prompt string, opts *RunOptions, extraArgs ...string) (string, error) {
	// JSON形式で出力を取得（構造化された結果）
	args := []string{"-p", "--output-format", "json"}

	// Add tool permissions if specified
	if opts != nil {
		if len(opts.AllowedTools) > 0 {
			args = append(args, "--allowed-tools", strings.Join(opts.AllowedTools, ","))
		}
		if len(opts.DisallowedTools) > 0 {
			args = append(args, "--disallowed-tools", strings.Join(opts.DisallowedTools, ","))
		}
	}

	args = append(args, extraArgs...) // 将来拡張用
	args = append(args, prompt)

	cctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, r.Bin, args...)
	out, err := cmd.CombinedOutput()

	// コマンド実行エラーの場合
	if err != nil {
		return "", fmt.Errorf("claude execution failed: %w (output: %s)", err, string(out))
	}

	// JSONをパース
	var response ClaudeResponse
	if err := json.Unmarshal(out, &response); err != nil {
		// JSON パースに失敗した場合は、生の出力を返す（後方互換性のため）
		return string(out), nil
	}

	// エラーレスポンスの場合
	if response.IsError {
		return "", fmt.Errorf("claude returned error: %s", response.Result)
	}

	// 正常レスポンスの場合、resultフィールドのみを返す
	return response.Result, nil
}

// RunWithStream runs Claude with streaming output to both logger and history file
func (r Runner) RunWithStream(ctx context.Context, prompt string, streamCtx *StreamContext, opts *RunOptions, extraArgs ...string) (string, error) {
	// Prepare histories directory
	historiesDir := filepath.Join(streamCtx.SBIDir, "histories")
	if err := os.MkdirAll(historiesDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create histories directory: %w", err)
	}

	// Create history file with consistent naming: workflow_step_N.jsonl
	historyFile := filepath.Join(historiesDir, fmt.Sprintf("workflow_step_%d.jsonl", streamCtx.Turn))
	file, err := os.Create(historyFile)
	if err != nil {
		return "", fmt.Errorf("failed to create history file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)

	// stream-json形式で出力を取得
	// Note: If Claude CLI doesn't support stream-json, try just json with verbose
	args := []string{"-p", "--verbose", "--output-format", "stream-json"}

	// Log the command for debugging
	if streamCtx.LogWriter != nil {
		streamCtx.LogWriter("[debug] Running command: %s %v", r.Bin, args)
	}

	// Add tool permissions if specified
	if opts != nil {
		if len(opts.AllowedTools) > 0 {
			args = append(args, "--allowed-tools", strings.Join(opts.AllowedTools, ","))
		}
		if len(opts.DisallowedTools) > 0 {
			args = append(args, "--disallowed-tools", strings.Join(opts.DisallowedTools, ","))
		}
	}

	args = append(args, extraArgs...)
	args = append(args, prompt)

	cctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, r.Bin, args...)

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	// Process stderr (errors/warnings) in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			if streamCtx.LogWriter != nil {
				streamCtx.LogWriter("[stderr] %s", scanner.Text())
			}
		}
	}()

	// Process stdout (streaming JSON events)
	var finalResult string
	var contentBuilder strings.Builder // Accumulate content chunks
	scanner := bufio.NewScanner(stdout)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Try to parse as JSON
		var event map[string]interface{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not JSON, log as raw line
			if streamCtx.LogWriter != nil {
				streamCtx.LogWriter("[raw:%d] %s", lineNum, line)
			}
			continue
		}

		// Create StreamEvent with timestamp
		streamEvent := StreamEvent{
			Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		}

		// Parse event type and content
		if eventType, ok := event["type"].(string); ok {
			streamEvent.Type = eventType

			switch eventType {
			case "content", "text":
				if content, ok := event["content"].(string); ok {
					streamEvent.Content = content
					// Accumulate content for final result
					contentBuilder.WriteString(content)
					if streamCtx.LogWriter != nil {
						if len(content) > 100 {
							streamCtx.LogWriter("[stream:content] %d chars received", len(content))
						} else {
							streamCtx.LogWriter("[stream:content] %s", content)
						}
					}
				}
				// Also check for "text" field
				if text, ok := event["text"].(string); ok {
					streamEvent.Content = text
					contentBuilder.WriteString(text)
					if streamCtx.LogWriter != nil {
						streamCtx.LogWriter("[stream:text] %s", text)
					}
				}

			case "tool_use":
				if tool, ok := event["tool"].(string); ok {
					streamEvent.Tool = tool
				}
				if args, ok := event["args"].(map[string]interface{}); ok {
					streamEvent.Args = args
				}
				if streamCtx.LogWriter != nil {
					streamCtx.LogWriter("[stream:tool] %s", streamEvent.Tool)
				}

			case "final", "result", "completion", "message_stop":
				// Try multiple fields that might contain the result
				if result, ok := event["result"].(string); ok {
					finalResult = result
					streamEvent.Result = result
				}
				// Also check for "content" field in final event
				if finalResult == "" {
					if content, ok := event["content"].(string); ok {
						finalResult = content
						streamEvent.Result = content
					}
				}
				// Also check for "text" field
				if finalResult == "" {
					if text, ok := event["text"].(string); ok {
						finalResult = text
						streamEvent.Result = text
					}
				}
				// If still no result, use accumulated content
				if finalResult == "" && contentBuilder.Len() > 0 {
					finalResult = contentBuilder.String()
					streamEvent.Result = finalResult
					if streamCtx.LogWriter != nil {
						streamCtx.LogWriter("[stream:final] Using accumulated content as result")
					}
				}
				if streamCtx.LogWriter != nil {
					streamCtx.LogWriter("[stream:final] Result length: %d", len(finalResult))
				}

			case "error":
				if errMsg, ok := event["error"].(string); ok {
					streamEvent.Error = errMsg
					if streamCtx.LogWriter != nil {
						streamCtx.LogWriter("[stream:error] %s", errMsg)
					}
				}

			default:
				// Store any unknown event type with its full data
				streamEvent.Metadata = event
				if streamCtx.LogWriter != nil {
					streamCtx.LogWriter("[stream:%s] %v", eventType, event)
				}
			}
		} else {
			// No type field, store as metadata
			streamEvent.Type = "unknown"
			streamEvent.Metadata = event
		}

		// Write event to JSONL file
		if err := encoder.Encode(streamEvent); err != nil {
			if streamCtx.LogWriter != nil {
				streamCtx.LogWriter("[error] Failed to write to history: %v", err)
			}
		}
	}

	// Check for scanner errors
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading stream: %w", err)
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		// If we got a final result, return it despite command error
		// (claude might exit with non-zero on some conditions)
		if finalResult != "" {
			if streamCtx.LogWriter != nil {
				streamCtx.LogWriter("[warning] Command exited with error but got result: %v", err)
			}
			return finalResult, nil
		}
		return "", fmt.Errorf("claude execution failed: %w", err)
	}

	// If no final result was captured, check accumulated content
	if finalResult == "" && contentBuilder.Len() > 0 {
		finalResult = contentBuilder.String()
		if streamCtx.LogWriter != nil {
			streamCtx.LogWriter("[info] Using accumulated content as final result (length: %d)", len(finalResult))
		}
	}

	// If still no result, fall back to standard mode
	if finalResult == "" {
		if streamCtx.LogWriter != nil {
			streamCtx.LogWriter("[warning] No final result captured from stream, falling back to standard mode")
		}
		// Fall back to non-streaming method but still save a simplified history
		result, err := r.Run(ctx, prompt, extraArgs...)
		if err == nil && result != "" {
			// Save the final result as a single event
			finalEvent := StreamEvent{
				Type:      "final",
				Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
				Result:    result,
			}
			encoder.Encode(finalEvent)
		}
		return result, err
	}

	if streamCtx.LogWriter != nil {
		streamCtx.LogWriter("[stream:complete] History saved to: %s", historyFile)
	}

	return finalResult, nil
}
