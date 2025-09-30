package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// RunConfig holds configuration for continuous execution
type RunConfig struct {
	AutoFB   bool
	Interval time.Duration
}

// ErrorStats tracks execution statistics
type ErrorStats struct {
	TotalExecutions int
	SuccessfulRuns  int
	TemporaryErrors int
	ConfigErrors    int
	CriticalErrors  int
	LastError       error
	LastErrorTime   time.Time
}

const (
	DefaultRunInterval = 5 * time.Second
	MinInterval        = 1 * time.Second
	MaxInterval        = 10 * time.Minute
)

// setupSignalHandler sets up graceful shutdown on SIGINT/SIGTERM
func setupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		os.Interrupt,    // Ctrl+C (SIGINT)
		syscall.SIGTERM, // kill command
	)

	go func() {
		sig := <-sigChan
		Info("Received signal: %v, initiating graceful shutdown...\n", sig)
		cancel()
	}()

	return ctx, cancel
}

// parseInterval parses interval string with validation
func parseInterval(intervalStr string) (time.Duration, error) {
	if intervalStr == "" {
		return DefaultRunInterval, nil
	}

	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		return 0, fmt.Errorf("invalid interval format: %v", err)
	}

	// Apply bounds
	if duration < MinInterval {
		Warn("Interval %v too small, using minimum %v\n", duration, MinInterval)
		return MinInterval, nil
	}
	if duration > MaxInterval {
		Warn("Interval %v too large, using maximum %v\n", duration, MaxInterval)
		return MaxInterval, nil
	}

	return duration, nil
}

// isTemporaryError checks if error is temporary and execution should continue
func isTemporaryError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "no such file")
}

// isConfigurationError checks if error is due to configuration issues
func isConfigurationError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "config") ||
		strings.Contains(errStr, "invalid flag") ||
		strings.Contains(errStr, "missing required")
}

// isCriticalError checks if error is critical and execution should stop
func isCriticalError(err error) bool {
	errStr := err.Error()
	return strings.Contains(errStr, "out of memory") ||
		strings.Contains(errStr, "disk full") ||
		strings.Contains(errStr, "corrupted")
}

// handleExecutionError processes execution errors and decides whether to continue
func handleExecutionError(err error, stats *ErrorStats) bool {
	stats.LastError = err
	stats.LastErrorTime = time.Now()

	switch {
	case isTemporaryError(err):
		stats.TemporaryErrors++
		Warn("Temporary error, will retry: %v\n", err)
		return true // continue execution

	case isConfigurationError(err):
		stats.ConfigErrors++
		Warn("Configuration error, please check settings: %v\n", err)
		return true // continue execution (allow user to fix config)

	case isCriticalError(err):
		stats.CriticalErrors++
		Error("Critical error, stopping execution: %v\n", err)
		return false // stop execution

	default:
		stats.TemporaryErrors++
		Warn("Unknown error, continuing execution: %v\n", err)
		return true // continue execution
	}
}

// calculateNextInterval implements exponential backoff for consecutive errors
func calculateNextInterval(baseInterval time.Duration, consecutiveErrors int) time.Duration {
	if consecutiveErrors == 0 {
		return baseInterval
	}

	// Exponential backoff with max 5 minutes
	backoff := time.Duration(math.Pow(2, float64(consecutiveErrors))) * baseInterval
	maxBackoff := 5 * time.Minute

	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}

// Report prints execution statistics
func (stats *ErrorStats) Report() {
	if stats.TotalExecutions == 0 {
		return
	}

	successRate := float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions) * 100
	Info("=== Execution Statistics ===\n")
	Info("  Total executions: %d\n", stats.TotalExecutions)
	Info("  Success rate: %.1f%%\n", successRate)
	Info("  Temporary errors: %d\n", stats.TemporaryErrors)
	Info("  Config errors: %d\n", stats.ConfigErrors)
	Info("  Critical errors: %d\n", stats.CriticalErrors)
	if stats.LastError != nil {
		Info("  Last error: %v (at %s)\n", stats.LastError, stats.LastErrorTime.Format("15:04:05"))
	}
	Info("===========================\n")
}

// logExecutionSchedule logs the next execution time
func logExecutionSchedule(interval time.Duration) {
	next := time.Now().Add(interval)
	Info("Next execution scheduled at: %s (in %v)\n",
		next.Format("15:04:05"), interval)
}

// runContinuous executes runOnce in a loop with proper error handling and signal support
func runContinuous(ctx context.Context, config RunConfig) error {
	stats := &ErrorStats{}
	consecutiveErrors := 0

	Info("Starting continuous execution (interval: %v, auto-fb: %v)\n",
		config.Interval, config.AutoFB)

	for {
		// Log execution schedule
		if stats.TotalExecutions > 0 {
			interval := calculateNextInterval(config.Interval, consecutiveErrors)
			logExecutionSchedule(interval)

			// Wait for next execution
			select {
			case <-ctx.Done():
				Info("Shutdown requested after %d executions\n", stats.TotalExecutions)
				stats.Report()
				return ctx.Err()
			case <-time.After(interval):
				// Continue to next execution
			}
		}

		// Execute one cycle
		Info("Starting execution #%d...\n", stats.TotalExecutions+1)
		stats.TotalExecutions++

		err := runOnce(config.AutoFB)

		if err != nil {
			consecutiveErrors++
			if !handleExecutionError(err, stats) {
				return err // Critical error, stop execution
			}
		} else {
			consecutiveErrors = 0
			stats.SuccessfulRuns++
			Info("Execution #%d completed successfully\n", stats.TotalExecutions)
		}

		// Report statistics every 10 executions
		if stats.TotalExecutions%10 == 0 {
			stats.Report()
		}

		// Check for shutdown before continuing
		select {
		case <-ctx.Done():
			Info("Shutdown requested after %d executions\n", stats.TotalExecutions)
			stats.Report()
			return ctx.Err()
		default:
			// Continue
		}
	}
}
