package workflow

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/service"
	"github.com/YoshitsuguKoike/deespec/internal/domain/repository"
)

// LogFunc is a function type for logging
type LogFunc func(format string, args ...interface{})

// WorkflowManager manages multiple workflows running in parallel
type WorkflowManager struct {
	workflows map[string]WorkflowRunner
	configs   map[string]WorkflowConfig
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mutex     sync.RWMutex

	// Statistics
	stats map[string]*WorkflowStats

	// Logging functions
	info  LogFunc
	warn  LogFunc
	debug LogFunc

	// Lock service for querying current task info (optional)
	lockService service.LockService

	// SBI repository for querying task status (optional)
	sbiRepo repository.SBIRepository
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager(info, warn, debug LogFunc) *WorkflowManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Use default no-op logger if not provided
	if info == nil {
		info = func(format string, args ...interface{}) {}
	}
	if warn == nil {
		warn = func(format string, args ...interface{}) {}
	}
	if debug == nil {
		debug = func(format string, args ...interface{}) {}
	}

	return &WorkflowManager{
		workflows: make(map[string]WorkflowRunner),
		configs:   make(map[string]WorkflowConfig),
		stats:     make(map[string]*WorkflowStats),
		ctx:       ctx,
		cancel:    cancel,
		info:      info,
		warn:      warn,
		debug:     debug,
	}
}

// SetLockService sets the lock service for querying task information (optional)
func (wm *WorkflowManager) SetLockService(lockService service.LockService) {
	wm.lockService = lockService
}

// SetSBIRepository sets the SBI repository for querying task status (optional)
func (wm *WorkflowManager) SetSBIRepository(sbiRepo repository.SBIRepository) {
	wm.sbiRepo = sbiRepo
}

// TaskInfo holds information about the currently processing task
type TaskInfo struct {
	TaskID string
	Title  string // Task title for display
	Step   string // Current workflow step (PICK, IMPLEMENT, REVIEW, DONE)
	Turn   int    // Current turn number (1-based)
}

// truncateString truncates a string to maxChars characters (counting runes for multibyte support)
// If truncated, appends "..." to the result
func truncateString(s string, maxChars int) string {
	runes := []rune(s)
	if len(runes) <= maxChars {
		return s
	}
	// Use maxChars-3 to leave room for "..."
	if maxChars <= 3 {
		return "..."
	}
	return string(runes[:maxChars-3]) + "..."
}

// getCurrentTaskInfo retrieves the currently processing task ID and status for a workflow
// by querying state locks and SBI repository
func (wm *WorkflowManager) getCurrentTaskInfo(workflowName string) *TaskInfo {
	if wm.lockService == nil {
		return nil
	}

	// Query all state locks
	locks, err := wm.lockService.ListStateLocks(wm.ctx)
	if err != nil {
		return nil
	}

	// Look for locks matching the workflow pattern (e.g., "sbi/SBI-XXX")
	prefix := workflowName + "/"
	for _, stateLock := range locks {
		lockID := stateLock.LockID().String()
		if strings.HasPrefix(lockID, prefix) {
			// Extract task ID from lock ID (e.g., "sbi/SBI-TEST-002" -> "SBI-TEST-002")
			taskID := strings.TrimPrefix(lockID, prefix)

			info := &TaskInfo{
				TaskID: taskID,
			}

			// Try to get task details from SBI repository if available
			if wm.sbiRepo != nil && workflowName == "sbi" {
				if sbi, err := wm.sbiRepo.Find(wm.ctx, repository.SBIID(taskID)); err == nil && sbi != nil {
					info.Title = truncateString(sbi.Title(), 50)
					info.Step = string(sbi.CurrentStep())
					info.Turn = sbi.ExecutionState().CurrentTurn.Value()
				}
			}

			return info
		}
	}

	return nil
}

// RegisterWorkflow registers a new workflow runner
func (wm *WorkflowManager) RegisterWorkflow(runner WorkflowRunner, config WorkflowConfig) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	name := runner.Name()
	if _, exists := wm.workflows[name]; exists {
		return fmt.Errorf("workflow %s already registered", name)
	}

	wm.workflows[name] = runner
	wm.configs[name] = config
	wm.stats[name] = &WorkflowStats{
		Name: name,
	}

	wm.info("Registered workflow: %s (%s)\n", name, runner.Description())
	return nil
}

// GetWorkflowNames returns a list of all registered workflow names
func (wm *WorkflowManager) GetWorkflowNames() []string {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	names := make([]string, 0, len(wm.workflows))
	for name := range wm.workflows {
		names = append(names, name)
	}
	return names
}

// GetEnabledWorkflows returns a list of enabled workflow names
func (wm *WorkflowManager) GetEnabledWorkflows() []string {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	var enabled []string
	for name, runner := range wm.workflows {
		if config, exists := wm.configs[name]; exists && config.Enabled && runner.IsEnabled() {
			enabled = append(enabled, name)
		}
	}
	return enabled
}

