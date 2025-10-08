package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/app"
	"github.com/YoshitsuguKoike/deespec/internal/domain/model/lock"
)

// WorkflowRunner defines the interface for workflow execution
type WorkflowRunner interface {
	// Name returns the workflow name (e.g., "sbi", "pbi")
	Name() string

	// Run executes one cycle of the workflow
	Run(ctx context.Context, config WorkflowConfig) error

	// IsEnabled checks if the workflow should be executed
	IsEnabled() bool

	// Description returns a human-readable description
	Description() string
}

// WorkflowConfig holds configuration for a specific workflow
type WorkflowConfig struct {
	Name      string                 `yaml:"name"`
	Enabled   bool                   `yaml:"enabled"`
	Interval  time.Duration          `yaml:"interval"`
	AutoFB    bool                   `yaml:"auto_fb"`
	ExtraArgs map[string]interface{} `yaml:"extra_args"`
}

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
}

// WorkflowStats tracks execution statistics for a specific workflow
type WorkflowStats struct {
	Name            string
	TotalExecutions int
	SuccessfulRuns  int
	FailedRuns      int
	LastExecution   time.Time
	LastError       error
	AverageInterval time.Duration
	IsRunning       bool
	mutex           sync.RWMutex
}

// ManagerConfig holds configuration for the workflow manager
type ManagerConfig struct {
	DefaultInterval time.Duration
	MaxConcurrent   int
	Workflows       []WorkflowConfig
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager() *WorkflowManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Improvement 3: Check and remove old locks at startup
	cleanupStaleLocks()

	return &WorkflowManager{
		workflows: make(map[string]WorkflowRunner),
		configs:   make(map[string]WorkflowConfig),
		stats:     make(map[string]*WorkflowStats),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// cleanupStaleLocks removes stale lock files at startup
func cleanupStaleLocks() {
	// Skip cleanup if no config available (e.g., in tests)
	if globalConfig == nil {
		return
	}

	// Get paths using config
	paths := app.GetPathsWithConfig(globalConfig)

	// Check for state.lock file
	if lockInfo, err := os.Stat(paths.StateLock); err == nil {
		// Lock file exists, check if it's stale
		shouldRemove := false
		removeReason := ""

		// Load state to check lease
		if st, loadErr := loadState(paths.State); loadErr == nil {
			if st.LeaseExpiresAt != "" && LeaseExpired(st) {
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
			Info("[Manager Startup] Removing stale lock: %s\n", removeReason)
			if err := os.Remove(paths.StateLock); err != nil {
				Warn("[Manager Startup] Failed to remove stale lock: %v\n", err)
			} else {
				Info("[Manager Startup] Successfully removed stale lock\n")
			}
		} else {
			Info("[Manager Startup] Active lock found, will respect it\n")
		}
	}

	// Also check for runlock using new Lock Service
	if container, err := initializeContainer(); err == nil {
		defer container.Close()
		lockService := container.GetLockService()
		ctx := context.Background()

		lockID, _ := lock.NewLockID("system-runlock")
		if runLock, err := lockService.FindRunLock(ctx, lockID); err == nil && runLock != nil {
			if runLock.IsExpired() {
				Info("[Manager Startup] Removing expired runlock from PID %d (expired: %s)\n",
					runLock.PID(), runLock.ExpiresAt())
				_ = lockService.ReleaseRunLock(ctx, lockID) // cleanup
			} else {
				Info("[Manager Startup] Valid runlock exists from PID %d (expires: %s)\n",
					runLock.PID(), runLock.ExpiresAt())
			}
		}
	}
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

	Info("Registered workflow: %s (%s)\n", name, runner.Description())
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

	Info("Started workflow: %s\n", name)
	return nil
}

// runWorkflowLoop runs a single workflow in a continuous loop
func (wm *WorkflowManager) runWorkflowLoop(runner WorkflowRunner, config WorkflowConfig, stats *WorkflowStats) {
	consecutiveErrors := 0

	// Log initial start
	Info("┌─────────────────────────────────────────────────────────┐\n")
	Info("│ Workflow '%s' started (interval: %v)\n", runner.Name(), config.Interval)
	Info("└─────────────────────────────────────────────────────────┘\n")

	for {
		select {
		case <-wm.ctx.Done():
			Info("Workflow %s stopping due to shutdown signal\n", runner.Name())
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

		Debug("[%s] Starting execution cycle #%d", runner.Name(), executionNum)

		err := runner.Run(wm.ctx, config)

		endTime := time.Now()
		duration := endTime.Sub(startTime)

		stats.mutex.Lock()
		if err != nil {
			stats.FailedRuns++
			stats.LastError = err
			consecutiveErrors++
			// Check if it's just a lock contention (not a real error)
			if strings.Contains(err.Error(), "another process is running") || strings.Contains(err.Error(), "state.lock: file exists") {
				Debug("[%s] Execution #%d skipped: lock contention (another instance running)\n", runner.Name(), executionNum)
			} else {
				Warn("[%s] Execution #%d failed: %v\n", runner.Name(), executionNum, err)
			}
		} else {
			stats.SuccessfulRuns++
			consecutiveErrors = 0
			Debug("[%s] Execution #%d completed successfully (took %v)",
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
		Debug("[%s] Next execution in %v", runner.Name(), interval)

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
				Info("💓 [%s] Workflow active - waiting for next cycle...", runner.Name())
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

	Info("Starting %d enabled workflows: %v\n", len(enabled), enabled)

	for _, name := range enabled {
		if err := wm.RunWorkflow(name); err != nil {
			Warn("Failed to start workflow %s: %v\n", name, err)
		}
	}

	return nil
}

// Stop gracefully stops all running workflows
func (wm *WorkflowManager) Stop() {
	Info("Stopping all workflows...\n")
	wm.cancel()
	wm.wg.Wait()
	Info("All workflows stopped\n")
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

	Info("=== Workflow Manager Statistics ===\n")
	for name, stat := range stats {
		var successRate float64
		if stat.TotalExecutions > 0 {
			successRate = float64(stat.SuccessfulRuns) / float64(stat.TotalExecutions) * 100
		}

		status := "STOPPED"
		if stat.IsRunning {
			status = "RUNNING"
		}

		Info("Workflow: %s [%s]\n", name, status)
		Info("  Total executions: %d\n", stat.TotalExecutions)
		Info("  Success rate: %.1f%%\n", successRate)
		Info("  Failed runs: %d\n", stat.FailedRuns)
		if !stat.LastExecution.IsZero() {
			Info("  Last execution: %s\n", stat.LastExecution.Format("15:04:05"))
		}
		if stat.LastError != nil {
			Info("  Last error: %v\n", stat.LastError)
		}
		Info("  Average duration: %v\n", stat.AverageInterval)
		Info("\n")
	}
	Info("===================================\n")
}
