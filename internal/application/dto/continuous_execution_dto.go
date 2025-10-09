package dto

import "time"

// ContinuousRunConfig holds configuration for continuous execution
type ContinuousRunConfig struct {
	AutoFB   bool
	Interval time.Duration
}

// ExecutionStatistics tracks execution statistics
type ExecutionStatistics struct {
	TotalExecutions int
	SuccessfulRuns  int
	TemporaryErrors int
	ConfigErrors    int
	CriticalErrors  int
	LastError       error
	LastErrorTime   time.Time
}

// ErrorClassification represents the type of error
type ErrorClassification string

const (
	ErrorTemporary     ErrorClassification = "temporary"
	ErrorConfiguration ErrorClassification = "configuration"
	ErrorCritical      ErrorClassification = "critical"
	ErrorUnknown       ErrorClassification = "unknown"
)

// ErrorHandlingResult contains the result of error handling
type ErrorHandlingResult struct {
	Classification ErrorClassification
	ShouldContinue bool // Whether execution should continue
	Message        string
}
