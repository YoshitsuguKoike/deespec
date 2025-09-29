package claudecli

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
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