// RunWorkflow starts a single workflow in its own goroutine
func (wm *WorkflowManager) RunWorkflow(name string) error {
	wm.mutex.RLock()
	runner, runnerExists := wm.workflows[name]
	config, configExists := wm.configs[name]
	stats := wm.stats[name]
	wm.mutex.RUnlock()

	if !runnerExists {
		return fmt.Errorf("workflow %s not found", name)
	}
	if !configExists {
		return fmt.Errorf("configuration for workflow %s not found", name)
	}
	if !config.Enabled || !runner.IsEnabled() {
		return fmt.Errorf("workflow %s is disabled", name)
	}

	// Check if already running
	stats.mutex.Lock()
	if stats.IsRunning {
		stats.mutex.Unlock()
		return fmt.Errorf("workflow %s is already running", name)
	}
	stats.IsRunning = true
	stats.mutex.Unlock()

	wm.wg.Add(1)
	go func() {
		defer wm.wg.Done()
		defer func() {
			stats.mutex.Lock()
			stats.IsRunning = false
			stats.mutex.Unlock()
		}()

		wm.runWorkflowLoop(runner, config, stats)
	}()

	wm.info("Started workflow: %s\n", name)
	return nil
}

// runWorkflowLoop runs a single workflow in a continuous loop
func (wm *WorkflowManager) runWorkflowLoop(runner WorkflowRunner, config WorkflowConfig, stats *WorkflowStats) {
	consecutiveErrors := 0

	// Log initial start
	wm.info("┌─────────────────────────────────────────────────────────┐\n")
	wm.info("│ Workflow '%s' started (interval: %v)\n", runner.Name(), config.Interval)
	wm.info("└─────────────────────────────────────────────────────────┘\n")

	for {
		select {
		case <-wm.ctx.Done():
			wm.info("Workflow %s stopping due to shutdown signal\n", runner.Name())
			return
		default:
		}

		// Execute one cycle
		startTime := time.Now()

		stats.mutex.Lock()
		stats.TotalExecutions++
		executionNum := stats.TotalExecutions
		stats.LastExecution = startTime
		stats.mutex.Unlock()

		wm.debug("[%s] Starting execution cycle #%d", runner.Name(), executionNum)

		// Execute runner.Run() asynchronously to allow heartbeat during execution
		done := make(chan error, 1)
		go func() {
			done <- runner.Run(wm.ctx, config)
		}()

		// Create heartbeat ticker for task execution monitoring
		executionHeartbeat := time.NewTicker(5 * time.Second)
		executionComplete := false
		var err error

		// Monitor task execution with periodic heartbeat
		for !executionComplete {
			select {
			case err = <-done:
				executionComplete = true
			case <-executionHeartbeat.C:
				// Try to get current task info from state locks and repository
				taskInfo := wm.getCurrentTaskInfo(runner.Name())
				if taskInfo != nil {
					// Use title if available, otherwise use task ID
					taskDisplay := taskInfo.TaskID
					if taskInfo.Title != "" {
						taskDisplay = taskInfo.Title
					}

					if taskInfo.Step != "" && taskInfo.Turn > 0 {
						wm.info("💓 [%s] Processing task %s [%s] (turn #%d)...",
							runner.Name(), taskDisplay, taskInfo.Step, taskInfo.Turn)
					} else if taskInfo.Step != "" {
						wm.info("💓 [%s] Processing task %s [%s] (execution #%d)...",
							runner.Name(), taskDisplay, taskInfo.Step, executionNum)
					} else if taskInfo.Turn > 0 {
						wm.info("💓 [%s] Processing task %s (turn #%d)...",
							runner.Name(), taskDisplay, taskInfo.Turn)
					} else {
						wm.info("💓 [%s] Processing task %s (execution #%d)...",
							runner.Name(), taskDisplay, executionNum)
					}
				} else {
					wm.info("💓 [%s] Processing task (execution #%d)...", runner.Name(), executionNum)
				}
			case <-wm.ctx.Done():
				executionHeartbeat.Stop()
				wm.info("Workflow %s stopping due to shutdown signal\n", runner.Name())
				return
			}
		}
		executionHeartbeat.Stop()

		endTime := time.Now()
		duration := endTime.Sub(startTime)

		stats.mutex.Lock()
		if err != nil {
			stats.FailedRuns++
			stats.LastError = err
			consecutiveErrors++
			// Check if it's a lock contention error
			isLockError := strings.Contains(err.Error(), "another instance") ||
				strings.Contains(err.Error(), "another process is running") ||
				strings.Contains(err.Error(), "state.lock: file exists")

			if isLockError {
				// If this is the first execution and lock is held, exit immediately
				if executionNum == 1 {
					stats.mutex.Unlock()
					wm.warn("[%s] Cannot start: %v\n", runner.Name(), err)
					return
				}
				wm.debug("[%s] Execution #%d skipped: lock contention (another instance running)\n", runner.Name(), executionNum)
			} else {
				wm.warn("[%s] Execution #%d failed: %v\n", runner.Name(), executionNum, err)
			}
		} else {
			stats.SuccessfulRuns++
			consecutiveErrors = 0
			wm.info("[%s] Execution #%d completed successfully (took %v)\n",
				runner.Name(), executionNum, duration)
		}

		// Update average interval (simple moving average)
		if stats.AverageInterval == 0 {
			stats.AverageInterval = duration
		} else {
			stats.AverageInterval = (stats.AverageInterval + duration) / 2
		}
		stats.mutex.Unlock()

		// Calculate next interval with backoff
		interval := calculateNextInterval(config.Interval, consecutiveErrors)

		// Wait for next execution with periodic heartbeat
		wm.debug("[%s] Next execution in %v", runner.Name(), interval)

		// Create ticker for heartbeat during wait
		heartbeatTicker := time.NewTicker(5 * time.Second)
		waitTimer := time.NewTimer(interval)

		waitComplete := false
		for !waitComplete {
			select {
			case <-wm.ctx.Done():
				heartbeatTicker.Stop()
				waitTimer.Stop()
				return
			case <-waitTimer.C:
				waitComplete = true
			case <-heartbeatTicker.C:
				wm.info("💓 [%s] Workflow active - waiting for next cycle...", runner.Name())
			}
		}
		heartbeatTicker.Stop()
		waitTimer.Stop()
	}
}

