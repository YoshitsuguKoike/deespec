package run

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

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
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/usecase/execution"
	"github.com/YoshitsuguKoike/deespec/internal/application/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
	infraRepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/claude_prompt"
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/workflow_sbi"
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
	common.Info("┌─ Claude Code Execution ─────────────────────────────────\n")
	common.Info("│ Start: %s\n", startTime.Format("15:04:05.000"))
	common.Info("│ Prompt: %d chars, %d lines\n", len(prompt), strings.Count(prompt, "\n")+1)

	// Show first few lines of prompt for context
	lines := strings.Split(prompt, "\n")
	if len(lines) > 0 {
		common.Info("│ Type: %s\n", strings.TrimPrefix(lines[0], "# "))
	}

	// Log end and result
	common.Info("│ End: %s (Duration: %.1fs)\n", endTime.Format("15:04:05.000"), elapsed.Seconds())

	if err != nil {
		common.Info("│ Status: ERROR - %v\n", err)
		common.Info("└──────────────────────────────────────────────────────────\n")
		return
	}

	// Analyze result
	resultLines := strings.Split(result, "\n")
	common.Info("│ Response: %d chars, %d lines\n", len(result), len(resultLines))

	// Log warnings for suspicious responses
	if len(result) == 0 {
		common.Warn("│ Warning: Empty response from AI\n")
	} else if len(result) < 100 {
		common.Warn("│ Warning: Unusually short response (%d chars)\n", len(result))
		common.Warn("│ Full content: %s\n", result)
	}

	// Always show AI response content (not just in debug mode)
	common.Info("┌─ AI Response Content ─────────────────────────────────────\n")
	// Show first 50 lines or entire response if shorter
	maxLines := 50
	if len(resultLines) <= maxLines {
		// Show entire response if it's short enough
		for i, line := range resultLines {
			common.Info("│ %4d: %s\n", i+1, line)
		}
	} else {
		// Show first and last parts for long responses
		for i := 0; i < 25; i++ {
			common.Info("│ %4d: %s\n", i+1, resultLines[i])
		}
		common.Info("│ ... (%d lines omitted) ...\n", len(resultLines)-50)
		for i := len(resultLines) - 25; i < len(resultLines); i++ {
			common.Info("│ %4d: %s\n", i+1, resultLines[i])
		}
	}
	common.Info("└──────────────────────────────────────────────────────────\n")

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
		common.Info("│ Decision: %s\n", decision)
	}
	if noteFound {
		common.Info("│ Note: Found in response\n")
	}
	common.Info("└──────────────────────────────────────────────────────────\n")
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
			common.Info("Decision extracted: %s (pattern: %s)\n", decision, pattern)
			return decision
		}
	}

	// Default to NEEDS_CHANGES if no valid decision found
	common.Info("No valid DECISION found in response, defaulting to NEEDS_CHANGES\n")
	return "NEEDS_CHANGES"
}

