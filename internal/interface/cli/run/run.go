package run

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/usecase/execution"
	"github.com/YoshitsuguKoike/deespec/internal/application/workflow"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
	"github.com/YoshitsuguKoike/deespec/internal/infrastructure/di"
	infraRepo "github.com/YoshitsuguKoike/deespec/internal/infrastructure/repository"
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

	// Check for runlock using Lock Service (DB-based)
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

// isProcessRunning checks if a process with the given PID is running
func isProcessRunning(pid int) bool {
	// Use ps command to check if process exists
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid))
	err := cmd.Run()
	return err == nil
}

// promptUserConfirmation asks the user for yes/no confirmation
func promptUserConfirmation(message string) bool {
	fmt.Printf("\n%s (y/N): ", message)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// killProcessAndCleanup kills a process and cleans up database locks
func killProcessAndCleanup(pid int, container *di.Container) error {
	// Try graceful termination first (SIGTERM)
	common.Info("Stopping process PID %d...\n", pid)
	cmd := exec.Command("kill", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send kill signal: %w", err)
	}

	// Wait for process to terminate
	time.Sleep(500 * time.Millisecond)

	// Check if process is still running
	if isProcessRunning(pid) {
		// Process didn't terminate, try force kill (SIGKILL)
		common.Warn("Process %d did not terminate gracefully, forcing termination...\n", pid)
		cmd = exec.Command("kill", "-9", strconv.Itoa(pid))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to force kill process: %w", err)
		}

		// Wait a bit and verify again
		time.Sleep(500 * time.Millisecond)
		if isProcessRunning(pid) {
			return fmt.Errorf("process %d is still running after force kill signal", pid)
		}
	}

	common.Info("Process %d stopped successfully\n", pid)

	// Clean up database locks
	common.Info("Cleaning up database locks...\n")
	db := container.GetDB()
	if db == nil {
		return fmt.Errorf("database not available")
	}

	// Delete run_locks and state_locks
	if _, err := db.Exec("DELETE FROM run_locks"); err != nil {
		return fmt.Errorf("failed to delete run_locks: %w", err)
	}
	if _, err := db.Exec("DELETE FROM state_locks"); err != nil {
		return fmt.Errorf("failed to delete state_locks: %w", err)
	}

	common.Info("Database locks cleaned up successfully\n")
	return nil
}

