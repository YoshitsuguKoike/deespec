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
	return "計画に基づき、実装の差分案を要点と一緒に提示せよ。\n最後に「## Implementation Note」セクションを追加し、実装の要点を簡潔にまとめよ。\n"
}

func buildTestPrompt(st *State) string {
	return "変更に対する簡易テスト手順と想定出力ログを提示せよ。\n"
}

func buildReviewPrompt(st *State) string {
	return "以下をレビューし、最後に 'DECISION: OK' もしくは 'DECISION: NEEDS_CHANGES' を1行で出力せよ。\n- 計画/差分/テスト（要約可）\n\n最後に「## Review Note」セクションを追加し、レビューの要点と判断理由を記載せよ。\n"
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
	startTime := time.Now()

	// Get paths
	paths := app.GetPaths()

	// 1) ロック
	release, err := fs.AcquireLock(paths.StateLock)
	if err != nil {
		return err
	}
	defer release()

	// 2) 読み込み
	st, err := loadState(paths.State)
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	prevV := st.Version

	// 3) WIP判定とピック/再開
	if st.CurrentTaskID == "" {
		// No WIP - try to pick next task
		cfg := PickConfig{
			JournalPath: paths.Journal,
		}

		picked, reason, err := PickNextTask(cfg)
		if err != nil {
			return fmt.Errorf("failed to pick task: %w", err)
		}

		if picked == nil {
			fmt.Fprintf(os.Stderr, "INFO: %s\n", reason)
			// No task to pick - just update health and exit
			if err := app.WriteHealth(paths.Health, st.Turn, st.Current, true, ""); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to write %s: %v\n", paths.Health, err)
			}
			return nil
		}

		// Record pick in journal
		if err := RecordPickInJournal(picked, st.Turn, paths.Journal); err != nil {
			return fmt.Errorf("failed to record pick: %w", err)
		}

		// Update state for new task
		st.CurrentTaskID = picked.ID
		st.Current = "implement" // Start with implement after plan
		st.Inputs = map[string]string{
			"todo": fmt.Sprintf("Implement task %s: %s", picked.ID, picked.Title),
		}

		fmt.Fprintf(os.Stderr, "INFO: picked task %s: %s\n", picked.ID, reason)
	} else {
		// WIP exists - try to resume
		resumed, reason, err := ResumeIfInProgress(st, paths.Journal)
		if err != nil {
			return fmt.Errorf("failed to resume: %w", err)
		}

		if resumed {
			fmt.Fprintf(os.Stderr, "INFO: %s\n", reason)
		}
	}

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
			// Extract and append implementation note to rolling file
			noteBody := ExtractNoteBody(result, "implement")
			if noteErr := AppendNote("implement", "PENDING", noteBody, st.Turn, time.Now()); noteErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to append impl_note: %v\n", noteErr)
			}
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
			// Extract and append review note to rolling file
			noteBody := ExtractNoteBody(result, "review")
			if noteErr := AppendNote("review", decision, noteBody, st.Turn, time.Now()); noteErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to append review_note: %v\n", noteErr)
			}
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

	// 6) Build artifacts list with rolling note paths
	artifacts := []interface{}{outFile}

	// Add rolling note path for implement step
	if st.Current == "implement" {
		artifacts = append(artifacts, map[string]interface{}{
			"type": "impl_note_rollup",
			"path": ".deespec/var/artifacts/impl_note.md",
		})
	}

	// Add rolling note path for review step
	if st.Current == "review" {
		artifacts = append(artifacts, map[string]interface{}{
			"type": "review_note_rollup",
			"path": ".deespec/var/artifacts/review_note.md",
		})
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

	// Clear WIP when task is done
	if next == "done" {
		st.CurrentTaskID = ""
	}

	// 8) health.json 更新（エラーに関わらず更新）
	healthOk := errorMsg == ""
	if err := app.WriteHealth(paths.Health, currentTurn, next, healthOk, errorMsg); err != nil {
		// health.json書き込みエラーも無視（ワークフローは継続）
		fmt.Fprintf(os.Stderr, "Warning: failed to write %s: %v\n", paths.Health, err)
	}

	// 9) 保存（CAS + atomic）
	return saveStateCAS(paths.State, st, prevV)
}