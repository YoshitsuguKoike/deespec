package workflow

import (
	"sync"
	"time"
)

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
