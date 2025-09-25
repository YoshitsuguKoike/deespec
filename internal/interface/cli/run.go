package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/infra/config"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
	"github.com/YoshitsuguKoike/deespec/internal/interface/external/claudecli"
)

func parseDecision(s string) string {
	re := regexp.MustCompile(`(?mi)^\s*DECISION:\s*(OK|NEEDS_CHANGES)\s*$`)
	m := re.FindStringSubmatch(s)
	if len(m) == 2 {
		return strings.ToUpper(strings.TrimSpace(m[1]))
	}
	return "NEEDS_CHANGES"
}

func buildPlanPrompt(st *State) string {
	todo := st.Inputs["todo"]
	if todo == "" {
		todo = "タスクが未定義"
	}
	return "次のTODOを200字で計画し、手順を列挙せよ。\nTODO: " + todo + "\n"
}

func buildImplementPrompt(st *State) string {
	return "計画に基づき、実装の差分案を要点と一緒に提示せよ。\n"
}

func buildTestPrompt(st *State) string {
	return "変更に対する簡易テスト手順と想定出力ログを提示せよ。\n"
}

func buildReviewPrompt(st *State) string {
	return "以下をレビューし、最後に 'DECISION: OK' もしくは 'DECISION: NEEDS_CHANGES' を1行で出力せよ。\n- 計画/差分/テスト（要約可）\n"
}

func newRunCmd() *cobra.Command {
	var once bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !once {
				return fmt.Errorf("use --once for now (single-step mode)")
			}
			return runOnce()
		},
	}
	cmd.Flags().BoolVar(&once, "once", false, "Advance exactly one step")
	return cmd
}

// generateReviewNote creates a review_note.md file for SBI-001
func generateReviewNote(output string, turn int, decision string, turnDir string) (string, error) {
	// TODO(human) - implement review note generation
	// Extract summary from agent output and create review note with DECISION

	// Create a brief summary of the review
	lines := strings.Split(output, "\n")
	summary := "## Review Summary\n\n"

	// Take first few non-empty lines as summary (up to 5 lines)
	lineCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && lineCount < 5 {
			summary += "- " + trimmed + "\n"
			lineCount++
		}
	}

	// Add turn information
	summary += fmt.Sprintf("\n## Turn Information\n\n")
	summary += fmt.Sprintf("- Turn: %d\n", turn)
	summary += fmt.Sprintf("- Timestamp: %s\n", time.Now().UTC().Format(time.RFC3339))

	// Determine DECISION based on the decision parameter
	finalDecision := "NEEDS_CHANGES"
	if decision == "OK" || strings.Contains(strings.ToUpper(output), "APPROVED") ||
	   strings.Contains(strings.ToUpper(output), "LOOKS GOOD") {
		finalDecision = "OK"
	}

	// Add DECISION as the last line
	summary += fmt.Sprintf("\nDECISION: %s", finalDecision)

	// Write the review note
	noteFile := filepath.Join(turnDir, "review_note.md")
	if err := os.WriteFile(noteFile, []byte(summary), 0o644); err != nil {
		return "", err
	}

	return noteFile, nil
}

