package run

import (
	"github.com/YoshitsuguKoike/deespec/internal/interface/cli/common"
)

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
	"github.com/YoshitsuguKoike/deespec/internal/application/service"
)

// RunConfig holds configuration for continuous execution
type RunConfig struct {
	AutoFB   bool
	Interval time.Duration
}

// SetupSignalHandler sets up graceful shutdown on SIGINT/SIGTERM/SIGTSTP
func SetupSignalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan,
		os.Interrupt,     // Ctrl+C (SIGINT)
		syscall.SIGTERM,  // kill command
		syscall.SIGTSTP,  // Ctrl+Z
	)

	go func() {
		sig := <-sigChan

		// Show specific message for Ctrl+Z
		if sig == syscall.SIGTSTP {
			common.Warn("\n⚠️  Ctrl+Z detected. Initiating graceful shutdown...\n")
			common.Info("💡 Tip: Use Ctrl+C to stop the process in the future.\n")
		} else {
			common.Info("Received signal: %v, initiating graceful shutdown...\n", sig)
		}

		cancel()
	}()

	return ctx, cancel
}

// ParseInterval parses interval string with validation (delegates to service)
func ParseInterval(intervalStr string) (time.Duration, error) {
	svc := service.NewContinuousExecutionService(common.Warn)
	return svc.ParseInterval(intervalStr)
}

// RunContinuous executes runOnce in a loop with proper error handling and signal support
func RunContinuous(ctx context.Context, config RunConfig) error {
	// Create service
	svc := service.NewContinuousExecutionService(common.Warn)

	// Initialize statistics
	stats := &dto.ExecutionStatistics{}
	consecutiveErrors := 0

	common.Info("Starting continuous execution (interval: %v, auto-fb: %v)\n",
		config.Interval, config.AutoFB)

	for {
		// Log execution schedule
		if stats.TotalExecutions > 0 {
			interval := svc.CalculateNextInterval(config.Interval, consecutiveErrors)
			common.Info("%s\n", svc.FormatSchedule(interval))

			// Wait for next execution
			select {
			case <-ctx.Done():
				common.Info("Shutdown requested after %d executions\n", stats.TotalExecutions)
				if report := svc.FormatStatistics(stats); report != "" {
					common.Info("%s", report)
				}
				return ctx.Err()
			case <-time.After(interval):
				// Continue to next execution
			}
		}

		// Execute one cycle
		common.Info("Starting execution #%d...\n", stats.TotalExecutions+1)
		stats.TotalExecutions++

		err := RunTurn(config.AutoFB)

		if err != nil {
			consecutiveErrors++
			result := svc.HandleExecutionError(err, stats)

			// Log appropriate message based on classification
			switch result.Classification {
			case dto.ErrorCritical:
				common.Error("%s\n", result.Message)
			default:
				common.Warn("%s\n", result.Message)
			}

			if !result.ShouldContinue {
				return err // Critical error, stop execution
			}
		} else {
			consecutiveErrors = 0
			stats.SuccessfulRuns++
			common.Info("Execution #%d completed successfully\n", stats.TotalExecutions)
		}

		// Report statistics every 10 executions
		if stats.TotalExecutions%10 == 0 {
			if report := svc.FormatStatistics(stats); report != "" {
				common.Info("%s", report)
			}
		}

		// Check for shutdown before continuing
		select {
		case <-ctx.Done():
			common.Info("Shutdown requested after %d executions\n", stats.TotalExecutions)
			if report := svc.FormatStatistics(stats); report != "" {
				common.Info("%s", report)
			}
			return ctx.Err()
		default:
			// Continue
		}
	}
}