// RunAll starts all enabled workflows
func (wm *WorkflowManager) RunAll() error {
	enabled := wm.GetEnabledWorkflows()
	if len(enabled) == 0 {
		return fmt.Errorf("no enabled workflows found")
	}

	wm.info("Starting %d enabled workflows: %v\n", len(enabled), enabled)

	for _, name := range enabled {
		if err := wm.RunWorkflow(name); err != nil {
			wm.warn("Failed to start workflow %s: %v\n", name, err)
		}
	}

	return nil
}

// Stop gracefully stops all running workflows
func (wm *WorkflowManager) Stop() {
	wm.info("Stopping all workflows...\n")
	wm.cancel()
	wm.wg.Wait()
	wm.info("All workflows stopped\n")
}

// GetStats returns statistics for all workflows
func (wm *WorkflowManager) GetStats() map[string]*WorkflowStats {
	wm.mutex.RLock()
	defer wm.mutex.RUnlock()

	result := make(map[string]*WorkflowStats)
	for name, stats := range wm.stats {
		// Create a copy to avoid race conditions
		stats.mutex.RLock()
		statsCopy := &WorkflowStats{
			Name:            stats.Name,
			TotalExecutions: stats.TotalExecutions,
			SuccessfulRuns:  stats.SuccessfulRuns,
			FailedRuns:      stats.FailedRuns,
			LastExecution:   stats.LastExecution,
			LastError:       stats.LastError,
			AverageInterval: stats.AverageInterval,
			IsRunning:       stats.IsRunning,
		}
		stats.mutex.RUnlock()
		result[name] = statsCopy
	}

	return result
}

// PrintStats prints statistics for all workflows
func (wm *WorkflowManager) PrintStats() {
	stats := wm.GetStats()

	wm.info("=== Workflow Manager Statistics ===\n")
	for name, stat := range stats {
		var successRate float64
		if stat.TotalExecutions > 0 {
			successRate = float64(stat.SuccessfulRuns) / float64(stat.TotalExecutions) * 100
		}

		status := "STOPPED"
		if stat.IsRunning {
			status = "RUNNING"
		}

		wm.info("Workflow: %s [%s]\n", name, status)
		wm.info("  Total executions: %d\n", stat.TotalExecutions)
		wm.info("  Success rate: %.1f%%\n", successRate)
		wm.info("  Failed runs: %d\n", stat.FailedRuns)
		if !stat.LastExecution.IsZero() {
			wm.info("  Last execution: %s\n", stat.LastExecution.Format("15:04:05"))
		}
		if stat.LastError != nil {
			wm.info("  Last error: %v\n", stat.LastError)
		}
		wm.info("  Average duration: %v\n", stat.AverageInterval)
		wm.info("\n")
	}
	wm.info("===================================\n")
}

// calculateNextInterval implements exponential backoff for consecutive errors
// This function is kept here to avoid circular dependencies with run_continuous.go
func calculateNextInterval(baseInterval time.Duration, consecutiveErrors int) time.Duration {
	if consecutiveErrors == 0 {
		return baseInterval
	}

	// Exponential backoff calculation moved inline to avoid import cycles
	backoff := baseInterval
	for i := 0; i < consecutiveErrors; i++ {
		backoff *= 2
	}

	maxBackoff := 10 * time.Second
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}
