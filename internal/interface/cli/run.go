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

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/infra/fs"
	"github.com/YoshitsuguKoike/deespec/internal/interface/external/claudecli"
)

// summarizeText returns first N lines and total line count of text
func summarizeText(text string, maxLines int) string {
	lines := strings.Split(text, "\n")
	total := len(lines)

	if total <= maxLines {
		return text
	}

	preview := strings.Join(lines[:maxLines], "\n")
	return fmt.Sprintf("%s\n... (total %d lines)", preview, total)
}

// logClaudeInteraction logs the prompt sent to Claude and the response with timing
func logClaudeInteraction(prompt, result string, err error, startTime, endTime time.Time) {
	elapsed := endTime.Sub(startTime)

	// Log start
	Info("┌─ Claude Code Execution ─────────────────────────────────\n")
	Info("│ Start: %s\n", startTime.Format("15:04:05.000"))
	Info("│ Prompt: %d chars, %d lines\n", len(prompt), strings.Count(prompt, "\n")+1)

	// Show first few lines of prompt for context
	lines := strings.Split(prompt, "\n")
	if len(lines) > 0 {
		Info("│ Type: %s\n", strings.TrimPrefix(lines[0], "# "))
	}

	// Log end and result
	Info("│ End: %s (Duration: %.1fs)\n", endTime.Format("15:04:05.000"), elapsed.Seconds())

	if err != nil {
		Info("│ Status: ERROR - %v\n", err)
		Info("└──────────────────────────────────────────────────────────\n")
		return
	}

	// Analyze result
	resultLines := strings.Split(result, "\n")
	Info("│ Response: %d chars, %d lines\n", len(result), len(resultLines))

	// Log warnings for suspicious responses
	if len(result) == 0 {
		Warn("│ Warning: Empty response from AI\n")
	} else if len(result) < 100 {
		Warn("│ Warning: Unusually short response (%d chars)\n", len(result))
		Warn("│ Full content: %s\n", result)
	}

	// Always show AI response content (not just in debug mode)
	Info("┌─ AI Response Content ─────────────────────────────────────\n")
	// Show first 50 lines or entire response if shorter
	maxLines := 50
	if len(resultLines) <= maxLines {
		// Show entire response if it's short enough
		for i, line := range resultLines {
			Info("│ %4d: %s\n", i+1, line)
		}
	} else {
		// Show first and last parts for long responses
		for i := 0; i < 25; i++ {
			Info("│ %4d: %s\n", i+1, resultLines[i])
		}
		Info("│ ... (%d lines omitted) ...\n", len(resultLines)-50)
		for i := len(resultLines) - 25; i < len(resultLines); i++ {
			Info("│ %4d: %s\n", i+1, resultLines[i])
		}
	}
	Info("└──────────────────────────────────────────────────────────\n")

	// Try to find key sections in the result
	var decision string
	var noteFound bool
	for _, line := range resultLines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "DECISION:") {
			decision = strings.TrimSpace(strings.Split(trimmed, "DECISION:")[1])
		}
		if strings.Contains(trimmed, "Implementation Note") ||
			strings.Contains(trimmed, "Review Note") ||
			strings.Contains(trimmed, "Test Note") {
			noteFound = true
		}
	}

	if decision != "" {
		Info("│ Decision: %s\n", decision)
	}
	if noteFound {
		Info("│ Note: Found in response\n")
	}
	Info("└──────────────────────────────────────────────────────────\n")
}

func parseDecision(s string) string {
	// Updated to match domain model decisions: SUCCEEDED, NEEDS_CHANGES, FAILED
	// Allow leading/trailing characters like asterisks or other markers
	// Try multiple patterns to be more flexible
	patterns := []string{
		`(?mi)DECISION:\s*(SUCCEEDED|NEEDS_CHANGES|FAILED)`,       // Basic pattern
		`(?mi)\*+\s*DECISION:\s*(SUCCEEDED|NEEDS_CHANGES|FAILED)`, // With leading asterisks
		`(?mi)DECISION:\s*(SUCCEEDED|NEEDS_CHANGES|FAILED)\s*\*+`, // With trailing asterisks
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		m := re.FindStringSubmatch(s)
		if len(m) >= 2 {
			decision := strings.ToUpper(strings.TrimSpace(m[1]))
			Info("Decision extracted: %s (pattern: %s)\n", decision, pattern)
			return decision
		}
	}

	// Default to NEEDS_CHANGES if no valid decision found
	Info("No valid DECISION found in response, defaulting to NEEDS_CHANGES\n")
	return "NEEDS_CHANGES"
}

