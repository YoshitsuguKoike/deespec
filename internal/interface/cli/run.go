package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

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

func runOnce() error {
	// 1) ロック
	release, err := fs.AcquireLock("state.lock")
	if err != nil {
		return err
	}
	defer release()

	// 2) 読み込み
	st, err := loadState("state.json")
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

	// 3) ステップ別処理
	switch st.Current {
	case "plan":
		prompt := buildPlanPrompt(st)
		result, err := agent.Run(context.Background(), prompt)
		if err != nil {
			output = fmt.Sprintf("# Plan failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
		} else {
			output = result
		}
		next = nextStep("plan", "OK")

	case "implement":
		prompt := buildImplementPrompt(st)
		result, err := agent.Run(context.Background(), prompt)
		if err != nil {
			output = fmt.Sprintf("# Implement failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
		} else {
			output = result
		}
		next = nextStep("implement", "OK")

	case "test":
		prompt := buildTestPrompt(st)
		result, err := agent.Run(context.Background(), prompt)
		if err != nil {
			output = fmt.Sprintf("# Test failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
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

	// 4) 成果物出力
	turnDir := filepath.Join(st.ArtifactsDir, fmt.Sprintf("turn%d", st.Turn))
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

	// 5) 遷移・ターン更新
	if st.Current == "review" && next == "implement" {
		// ブーメラン時はturn据え置き
	} else if st.Current != "done" && next != st.Current {
		st.Turn++
	}
	st.Current = next

	// 6) ジャーナル追記
	journalRec := map[string]any{
		"ts":        time.Now().UTC().Format(time.RFC3339),
		"turn":      st.Turn,
		"step":      st.Current,
		"artifacts": []string{outFile},
	}
	if st.Current == "review" || st.Current == "done" {
		journalRec["decision"] = decision
	}
	appendJournal(journalRec)

	// 7) 保存（CAS + atomic）
	return saveStateCAS("state.json", st, prevV)
}

func appendJournal(rec map[string]any) {
	f, err := os.OpenFile("journal.ndjson", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	b, _ := json.Marshal(rec)
	_, _ = f.Write(append(b, '\n'))
}