func runOnce() error {
	startTime := time.Now()

	// 1) ロック
	release, err := fs.AcquireLock(".deespec/var/state.lock")
	if err != nil {
		return err
	}
	defer release()

	// 2) 読み込み
	st, err := loadState(".deespec/var/state.json")
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	prevV := st.Version

	// 設定読み込みとエージェント作成
	cfg := config.Load()
	agent := claudecli.Runner{Bin: cfg.AgentBin, Timeout: cfg.Timeout}

	next := st.Current
	output := "# no-op\n"
	decision := "OK"
	errorMsg := ""

	// 3) ステップ別処理
	switch st.Current {
	case "plan":
		prompt := buildPlanPrompt(st)
		result, err := agent.Run(context.Background(), prompt)
		if err != nil {
			output = fmt.Sprintf("# Plan failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
			errorMsg = err.Error()
		} else {
			output = result
		}
		next = nextStep("plan", "OK")

	case "implement":
		prompt := buildImplementPrompt(st)
		result, err := agent.Run(context.Background(), prompt)
		if err != nil {
			output = fmt.Sprintf("# Implement failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
			errorMsg = err.Error()
		} else {
			output = result
		}
		next = nextStep("implement", "OK")

	case "test":
		prompt := buildTestPrompt(st)
		result, err := agent.Run(context.Background(), prompt)
		if err != nil {
			output = fmt.Sprintf("# Test failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
			errorMsg = err.Error()
		} else {
			output = result
		}
		next = nextStep("test", "OK")

	case "review":
		prompt := buildReviewPrompt(st)
		result, err := agent.Run(context.Background(), prompt)
		if err != nil {
			output = fmt.Sprintf("# Review failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
			decision = "NEEDS_CHANGES"
			errorMsg = err.Error()
		} else {
			output = result
			decision = parseDecision(result)
		}
		next = nextStep("review", decision)

	case "done":
		output = fmt.Sprintf("# Workflow completed at %s\n", time.Now().Format(time.RFC3339))
		next = "done"

	default:
		// 不明なステップの場合はplanに戻る
		next = "plan"
	}

	// 4) 現在のターンを固定（これが成果物とジャーナルで使用される）
	currentTurn := st.Turn

	// 5) 成果物出力
	turnDir := filepath.Join(st.ArtifactsDir, fmt.Sprintf("turn%d", currentTurn))
	if err := os.MkdirAll(turnDir, 0o755); err != nil {
		return err
	}

	// 出力ファイル名を決定（現在のステップではなく次のステップ名を使う既存ロジックを保持）
	stepName := next
	if st.Current != "done" {
		stepName = next
	} else {
		stepName = st.Current
	}

	outFile := filepath.Join(turnDir, fmt.Sprintf("%s.md", stepName))
	if err := os.WriteFile(outFile, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write artifact: %w", err)
	}

	if st.LastArtifacts == nil {
		st.LastArtifacts = map[string]string{}
	}
	st.LastArtifacts[stepName] = outFile

	// 6) review_note.md生成（SBI-001: reviewステップ完了時）
	artifacts := []string{outFile}
	if st.Current == "review" {
		// TODO(human) - implement generateReviewNote function
		if noteFile, err := generateReviewNote(output, currentTurn, decision, turnDir); err == nil && noteFile != "" {
			artifacts = append(artifacts, noteFile)
		}
	}

	// 7) ジャーナル追記（currentTurnを使用）
	elapsedMs := int(time.Since(startTime).Milliseconds())

	journalRec := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       currentTurn,  // 固定されたターン番号を使用
		"step":       next,         // 次のステップを記録
		"decision":   "",
		"elapsed_ms": elapsedMs,
		"error":      errorMsg,
		"artifacts":  artifacts,
	}

	// decision は review の時のみセット（次がdoneまたはimplementの場合）
	if st.Current == "review" {
		journalRec["decision"] = decision
	}

	// 正規化してから書き込み
	if err := app.AppendNormalizedJournal(journalRec); err != nil {
		// ジャーナル書き込みエラーは無視（ワークフローは継続）
		fmt.Fprintf(os.Stderr, "Warning: failed to write journal: %v\n", err)
	}

	// 7) ターンとステップの更新（ジャーナル記録後）
	if st.Current == "review" && next == "implement" {
		// ブーメラン時はturn据え置き
	} else if st.Current != "done" && next != st.Current {
		st.Turn++
	}
	st.Current = next

	// 8) health.json 更新（エラーに関わらず更新）
	healthOk := errorMsg == ""
	if err := app.WriteHealth(".deespec/var/health.json", currentTurn, next, healthOk, errorMsg); err != nil {
		// health.json書き込みエラーも無視（ワークフローは継続）
		fmt.Fprintf(os.Stderr, "Warning: failed to write .deespec/var/health.json: %v\n", err)
	}

	// 9) 保存（CAS + atomic）
	return saveStateCAS(".deespec/var/state.json", st, prevV)
}