// getTaskDescription returns task description based on status and attempt
func getTaskDescription(st *State) string {
	switch st.Status {
	case "READY", "", "WIP":
		if st.Attempt == 1 {
			todo := st.Inputs["todo"]
			if todo == "" {
				todo = fmt.Sprintf("Implement SBI task %s", st.WIP)
			}
			return todo
		} else if st.Attempt == 2 {
			taskDesc := fmt.Sprintf("Second attempt for %s. Review feedback and implement improvements.", st.WIP)
			if reviewFile := st.LastArtifacts["review"]; reviewFile != "" {
				if content, err := os.ReadFile(reviewFile); err == nil {
					taskDesc = fmt.Sprintf("Second attempt based on review feedback:\n\n%s", string(content))
				}
			}
			return taskDesc
		} else {
			taskDesc := fmt.Sprintf("Third attempt for %s. Final chance to implement correctly.", st.WIP)
			if reviewFile := st.LastArtifacts["review"]; reviewFile != "" {
				if content, err := os.ReadFile(reviewFile); err == nil {
					taskDesc = fmt.Sprintf("Third attempt based on review feedback:\n\n%s", string(content))
				}
			}
			return taskDesc
		}

	case "REVIEW":
		implArtifact := ""
		if st.Attempt == 1 {
			implArtifact = filepath.Join(".deespec", "specs", "sbi", st.WIP, fmt.Sprintf("implement_%d.md", st.Turn))
		} else {
			implArtifact = filepath.Join(".deespec", "specs", "sbi", st.WIP, fmt.Sprintf("implement_attempt%d_%d.md", st.Attempt, st.Turn))
		}
		return fmt.Sprintf("Review the implementation at: %s", implArtifact)

	case "REVIEW&WIP":
		return fmt.Sprintf("Force implementation for %s after 3 failed attempts. As reviewer, implement the solution directly.", st.WIP)

	default:
		return fmt.Sprintf("Process task %s", st.WIP)
	}
}