// getTaskDescription returns task description based on status and attempt
func getTaskDescription(st *common.State) string {
	switch st.Status {
	case "READY", "", "WIP":
		if st.Attempt == 1 {
			todo := ""
			if todoVal, ok := st.Inputs["todo"].(string); ok {
				todo = todoVal
			}
			if todo == "" {
				todo = fmt.Sprintf("Implement SBI task %s", st.WIP)
			}
			return todo
		} else if st.Attempt == 2 {
			taskDesc := fmt.Sprintf("Second attempt for %s. Review feedback and implement improvements.", st.WIP)
			// LastArtifacts is now []string, not map - look for review file in array
			for _, artifact := range st.LastArtifacts {
				if strings.Contains(artifact, "review") {
					if content, err := os.ReadFile(artifact); err == nil {
						taskDesc = fmt.Sprintf("Second attempt based on review feedback:\n\n%s", string(content))
					}
					break
				}
			}
			return taskDesc
		} else {
			taskDesc := fmt.Sprintf("Third attempt for %s. Final chance to implement correctly.", st.WIP)
			// LastArtifacts is now []string, not map - look for review file in array
			for _, artifact := range st.LastArtifacts {
				if strings.Contains(artifact, "review") {
					if content, err := os.ReadFile(artifact); err == nil {
						taskDesc = fmt.Sprintf("Third attempt based on review feedback:\n\n%s", string(content))
					}
					break
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
// Phase 9.1e: Added labelRepo parameter for Repository-based label enrichment
func buildPromptByStatus(st *common.State, labelRepo repository.LabelRepository) string {
	builder := claude_prompt.NewClaudeCodePromptBuilder(
		getCurrentWorkDir(),
		filepath.Join(".deespec", "specs", "sbi", st.WIP),
		st.WIP,
		st.Turn,
		determineStep(st.Status, st.Attempt),
		labelRepo, // Phase 9.1e: Repository for label enrichment
	)

	// Determine task description based on status and attempt
	taskDesc := getTaskDescription(st)

	// Try to load external prompt first
	externalPrompt, err := builder.LoadExternalPrompt(st.Status, taskDesc)
	if err == nil {
		common.Info("Loaded external prompt from .deespec/prompts/ for status: %s\n", st.Status)
		return externalPrompt
	}

	// Fall back to hardcoded prompts
	common.Info("Using default prompt (external prompt not found: %v)\n", err)

	switch st.Status {
	case "READY", "":
		// Initial implementation (Turn 1, Step 2: implement_try)
		todo := ""
		if todoVal, ok := st.Inputs["todo"].(string); ok {
			todo = todoVal
		}
		if todo == "" {
			todo = fmt.Sprintf("Implement SBI task %s", st.WIP)
		}
		return builder.BuildImplementPrompt(todo)

	case "WIP":
		// Implementation or re-implementation
		if st.Attempt == 1 {
			// First attempt (Step 2: implement_try)
			todo := ""
			if todoVal, ok := st.Inputs["todo"].(string); ok {
				todo = todoVal
			}
			if todo == "" {
				todo = fmt.Sprintf("Implement SBI task %s", st.WIP)
			}
			return builder.BuildImplementPrompt(todo)
		} else if st.Attempt == 2 {
			// Second attempt (Step 4: implement_2nd_try)
			taskDesc := fmt.Sprintf("Second attempt for %s. Review feedback and implement improvements.", st.WIP)
			// LastArtifacts is now []string, not map - look for review file in array
			for _, artifact := range st.LastArtifacts {
				if strings.Contains(artifact, "review") {
					if content, err := os.ReadFile(artifact); err == nil {
						taskDesc = fmt.Sprintf("Second attempt based on review feedback:\n\n%s", string(content))
					}
					break
				}
			}
			return builder.BuildImplementPrompt(taskDesc)
		} else {
			// Third attempt (Step 6: implement_3rd_try)
			taskDesc := fmt.Sprintf("Third attempt for %s. Final chance to implement correctly.", st.WIP)
			// LastArtifacts is now []string, not map - look for review file in array
			for _, artifact := range st.LastArtifacts {
				if strings.Contains(artifact, "review") {
					if content, err := os.ReadFile(artifact); err == nil {
						taskDesc = fmt.Sprintf("Third attempt based on review feedback:\n\n%s", string(content))
					}
					break
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
func buildImplementPrompt(st *common.State) string {
	return buildPromptByStatus(st, nil) // nil for backward compatibility
}

func buildReviewPrompt(st *common.State) string {
	return buildPromptByStatus(st, nil) // nil for backward compatibility
}

func getCurrentWorkDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}

// cleanupStaleLocks removes stale lock files at startup
func cleanupStaleLocks() {
	// Skip cleanup if no config available (e.g., in tests)
	if common.GetGlobalConfig() == nil {
		return
	}

	// Get paths using config
	paths := app.GetPathsWithConfig(common.GetGlobalConfig())

	// Check for state.lock file
	if lockInfo, err := os.Stat(paths.StateLock); err == nil {
		// Lock file exists, check if it's stale
		shouldRemove := false
		removeReason := ""

		// Load state to check lease
		if st, loadErr := common.LoadState(paths.State); loadErr == nil {
			if st.LeaseExpiresAt != "" && common.LeaseExpired(st) {
				// Lease expired
				shouldRemove = true
				removeReason = fmt.Sprintf("expired lease for task %s", st.WIP)
			} else if st.LeaseExpiresAt == "" {
				// No active lease, check lock age
				lockAge := time.Since(lockInfo.ModTime())
				if lockAge > 10*time.Minute {
					shouldRemove = true
					removeReason = fmt.Sprintf("lock file is %v old with no active lease", lockAge.Round(time.Second))
				}
			}
		} else {
			// Can't read state, check lock age
			lockAge := time.Since(lockInfo.ModTime())
			if lockAge > 10*time.Minute {
				shouldRemove = true
				removeReason = fmt.Sprintf("lock file is %v old and state unreadable", lockAge.Round(time.Second))
			}
		}

		if shouldRemove {
			common.Info("[Manager Startup] Removing stale lock: %s\n", removeReason)
			if err := os.Remove(paths.StateLock); err != nil {
				common.Warn("[Manager Startup] Failed to remove stale lock: %v\n", err)
			} else {
				common.Info("[Manager Startup] Successfully removed stale lock\n")
			}
		} else {
			common.Info("[Manager Startup] Active lock found, will respect it\n")
		}
	}

	// Also check for runlock using new Lock Service
	if container, err := common.InitializeContainer(); err == nil {
		defer container.Close()
		lockService := container.GetLockService()
		ctx := context.Background()

		lockID, _ := lock.NewLockID("system-runlock")
		if runLock, err := lockService.FindRunLock(ctx, lockID); err == nil && runLock != nil {
			if runLock.IsExpired() {
				common.Info("[Manager Startup] Removing expired runlock from PID %d (expired: %s)\n",
					runLock.PID(), runLock.ExpiresAt())
				_ = lockService.ReleaseRunLock(ctx, lockID) // cleanup
			} else {
				common.Info("[Manager Startup] Valid runlock exists from PID %d (expires: %s)\n",
					runLock.PID(), runLock.ExpiresAt())
			}
		}
	}
}

// NewCommand creates the run command
func NewCommand() *cobra.Command {
	var autoFB bool
	var intervalStr string
	var enabledWorkflows []string
	var maxParallel int // Maximum number of concurrent SBI executions

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run all enabled workflows with optional parallel execution",
		Long: `Run all enabled workflows with optional parallel execution.

This command runs multiple workflow types (SBI, PBI, etc.) simultaneously,
each in their own execution loop. Use Ctrl+C to stop all workflows gracefully.

Parallel Execution:
  Use --parallel flag to enable concurrent SBI processing (1-10 tasks).
  Default is 1 (sequential execution). Higher values increase throughput
  but require more system resources.

Configuration:
  Workflows can be configured via .deespec/workflow.yaml file.
  Use 'deespec workflow generate-example' to create a sample configuration.

Individual workflows:
  - deespec sbi run   (for SBI workflow only)
  - deespec pbi run   (for PBI workflow only, when available)

Examples:
  deespec run                           # Run all enabled workflows (sequential)
  deespec run --parallel 3              # Run up to 3 SBIs concurrently
  deespec run --workflows sbi           # Run only SBI workflow
  deespec run --interval 10s            # Run with 10-second intervals
  deespec run --auto-fb                 # Enable automatic FB-SBI registration
  deespec run --parallel 5 --interval 30s  # 5 concurrent tasks, 30s intervals`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse interval
			interval, err := ParseInterval(intervalStr)
			if err != nil {
				return fmt.Errorf("invalid interval: %v", err)
			}

			// Validate and set default for maxParallel
			if maxParallel < 1 {
				maxParallel = 1 // Default to sequential execution
			}
			if maxParallel > 10 {
				return fmt.Errorf("--parallel must be between 1 and 10, got: %d", maxParallel)
			}

			// Check config for auto-fb (config takes precedence over flag)
			if common.GetGlobalConfig() != nil && common.GetGlobalConfig().AutoFB() {
				autoFB = true
			}

			// Log parallel execution mode
			if maxParallel > 1 {
				common.Info("[Parallel Mode] Enabled with max %d concurrent SBI executions\n", maxParallel)
			} else {
				common.Info("[Sequential Mode] Running one SBI at a time\n")
			}

			// Cleanup stale locks before starting
			cleanupStaleLocks()

			// Initialize DI container once for the entire command execution
			// This avoids repeated container creation and database connection overhead
			common.Info("[Container] Initializing DI container...\n")
			container, err := common.InitializeContainer()
			if err != nil {
				return fmt.Errorf("failed to initialize container: %w", err)
			}
			defer func() {
				common.Info("[Container] Closing DI container...\n")
				container.Close()
			}()

			// Start container services (Lock Service, etc.)
			ctx := context.Background()
			if err := container.Start(ctx); err != nil {
				return fmt.Errorf("failed to start container services: %w", err)
			}

			// Create workflow manager with logging functions
			manager := workflow.NewWorkflowManager(common.Info, common.Warn, common.Debug)

			// Register SBI workflow (parallel or sequential based on maxParallel)
			var sbiRunner workflow.WorkflowRunner

			if maxParallel > 1 {
				// Use ParallelSBIWorkflowRunner for concurrent execution
				// Create ExecuteTurnFunc that wraps RunTurnWithContainer
				executeTurnFunc := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
					// TODO: Implement per-SBI execution logic
					// For now, fallback to RunTurnWithContainer
					return RunTurnWithContainer(container, autoFB)
				}

				sbiRunner = workflow_sbi.NewParallelSBIWorkflowRunner(container, maxParallel, executeTurnFunc)
			} else {
				// Use sequential SBIWorkflowRunner
				runTurnFunc := func(autoFB bool) error {
					return RunTurnWithContainer(container, autoFB)
				}
				sbiRunner = workflow_sbi.NewSBIWorkflowRunnerWithFunc(runTurnFunc)
			}

			sbiConfig := workflow.WorkflowConfig{
				Name:     "sbi",
				Enabled:  true,
				Interval: interval,
				AutoFB:   autoFB,
			}

			// Override enabled workflows if specified
			if len(enabledWorkflows) > 0 {
				sbiConfig.Enabled = false
				for _, wf := range enabledWorkflows {
					if wf == "sbi" {
						sbiConfig.Enabled = true
						break
					}
				}
			}

			if err := manager.RegisterWorkflow(sbiRunner, sbiConfig); err != nil {
				return fmt.Errorf("failed to register SBI workflow: %v", err)
			}

			// Setup signal handling for graceful shutdown
			signalCtx, cancel := SetupSignalHandler()
			defer cancel()

			// Setup cleanup
			defer func() {
				manager.Stop()
				manager.PrintStats()
			}()

			// Start all enabled workflows
			if err := manager.RunAll(); err != nil {
				return fmt.Errorf("failed to start workflows: %v", err)
			}

			// Wait briefly for workflows to start, then check if they all stopped immediately
			time.Sleep(500 * time.Millisecond)

			stats := manager.GetStats()
			allStopped := true
			for _, stat := range stats {
				if stat.IsRunning {
					allStopped = false
					break
				}
			}

			if allStopped {
				// All workflows stopped immediately (e.g., due to lock errors)
				return fmt.Errorf("all workflows stopped - another instance may be running")
			}

			// Wait for shutdown signal
			<-signalCtx.Done()
			common.Info("Shutdown signal received, stopping all workflows...\n")

			return nil
		},
	}

	cmd.Flags().BoolVar(&autoFB, "auto-fb", false, "Automatically register FB-SBI drafts")
	cmd.Flags().StringVar(&intervalStr, "interval", "", "Execution interval for all workflows (default: 5s, min: 1s, max: 10m)")
	cmd.Flags().StringSliceVar(&enabledWorkflows, "workflows", nil, "Comma-separated list of workflows to enable (default: all available)")
	cmd.Flags().IntVar(&maxParallel, "parallel", 1, "Maximum concurrent SBI executions (1-10, default: 1)")

	return cmd
}

// Helper function to run agent and save history
func runAgent(agent claudecli.Runner, prompt string, sbiDir string, stepName string, turn int, enableStream bool) (string, error) {
	// Log that we're starting AI execution
	common.Info("🤖 Starting AI agent execution for step: %s (turn: %d)\n", stepName, turn)
	common.Info("   Working directory: %s\n", sbiDir)

	// Create histories directory
	historiesDir := filepath.Join(sbiDir, "histories")
	if err := os.MkdirAll(historiesDir, 0755); err != nil {
		common.Warn("Failed to create histories directory: %v\n", err)
	}

	// Create history file with consistent naming: workflow_step_N.jsonl
	historyFile := filepath.Join(historiesDir, fmt.Sprintf("workflow_step_%d.jsonl", turn))

	if enableStream {
		// Try streaming mode first
		// Start heartbeat for streaming mode
		common.Info("   Starting AI execution (streaming mode) with heartbeat monitoring...")
		streamHeartbeatCtx, cancelStreamHeartbeat := context.WithCancel(context.Background())
		defer cancelStreamHeartbeat()

		streamStarted := make(chan bool, 1)
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			elapsed := 0

			// Signal that goroutine has started
			streamStarted <- true

			for {
				select {
				case <-streamHeartbeatCtx.Done():
					return
				case <-ticker.C:
					elapsed += 5
					common.Info("   ⏳ AI agent still processing (streaming)... (%d seconds elapsed)", elapsed)
				}
			}
		}()

		// Wait for goroutine to start
		select {
		case <-streamStarted:
			common.Debug("   Streaming heartbeat goroutine started")
		case <-time.After(100 * time.Millisecond):
			common.Debug("   Streaming heartbeat goroutine timeout")
		}

		streamCtx := &claudecli.StreamContext{
			SBIDir:   sbiDir,
			StepName: stepName,
			Turn:     turn,
			LogWriter: func(format string, args ...interface{}) {
				// Log stream events with prefix for clarity
				// Always log to debug
				common.Debug("[STREAM] "+format, args...)
				// Also log important events to info
				if strings.Contains(format, "error") || strings.Contains(format, "warning") || strings.Contains(format, "final") {
					common.Info("[STREAM] "+format, args...)
				}
			},
		}
		result, err := agent.RunWithStream(context.Background(), prompt, streamCtx, nil)
		cancelStreamHeartbeat() // Stop streaming heartbeat
		if err == nil {
			// Check if result seems valid
			if len(result) == 0 {
				common.Warn("Streaming returned empty result, falling back to regular mode\n")
			} else {
				common.Info("✅ AI agent completed successfully (streaming mode, %d chars)\n", len(result))
				// Also save raw response to a debug file for inspection
				resultsDir := filepath.Join(sbiDir, "results")
				if err := os.MkdirAll(resultsDir, 0755); err == nil {
					if debugFile := filepath.Join(resultsDir, fmt.Sprintf("raw_response_%s_%d.txt", stepName, turn)); len(result) > 0 {
						if err := os.WriteFile(debugFile, []byte(result), 0644); err == nil {
							common.Debug("Raw response saved to: %s\n", debugFile)
						}
					}
				}
				return result, nil
			}
		} else {
			// If streaming fails, fall back to regular mode
			common.Warn("Streaming mode failed, falling back to regular mode: %v\n", err)
		}
	}

	// Use regular mode and save result as history
	startTime := time.Now()

	// Start heartbeat goroutine for long-running AI execution
	common.Info("   Starting AI execution with heartbeat monitoring...")
	heartbeatCtx, cancelHeartbeat := context.WithCancel(context.Background())
	defer cancelHeartbeat()

	// Create a channel to confirm goroutine started
	started := make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		elapsed := 0

		// Signal that goroutine has started
		started <- true

		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				elapsed += 5
				common.Info("   ⏳ AI agent still processing... (%d seconds elapsed)", elapsed)
			}
		}
	}()

	// Wait for goroutine to start (with timeout)
	select {
	case <-started:
		common.Debug("   Heartbeat goroutine started successfully")
	case <-time.After(100 * time.Millisecond):
		common.Debug("   Heartbeat goroutine start timeout")
	}

	common.Info("   Calling AI agent now...")
	result, err := agent.Run(context.Background(), prompt)
	cancelHeartbeat() // Stop heartbeat
	endTime := time.Now()
	common.Info("   AI agent call completed (duration: %v)", endTime.Sub(startTime))

	// Save raw response to a debug file for inspection
	resultsDir := filepath.Join(sbiDir, "results")
	if err := os.MkdirAll(resultsDir, 0755); err == nil {
		if debugFile := filepath.Join(resultsDir, fmt.Sprintf("raw_response_%s_%d.txt", stepName, turn)); err == nil && len(result) > 0 {
			if werr := os.WriteFile(debugFile, []byte(result), 0644); werr == nil {
				common.Debug("Raw response saved to: %s\n", debugFile)
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
		common.Debug("History saved to: %s\n", historyFile)
	}

	if err != nil {
		common.Info("❌ AI agent execution failed: %v\n", err)
	} else if len(result) > 0 {
		common.Info("✅ AI agent completed successfully (regular mode, %d chars)\n", len(result))
	}

	return result, err
}

// runOnce has been removed and replaced by runTurn()
// See runTurn() below for the new UseCase-based implementation

// RunTurnWithContainer executes a single workflow turn using a shared DI container
// This function accepts a pre-initialized container to avoid repeated initialization
func RunTurnWithContainer(container *di.Container, autoFB bool) error {
	startTime := time.Now()

	// Get paths using config
	paths := app.GetPathsWithConfig(common.GetGlobalConfig())

	// Get services from container
	lockService := container.GetLockService()
	sbiRepo := container.GetSBIRepository() // Added for DB-based task picking
	ctx := context.Background()

	// Create repository implementations
	stateRepo := infraRepo.NewStateRepositoryImpl(paths.State)
	journalRepo := infraRepo.NewJournalRepositoryImpl(paths.Journal)

	// Get AgentGateway from container
	agentGateway := container.GetAgentGateway()

	// Get max turns and lease TTL from config
	maxTurns := 8
	leaseTTL := 10 * time.Minute
	if common.GetGlobalConfig() != nil {
		maxTurns = common.GetGlobalConfig().MaxTurns()
	}

	// Create RunTurnUseCase with SBIRepository for DB-based task management
	useCase := execution.NewRunTurnUseCase(
		stateRepo,
		journalRepo,
		sbiRepo, // Added for DB-based task picking
		lockService,
		agentGateway,
		maxTurns,
		leaseTTL,
	)

	// Execute turn
	input := dto.RunTurnInput{
		AutoFB: autoFB,
	}

	output, err := useCase.Execute(ctx, input)
	if err != nil {
		common.Error("failed to execute turn: %v", err)
		return fmt.Errorf("execute turn: %w", err)
	}

	// Log execution results
	if output.NoOp {
		switch output.NoOpReason {
		case "lock_held":
			// Return error immediately when lock is held by another instance
			// This prevents the workflow from waiting indefinitely
			lockService := container.GetLockService()
			lockID, _ := lock.NewLockID("system-runlock")
			if existingLock, err := lockService.FindRunLock(ctx, lockID); err == nil && existingLock != nil {
				return fmt.Errorf("another instance is already running (PID %d on %s, expires: %s)",
					existingLock.PID(), existingLock.Hostname(), existingLock.ExpiresAt().Format("15:04:05"))
			}
			return fmt.Errorf("another instance is already running")
		case "no_tasks":
			common.Info("💤 No tasks available to process")
		default:
			if output.Turn == 0 {
				common.Info("⏳ Waiting...")
			} else {
				common.Info("No work done (Turn: %d)", output.Turn)
			}
		}
	} else {
		common.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		common.Info("🔄 Turn %d completed at %s", output.Turn, output.CompletedAt.Format("15:04:05"))

		if output.TaskPicked {
			common.Info("   ✓ Task picked: %s", output.SBIID)
			common.Info("   Status: %s", output.NextStatus)
		} else if output.TaskCompleted {
			common.Info("   ✓ Task completed: %s", output.SBIID)
			common.Info("   Final decision: %s", output.Decision)
		} else {
			common.Info("   SBI: %s", output.SBIID)
			common.Info("   Transition: %s -> %s", output.PrevStatus, output.NextStatus)
			if output.PrevStep != output.NextStep {
				common.Info("   Step: %s -> %s", output.PrevStep, output.NextStep)
			}
			if output.Decision != "" {
				common.Info("   Decision: %s", output.Decision)
			}
			if output.Attempt > 0 {
				common.Info("   Attempt: %d", output.Attempt)
			}
		}

		if output.ErrorMsg != "" {
			common.Warn("   Error: %s", output.ErrorMsg)
		}

		common.Info("   Elapsed: %dms", output.ElapsedMs)
		common.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	// Update health
	healthOk := output.ErrorMsg == ""
	if err := app.WriteHealth(paths.Health, output.Turn, output.NextStep, healthOk, output.ErrorMsg); err != nil {
		common.Warn("failed to write %s: %v\n", paths.Health, err)
	}

	common.Debug("Turn execution took %v", time.Since(startTime))
	return nil
}

// RunTurn executes a single workflow turn (Legacy compatibility wrapper)
// This function creates a new container for each execution
// Deprecated: Use RunTurnWithContainer for better performance
func RunTurn(autoFB bool) error {
	// Initialize DI container
	container, err := common.InitializeContainer()
	if err != nil {
		return fmt.Errorf("failed to initialize container: %w", err)
	}
	defer container.Close()

	// Start lock service for heartbeat monitoring
	ctx := context.Background()
	if err := container.Start(ctx); err != nil {
		return fmt.Errorf("failed to start lock service: %w", err)
	}

	return RunTurnWithContainer(container, autoFB)
}