// handleLockConflict handles the case when another instance is running
// Returns true if the user wants to continue after cleanup, false otherwise
func handleLockConflict(ctx context.Context, container *di.Container) (bool, error) {
	lockService := container.GetLockService()
	lockID, _ := lock.NewLockID("system-runlock")

	existingLock, err := lockService.FindRunLock(ctx, lockID)
	if err != nil || existingLock == nil {
		// Lock no longer exists, can continue
		return true, nil
	}

	pid := existingLock.PID()
	hostname := existingLock.Hostname()
	expiresAt := existingLock.ExpiresAt().Format("15:04:05")

	// Check if process is actually running
	if !isProcessRunning(pid) {
		common.Warn("Lock held by PID %d, but process is not running (stale lock)\n", pid)
		common.Info("Cleaning up stale lock...\n")

		// Clean up stale lock
		if err := lockService.ReleaseRunLock(ctx, lockID); err != nil {
			return false, fmt.Errorf("failed to release stale lock: %w", err)
		}

		// Also clean up database locks
		db := container.GetDB()
		if db != nil {
			db.Exec("DELETE FROM run_locks")
			db.Exec("DELETE FROM state_locks")
		}

		common.Info("Stale lock cleaned up successfully\n")
		return true, nil
	}

	// Process is running - prompt user for confirmation
	common.Warn("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	common.Warn("⚠️  Another instance is already running\n")
	common.Warn("    PID: %d\n", pid)
	common.Warn("    Hostname: %s\n", hostname)
	common.Warn("    Lock expires: %s\n", expiresAt)
	common.Warn("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if !promptUserConfirmation("Do you want to stop the other process and continue?") {
		common.Info("Aborted by user\n")
		return false, nil
	}

	// User confirmed - kill process and cleanup
	if err := killProcessAndCleanup(pid, container); err != nil {
		return false, fmt.Errorf("failed to cleanup: %w", err)
	}

	common.Info("✓ Ready to start\n\n")
	return true, nil
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

			// Set lock service for querying current task info during heartbeat
			manager.SetLockService(container.GetLockService())

			// Set SBI repository for querying task status during heartbeat
			manager.SetSBIRepository(container.GetSBIRepository())

			// Register SBI workflow (parallel or sequential based on maxParallel)
			var sbiRunner workflow.WorkflowRunner

			if maxParallel > 1 {
				// Use ParallelSBIWorkflowRunner for concurrent execution
				// Create ExecuteTurnFunc that executes a specific SBI
				executeTurnFunc := func(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
					// Use ExecuteSingleSBI which doesn't acquire RunLock
					// The RunLock is managed by the parallel workflow manager itself
					return ExecuteSingleSBI(ctx, container, sbiID, autoFB)
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
				// All workflows stopped immediately - check if it's due to lock conflict
				shouldContinue, err := handleLockConflict(ctx, container)
				if err != nil {
					return fmt.Errorf("failed to handle lock conflict: %w", err)
				}
				if !shouldContinue {
					return fmt.Errorf("all workflows stopped - another instance may be running")
				}

				// User confirmed cleanup - retry starting workflows
				common.Info("Retrying workflow startup...\n")
				if err := manager.RunAll(); err != nil {
					return fmt.Errorf("failed to restart workflows: %v", err)
				}

				// Wait briefly and check again
				time.Sleep(500 * time.Millisecond)
				stats = manager.GetStats()
				allStopped = true
				for _, stat := range stats {
					if stat.IsRunning {
						allStopped = false
						break
					}
				}

				if allStopped {
					return fmt.Errorf("workflows still failed to start after cleanup")
				}

				common.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
				common.Info("✅ Workflows started successfully\n")
				common.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")
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

// ExecuteSingleSBI executes a turn for a specific SBI without acquiring RunLock
// This function is designed for parallel execution where RunLock is managed externally
// StateLock for the specific SBI should be acquired by the caller before calling this
func ExecuteSingleSBI(ctx context.Context, container *di.Container, sbiID string, autoFB bool) error {
	startTime := time.Now()

	// Get paths and services
	paths := app.GetPathsWithConfig(common.GetGlobalConfig())
	lockService := container.GetLockService()
	sbiRepo := container.GetSBIRepository()

	// Create repository implementations
	journalRepo := infraRepo.NewJournalRepositoryImpl(paths.Journal)

	// Get AgentGateway from container
	agentGateway := container.GetAgentGateway()

	// Get max turns and lease TTL from config
	maxTurns := 8
	leaseTTL := 10 * time.Minute
	if common.GetGlobalConfig() != nil {
		maxTurns = common.GetGlobalConfig().MaxTurns()
	}

	// Create RunTurnUseCase
	useCase := execution.NewRunTurnUseCase(
		journalRepo,
		sbiRepo,
		lockService,
		agentGateway,
		maxTurns,
		leaseTTL,
	)

	// Execute turn for the specific SBI
	// Note: ExecuteForSBI skips SBI picking and uses the provided SBI ID
	input := dto.RunTurnInput{
		AutoFB: autoFB,
	}

	output, err := useCase.ExecuteForSBI(ctx, sbiID, input)
	if err != nil {
		common.Error("failed to execute turn for SBI %s: %v", sbiID, err)
		return fmt.Errorf("execute turn for SBI %s: %w", sbiID, err)
	}

	// Log execution results (simplified for parallel execution)
	if output.NoOp {
		common.Debug("SBI %s: No-op (%s)", sbiID, output.NoOpReason)
	} else {
		common.Info("SBI %s: Turn %d completed (%s -> %s)",
			sbiID[:8], output.Turn, output.PrevStatus, output.NextStatus)
	}

	// Update health
	healthOk := output.ErrorMsg == ""
	if err := app.WriteHealth(paths.Health, output.Turn, output.NextStep, healthOk, output.ErrorMsg); err != nil {
		common.Warn("failed to write %s: %v\n", paths.Health, err)
	}

	common.Debug("SBI %s execution took %v", sbiID[:8], time.Since(startTime))
	return nil
}

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

	// Acquire RunLock at CLI layer (not in UseCase layer)
	// This ensures single instance execution across sequential/parallel modes
	lockID, err := lock.NewLockID("system-runlock")
	if err != nil {
		return fmt.Errorf("failed to create lock ID: %w", err)
	}

	leaseTTL := 10 * time.Minute
	runLock, err := lockService.AcquireRunLock(ctx, lockID, leaseTTL)
	if err != nil {
		return fmt.Errorf("failed to acquire run lock: %w", err)
	}

	if runLock == nil {
		// Another instance is running - return error immediately
		if existingLock, err := lockService.FindRunLock(ctx, lockID); err == nil && existingLock != nil {
			return fmt.Errorf("another instance is already running (PID %d on %s, expires: %s)",
				existingLock.PID(), existingLock.Hostname(), existingLock.ExpiresAt().Format("15:04:05"))
		}
		return fmt.Errorf("another instance is already running")
	}

	defer func() {
		if err := lockService.ReleaseRunLock(ctx, lockID); err != nil {
			common.Warn("Failed to release run lock: %v", err)
		}
	}()

	// Create repository implementations
	journalRepo := infraRepo.NewJournalRepositoryImpl(paths.Journal)

	// Get AgentGateway from container
	agentGateway := container.GetAgentGateway()

	// Get max turns from config (leaseTTL already defined above)
	maxTurns := 8
	if common.GetGlobalConfig() != nil {
		maxTurns = common.GetGlobalConfig().MaxTurns()
	}

	// Create RunTurnUseCase with DB-based repositories
	useCase := execution.NewRunTurnUseCase(
		journalRepo,
		sbiRepo,
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