// buildPromptByStatus creates appropriate prompt based on status and turn
func buildPromptByStatus(st *State) string {
	builder := ClaudeCodePromptBuilder{
		WorkDir: getCurrentWorkDir(),
		SBIDir:  filepath.Join(".deespec", "specs", "sbi", st.WIP),
		SBIID:   st.WIP,
		Turn:    st.Turn,
		Step:    determineStep(st.Status, st.Attempt),
	}

	// Determine task description based on status and attempt
	taskDesc := getTaskDescription(st)

	// Try to load external prompt first
	externalPrompt, err := builder.LoadExternalPrompt(st.Status, taskDesc)
	if err == nil {
		Info("Loaded external prompt from .deespec/prompts/ for status: %s\n", st.Status)
		return externalPrompt
	}

	// Fall back to hardcoded prompts
	Info("Using default prompt (external prompt not found: %v)\n", err)

	switch st.Status {
	case "READY", "":
		// Initial implementation (Turn 1, Step 2: implement_try)
		todo := st.Inputs["todo"]
		if todo == "" {
			todo = fmt.Sprintf("Implement SBI task %s", st.WIP)
		}
		return builder.BuildImplementPrompt(todo)

	case "WIP":
		// Implementation or re-implementation
		if st.Attempt == 1 {
			// First attempt (Step 2: implement_try)
			todo := st.Inputs["todo"]
			if todo == "" {
				todo = fmt.Sprintf("Implement SBI task %s", st.WIP)
			}
			return builder.BuildImplementPrompt(todo)
		} else if st.Attempt == 2 {
			// Second attempt (Step 4: implement_2nd_try)
			taskDesc := fmt.Sprintf("Second attempt for %s. Review feedback and implement improvements.", st.WIP)
			if reviewFile := st.LastArtifacts["review"]; reviewFile != "" {
				if content, err := os.ReadFile(reviewFile); err == nil {
					taskDesc = fmt.Sprintf("Second attempt based on review feedback:\n\n%s", string(content))
				}
			}
			return builder.BuildImplementPrompt(taskDesc)
		} else {
			// Third attempt (Step 6: implement_3rd_try)
			taskDesc := fmt.Sprintf("Third attempt for %s. Final chance to implement correctly.", st.WIP)
			if reviewFile := st.LastArtifacts["review"]; reviewFile != "" {
				if content, err := os.ReadFile(reviewFile); err == nil {
					taskDesc = fmt.Sprintf("Third attempt based on review feedback:\n\n%s", string(content))
				}
			}
			return builder.BuildImplementPrompt(taskDesc)
		}

	case "REVIEW":
		// Review implementation
		implArtifact := ""
		if st.Attempt == 1 {
			implArtifact = filepath.Join(".deespec", "specs", "sbi", st.WIP, fmt.Sprintf("implement_%d.md", st.Turn))
		} else {
			implArtifact = filepath.Join(".deespec", "specs", "sbi", st.WIP, fmt.Sprintf("implement_attempt%d_%d.md", st.Attempt, st.Turn))
		}
		// No test artifact since test step is removed
		return builder.BuildReviewPrompt(implArtifact, "")

	case "REVIEW&WIP":
		// Force termination - reviewer implements (Step 8: reviewer_force_implement)
		taskDesc := fmt.Sprintf("Force implementation for %s after 3 failed attempts. As reviewer, implement the solution directly.", st.WIP)
		return builder.BuildImplementPrompt(taskDesc)

	case "DONE":
		return "# Task completed\n\nThe SBI execution has been completed."

	default:
		return fmt.Sprintf("# Unknown status: %s\n\nCannot determine appropriate action.", st.Status)
	}
}

// determineStep maps status and attempt to execution step
func determineStep(status string, attempt int) string {
	switch status {
	case "READY", "":
		return "implement_try"
	case "WIP":
		if attempt == 1 {
			return "implement_try"
		} else if attempt == 2 {
			return "implement_2nd_try"
		} else {
			return "implement_3rd_try"
		}
	case "REVIEW":
		if attempt == 1 {
			return "first_review"
		} else if attempt == 2 {
			return "second_review"
		} else {
			return "third_review"
		}
	case "REVIEW&WIP":
		return "reviewer_force_implement"
	case "DONE":
		return "done"
	default:
		return "unknown"
	}
}

// Legacy functions kept for compatibility
func buildImplementPrompt(st *State) string {
	return buildPromptByStatus(st)
}

func buildReviewPrompt(st *State) string {
	return buildPromptByStatus(st)
}

func getCurrentWorkDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

func newRunCmd() *cobra.Command {
	var once bool
	var autoFB bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run workflow",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !once {
				return fmt.Errorf("use --once for now (single-step mode)")
			}
			// Check config for auto-fb (config takes precedence over flag)
			if globalConfig != nil && globalConfig.AutoFB() {
				autoFB = true
			}
			return runOnce(autoFB)
		},
	}
	cmd.Flags().BoolVar(&once, "once", false, "Advance exactly one step")
	cmd.Flags().BoolVar(&autoFB, "auto-fb", false, "Automatically register FB-SBI drafts")
	return cmd
}

