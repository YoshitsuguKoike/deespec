package service

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/YoshitsuguKoike/deespec/internal/application/dto"
)

const (
	DefaultRunInterval = 5 * time.Second
	MinInterval        = 1 * time.Second
	MaxInterval        = 10 * time.Minute
	MaxBackoff         = 10 * time.Second // Reduced from 5 minutes for lock contention visibility
)

// ContinuousExecutionService handles business logic for continuous execution
type ContinuousExecutionService struct {
	warnLog func(format string, args ...interface{})
}

// NewContinuousExecutionService creates a new continuous execution service
func NewContinuousExecutionService(warnLog func(format string, args ...interface{})) *ContinuousExecutionService {
	return &ContinuousExecutionService{
		warnLog: warnLog,
	}
}

// ParseInterval parses interval string with validation
func (s *ContinuousExecutionService) ParseInterval(intervalStr string) (time.Duration, error) {
	if intervalStr == "" {
		return DefaultRunInterval, nil
	}

	duration, err := time.ParseDuration(intervalStr)
	if err != nil {
		return 0, fmt.Errorf("invalid interval format: %v", err)
	}

	// Apply bounds with warnings
	if duration < MinInterval {
		if s.warnLog != nil {
			s.warnLog("Interval %v too small, using minimum %v\n", duration, MinInterval)
		}
		return MinInterval, nil
	}
	if duration > MaxInterval {
		if s.warnLog != nil {
			s.warnLog("Interval %v too large, using maximum %v\n", duration, MaxInterval)
		}
		return MaxInterval, nil
	}

	return duration, nil
}

// ClassifyError determines the type of error
func (s *ContinuousExecutionService) ClassifyError(err error) dto.ErrorClassification {
	if err == nil {
		return dto.ErrorUnknown
	}

	errStr := err.Error()

	// Check for temporary errors
	if s.isTemporaryError(errStr) {
		return dto.ErrorTemporary
	}

	// Check for configuration errors
	if s.isConfigurationError(errStr) {
		return dto.ErrorConfiguration
	}

	// Check for critical errors
	if s.isCriticalError(errStr) {
		return dto.ErrorCritical
	}

	return dto.ErrorUnknown
}

// HandleExecutionError processes execution errors and decides whether to continue
func (s *ContinuousExecutionService) HandleExecutionError(
	err error,
	stats *dto.ExecutionStatistics,
) *dto.ErrorHandlingResult {
	stats.LastError = err
	stats.LastErrorTime = time.Now()

	classification := s.ClassifyError(err)

	result := &dto.ErrorHandlingResult{
		Classification: classification,
	}

	switch classification {
	case dto.ErrorTemporary:
		stats.TemporaryErrors++
		result.ShouldContinue = true
		result.Message = fmt.Sprintf("Temporary error, will retry: %v", err)

	case dto.ErrorConfiguration:
		stats.ConfigErrors++
		result.ShouldContinue = true
		result.Message = fmt.Sprintf("Configuration error, please check settings: %v", err)

	case dto.ErrorCritical:
		stats.CriticalErrors++
		result.ShouldContinue = false
		result.Message = fmt.Sprintf("Critical error, stopping execution: %v", err)

	case dto.ErrorUnknown:
		stats.TemporaryErrors++
		result.ShouldContinue = true
		result.Message = fmt.Sprintf("Unknown error, continuing execution: %v", err)
	}

	return result
}

// CalculateNextInterval implements exponential backoff for consecutive errors
func (s *ContinuousExecutionService) CalculateNextInterval(
	baseInterval time.Duration,
	consecutiveErrors int,
) time.Duration {
	if consecutiveErrors == 0 {
		return baseInterval
	}

	// Exponential backoff with max
	backoff := time.Duration(math.Pow(2, float64(consecutiveErrors))) * baseInterval

	if backoff > MaxBackoff {
		return MaxBackoff
	}
	return backoff
}

// FormatStatistics formats execution statistics for reporting
func (s *ContinuousExecutionService) FormatStatistics(stats *dto.ExecutionStatistics) string {
	if stats.TotalExecutions == 0 {
		return ""
	}

	successRate := float64(stats.SuccessfulRuns) / float64(stats.TotalExecutions) * 100

	result := "=== Execution Statistics ===\n"
	result += fmt.Sprintf("  Total executions: %d\n", stats.TotalExecutions)
	result += fmt.Sprintf("  Success rate: %.1f%%\n", successRate)
	result += fmt.Sprintf("  Temporary errors: %d\n", stats.TemporaryErrors)
	result += fmt.Sprintf("  Config errors: %d\n", stats.ConfigErrors)
	result += fmt.Sprintf("  Critical errors: %d\n", stats.CriticalErrors)

	if stats.LastError != nil {
		result += fmt.Sprintf("  Last error: %v (at %s)\n",
			stats.LastError, stats.LastErrorTime.Format("15:04:05"))
	}

	result += "===========================\n"
	return result
}

// FormatSchedule formats the next execution schedule
func (s *ContinuousExecutionService) FormatSchedule(interval time.Duration) string {
	next := time.Now().Add(interval)
	return fmt.Sprintf("Next execution scheduled at: %s (in %v)",
		next.Format("15:04:05"), interval)
}

// isTemporaryError checks if error is temporary and execution should continue
func (s *ContinuousExecutionService) isTemporaryError(errStr string) bool {
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "no such file")
}

// isConfigurationError checks if error is due to configuration issues
func (s *ContinuousExecutionService) isConfigurationError(errStr string) bool {
	return strings.Contains(errStr, "config") ||
		strings.Contains(errStr, "invalid flag") ||
		strings.Contains(errStr, "missing required")
}

// isCriticalError checks if error is critical and execution should stop
func (s *ContinuousExecutionService) isCriticalError(errStr string) bool {
	return strings.Contains(errStr, "out of memory") ||
		strings.Contains(errStr, "disk full") ||
		strings.Contains(errStr, "corrupted")
}