// Helper function to run agent and save history
func runAgent(agent claudecli.Runner, prompt string, sbiDir string, stepName string, turn int, enableStream bool) (string, error) {
	// Create histories directory
	historiesDir := filepath.Join(sbiDir, "histories")
	if err := os.MkdirAll(historiesDir, 0755); err != nil {
		Warn("Failed to create histories directory: %v\n", err)
	}

	// Create history file with consistent naming: workflow_step_N.jsonl
	historyFile := filepath.Join(historiesDir, fmt.Sprintf("workflow_step_%d.jsonl", turn))

	if enableStream {
		// Try streaming mode first
		streamCtx := &claudecli.StreamContext{
			SBIDir:   sbiDir,
			StepName: stepName,
			Turn:     turn,
			LogWriter: func(format string, args ...interface{}) {
				// Log stream events with prefix for clarity
				// Always log to debug
				Debug("[STREAM] "+format, args...)
				// Also log important events to info
				if strings.Contains(format, "error") || strings.Contains(format, "warning") || strings.Contains(format, "final") {
					Info("[STREAM] "+format, args...)
				}
			},
		}
		result, err := agent.RunWithStream(context.Background(), prompt, streamCtx, nil)
		if err == nil {
			// Check if result seems valid
			if len(result) == 0 {
				Warn("Streaming returned empty result, falling back to regular mode\n")
			} else {
				Info("Streaming successful, result length: %d chars\n", len(result))
				// Also save raw response to a debug file for inspection
				resultsDir := filepath.Join(sbiDir, "results")
				if err := os.MkdirAll(resultsDir, 0755); err == nil {
					if debugFile := filepath.Join(resultsDir, fmt.Sprintf("raw_response_%s_%d.txt", stepName, turn)); len(result) > 0 {
						if err := os.WriteFile(debugFile, []byte(result), 0644); err == nil {
							Debug("Raw response saved to: %s\n", debugFile)
						}
					}
				}
				return result, nil
			}
		} else {
			// If streaming fails, fall back to regular mode
			Warn("Streaming mode failed, falling back to regular mode: %v\n", err)
		}
	}

	// Use regular mode and save result as history
	startTime := time.Now()
	result, err := agent.Run(context.Background(), prompt)
	endTime := time.Now()

	// Save raw response to a debug file for inspection
	resultsDir := filepath.Join(sbiDir, "results")
	if err := os.MkdirAll(resultsDir, 0755); err == nil {
		if debugFile := filepath.Join(resultsDir, fmt.Sprintf("raw_response_%s_%d.txt", stepName, turn)); err == nil && len(result) > 0 {
			if werr := os.WriteFile(debugFile, []byte(result), 0644); werr == nil {
				Debug("Raw response saved to: %s\n", debugFile)
			}
		}
	}

	// Save history even in regular mode
	if file, ferr := os.Create(historyFile); ferr == nil {
		defer file.Close()
		encoder := json.NewEncoder(file)

		// Write request event with step info
		encoder.Encode(map[string]interface{}{
			"type":      "request",
			"timestamp": startTime.UTC().Format(time.RFC3339Nano),
			"step":      stepName,
			"turn":      turn,
			"prompt":    prompt,
		})

		// Write response event
		if err != nil {
			encoder.Encode(map[string]interface{}{
				"type":      "error",
				"timestamp": endTime.UTC().Format(time.RFC3339Nano),
				"step":      stepName,
				"turn":      turn,
				"error":     err.Error(),
			})
		} else {
			encoder.Encode(map[string]interface{}{
				"type":        "response",
				"timestamp":   endTime.UTC().Format(time.RFC3339Nano),
				"step":        stepName,
				"turn":        turn,
				"result":      result,
				"duration_ms": endTime.Sub(startTime).Milliseconds(),
			})
		}
		Debug("History saved to: %s\n", historyFile)
	}

	return result, err
}

func runOnce(autoFB bool) error {
	startTime := time.Now()

	// Get paths using config
	paths := app.GetPathsWithConfig(globalConfig)

	// 1) 排他ロック（WIP=1 enforcement）
	releaseFsLock, err := fs.AcquireLock(paths.StateLock)
	if err != nil {
		return err
	}
	defer releaseFsLock()

	// 1.2) Run-level lock (parallel execution guard)
	runLockPath := paths.Var + "/runlock"
	releaseRunLock, acquired, err := AcquireLock(runLockPath, 10*time.Minute)
	if err != nil {
		return fmt.Errorf("failed to acquire run lock: %w", err)
	}

	if !acquired {
		// Another instance is running - this is normal, not an error
		Info("another instance active")

		// Update health even on no-op
		if err := app.WriteHealth(paths.Health, 0, "plan", true, ""); err != nil {
			Warn("failed to write %s: %v\n", paths.Health, err)
		}

		return nil // Exit 0 - not an error condition
	}
	defer func() {
		if releaseRunLock != nil {
			if err := releaseRunLock(); err != nil {
				Warn("failed to release run lock: %v\n", err)
			}
		}
	}()

	// 2) 読み込み
	st, err := loadState(paths.State)
	if err != nil {
		Error("failed to read state: %v", err)
		return fmt.Errorf("read state: %w", err)
	}
	prevV := st.Version

	// 3) Turn management: 1 run = 1 turn
	currentTurn := st.Turn + 1
	st.Turn = currentTurn

	// 3.5) Lease management - check for expired lease
	if st.WIP != "" && LeaseExpired(st) {
		Info("Lease expired for task %s, taking over\n", st.WIP)
		// Lease expired, we can take over the task
		// The task will be resumed in the next section
	}

	// Renew or set lease for current process
	if st.WIP != "" {
		if RenewLease(st, DefaultLeaseTTL) {
			Info("Renewed lease for task %s until %s\n", st.WIP, st.LeaseExpiresAt)
		}
	}

	// 3) WIP判定とピック/再開
	Info("Current state: WIP=%s, Current=%s, Turn=%d\n", st.WIP, st.Current, st.Turn)

	if st.WIP == "" {
		// No WIP - try to pick next task
		Info("No WIP found, attempting to pick next task...\n")
		cfg := PickConfig{
			JournalPath: paths.Journal,
		}

		picked, reason, err := PickNextTask(cfg)
		if err != nil {
			Error("failed to pick task: %v\n", err)
			return fmt.Errorf("failed to pick task: %w", err)
		}

		// Handle auto-fb if enabled and FB drafts were created
		if autoFB {
			if err := HandleAutoFBRegistration(paths.Journal, currentTurn); err != nil {
				Warn("Failed to auto-register FB drafts: %v\n", err)
			}
		}

		if picked == nil {
			Info("%s\n", reason)
			// No task to pick - update health and exit
			if err := app.WriteHealth(paths.Health, currentTurn, "plan", true, ""); err != nil {
				Warn("failed to write %s: %v\n", paths.Health, err)
			}
			return nil
		}

		// Record pick in journal
		if err := RecordPickInJournal(picked, currentTurn, paths.Journal); err != nil {
			Error("failed to record pick: %v\n", err)
			return fmt.Errorf("failed to record pick: %w", err)
		}

		// Update state for new task
		st.WIP = picked.ID
		st.Status = "READY"      // Start with READY status
		st.Current = "implement" // Legacy field
		st.Attempt = 1           // First attempt
		st.Decision = "PENDING"
		st.Inputs = map[string]string{
			"todo": fmt.Sprintf("Implement task %s: %s", picked.ID, picked.Title),
		}

		// Set lease for new task
		RenewLease(st, DefaultLeaseTTL)
		Info("picked task %s: %s (lease until %s)\n", picked.ID, reason, st.LeaseExpiresAt)
	} else {
		// WIP exists - try to resume
		Info("WIP exists: %s, attempting to resume from step: %s\n", st.WIP, st.Current)
		resumed, reason, err := ResumeIfInProgress(st, paths.Journal)
		if err != nil {
			Error("failed to resume: %v\n", err)
			return fmt.Errorf("failed to resume: %w", err)
		}

		if resumed {
			Info("Resume result: %s\n", reason)
		}
	}

	// 設定読み込みとエージェント作成
	// Use globalConfig if available, otherwise use defaults
	agentBin := "claude"
	timeout := 60 * time.Second
	if globalConfig != nil {
		agentBin = globalConfig.AgentBin()
		timeout = time.Duration(globalConfig.TimeoutSec()) * time.Second
	}

	// Always enable streaming to save histories
	// This provides audit trail and debugging capability
	enableStream := true

	agent := claudecli.Runner{Bin: agentBin, Timeout: timeout}

	// Initialize status if not set
	if st.Status == "" {
		if st.WIP != "" {
			st.Status = "READY"
			st.Attempt = 1
			st.Decision = "PENDING"
		}
	}

	output := "# no-op\n"
	decision := st.Decision
	if decision == "" {
		decision = "PENDING"
	}
	errorMsg := ""

	// 3) Status-based processing
	Info("Processing status: %s for SBI: %s (Turn: %d, Attempt: %d)\n", st.Status, st.WIP, st.Turn, st.Attempt)

	// Generate prompt based on current status
	prompt := buildPromptByStatus(st)
	Info("Generated prompt type: %s (length: %d chars)\n", determineStep(st.Status, st.Attempt), len(prompt))

	// SBIディレクトリパス: .deespec/specs/sbi/<SBI-ID>/
	sbiDir := filepath.Join(".deespec", "specs", "sbi", st.WIP)

	switch st.Status {
	case "READY", "WIP":
		// Implementation phase
		Info("Running implementation for attempt %d\n", st.Attempt)
		claudeStart := time.Now()
		result, err := runAgent(agent, prompt, sbiDir, "implement", currentTurn, enableStream)
		claudeEnd := time.Now()
		logClaudeInteraction(prompt, result, err, claudeStart, claudeEnd)
		if err != nil {
			output = fmt.Sprintf("# Implementation failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
			errorMsg = err.Error()
			decision = "NEEDS_CHANGES"
		} else {
			output = result
			// Extract and append implementation note
			noteBody := ExtractNoteBody(result, "implement")
			if noteErr := AppendNote("implement", "PENDING", noteBody, st.Turn, st.WIP, time.Now()); noteErr != nil {
				Warn("failed to append impl_note: %v\n", noteErr)
			}
			decision = "PENDING" // Will be determined in review
		}

	case "REVIEW":
		// Review phase
		Info("Running review for attempt %d\n", st.Attempt)
		claudeStart := time.Now()
		result, err := runAgent(agent, prompt, sbiDir, "review", currentTurn, enableStream)
		claudeEnd := time.Now()
		logClaudeInteraction(prompt, result, err, claudeStart, claudeEnd)
		if err != nil {
			output = fmt.Sprintf("# Review failed\n\nError: %v\n\nDECISION: NEEDS_CHANGES\n", err)
			decision = "NEEDS_CHANGES"
			errorMsg = err.Error()
		} else {
			output = result
			// Log the exact DECISION line if found
			lines := strings.Split(result, "\n")
			for _, line := range lines {
				if strings.Contains(line, "DECISION:") {
					Info("Found DECISION line: %s\n", strings.TrimSpace(line))
					break
				}
			}
			decision = parseDecision(result)
			Info("Review decision parsed: %s\n", decision)
			// Extract and append review note
			noteBody := ExtractNoteBody(result, "review")
			if noteErr := AppendNote("review", decision, noteBody, st.Turn, st.WIP, time.Now()); noteErr != nil {
				Warn("failed to append review_note: %v\n", noteErr)
			}
		}

	case "REVIEW&WIP":
		// Force implementation by reviewer
		Info("Running force implementation by reviewer\n")
		claudeStart := time.Now()
		result, err := runAgent(agent, prompt, sbiDir, "force_implement", currentTurn, enableStream)
		claudeEnd := time.Now()
		logClaudeInteraction(prompt, result, err, claudeStart, claudeEnd)
		if err != nil {
			output = fmt.Sprintf("# Force implementation failed\n\nError: %v\n\nDECISION: FAILED\n", err)
			decision = "FAILED"
			errorMsg = err.Error()
		} else {
			output = result
			decision = "SUCCEEDED" // Force implementation is final
			Info("Force implementation completed successfully\n")
		}

	case "DONE":
		output = fmt.Sprintf("# Workflow completed at %s\n", time.Now().Format(time.RFC3339))
		decision = st.Decision // Keep existing decision

	default:
		// Unknown status
		Error("Unknown status: %s\n", st.Status)
		st.Status = "READY"
	}

	// Determine next status
	nextStatus := nextStatusTransition(st.Status, decision, st.Attempt)
	Info("Transition: %s -> %s (decision: %s)\n", st.Status, nextStatus, decision)

	// Update attempt counter if going back to WIP
	if st.Status == "REVIEW" && nextStatus == "WIP" && decision == "NEEDS_CHANGES" {
		st.Attempt++
	}

	// For compatibility, update Current field
	next := "done"
	if nextStatus == "WIP" || nextStatus == "READY" {
		next = "implement"
	} else if nextStatus == "REVIEW" {
		next = "review"
	} else if nextStatus == "DONE" {
		next = "done"
	}

	// All journal records in this run will use the same turn number
	// (this is now set at the beginning of the run)

	// 5) 成果物出力 - SBIディレクトリ配下に保存
	// WIP (Work in Progress) からSBI-IDを取得
	if st.WIP == "" {
		// WIPがない場合はエラー
		Error("No work in progress (WIP) found in state\n")
		return fmt.Errorf("no work in progress")
	}

	// sbiDir is already defined earlier in the function
	if err := os.MkdirAll(sbiDir, 0o755); err != nil {
		return err
	}

	// 出力ファイル名を決定（ステップ名_turn番号.md）
	stepName := next
	if st.Current != "done" {
		stepName = next
	} else {
		stepName = st.Current
	}

	outFile := filepath.Join(sbiDir, fmt.Sprintf("%s_%d.md", stepName, currentTurn))
	if err := os.WriteFile(outFile, []byte(output), 0o644); err != nil {
		Error("failed to write artifact: %v\n", err)
		return fmt.Errorf("write artifact: %w", err)
	}

	if st.LastArtifacts == nil {
		st.LastArtifacts = map[string]string{}
	}
	st.LastArtifacts[stepName] = outFile

	// 6) Build artifacts list with note paths in SBI directory
	artifacts := []interface{}{outFile}

	// Add rolling note path for implement step (in SBI directory)
	if st.Current == "implement" {
		implNotePath := filepath.Join(sbiDir, "impl_notes.md")
		artifacts = append(artifacts, map[string]interface{}{
			"type": "impl_note_rollup",
			"path": implNotePath,
		})
	}

	// Add rolling note path for review step (in SBI directory)
	if st.Current == "review" {
		reviewNotePath := filepath.Join(sbiDir, "review_notes.md")
		artifacts = append(artifacts, map[string]interface{}{
			"type": "review_note_rollup",
			"path": reviewNotePath,
		})
	}

	// 7) ジャーナル追記（currentTurnを使用）
	elapsedMs := int(time.Since(startTime).Milliseconds())

	journalRec := map[string]interface{}{
		"ts":         time.Now().UTC().Format(time.RFC3339Nano),
		"turn":       currentTurn, // All entries in this run use same turn
		"step":       next,        // Legacy field
		"status":     nextStatus,  // New status field
		"attempt":    st.Attempt,  // Current attempt number
		"decision":   decision,
		"elapsed_ms": elapsedMs,
		"error":      errorMsg,
		"artifacts":  artifacts,
	}

	// 7) ステップの更新（ジャーナル記録前に状態を更新）
	// ターンは既にLine 119で設定済み（1 run = 1 turn）
	Info("Transitioning from status '%s' to '%s' (decision: %s)\n", st.Status, nextStatus, decision)
	st.Current = next
	st.Status = nextStatus
	st.Decision = decision

	// Clear WIP and lease when task is done
	if nextStatus == "DONE" {
		Info("Task %s completed! Clearing WIP, lease, and resetting turn.\n", st.WIP)
		st.WIP = ""
		ClearLease(st)
		st.Attempt = 0 // Reset attempt counter
		st.Turn = 0    // Reset turn counter for next task
		Info("Task completed, WIP and lease cleared, turn reset to 0")
	}

	// 8) health.json 更新（エラーに関わらず更新）
	healthOk := errorMsg == ""
	if err := app.WriteHealth(paths.Health, currentTurn, next, healthOk, errorMsg); err != nil {
		// health.json書き込みエラーも無視（ワークフローは継続）
		Warn("failed to write %s: %v\n", paths.Health, err)
	}

	// 9) State and Journal atomic save with TX
	// Always use TX mode: atomic update of state.json and journal
	if err := SaveStateAndJournalTX(st, journalRec, paths, prevV); err != nil {
		Error("failed to save state and journal (TX): %v\n", err)
		return err
	}
	Debug("state.json and journal saved atomically via TX")

	return nil
}
