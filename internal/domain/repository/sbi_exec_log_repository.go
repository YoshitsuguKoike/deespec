package repository

import (
	"context"
	"time"
)

// SBIExecLog represents a single execution log entry for an SBI turn
type SBIExecLog struct {
	ID          int64
	SBIID       string
	Turn        int
	Step        string  // 'IMPLEMENT' or 'REVIEW'
	Decision    *string // NULL for IMPLEMENT, decision string for REVIEW
	ReportPath  string
	ExecutedAt  time.Time
	CreatedAt   time.Time
}

// SBIExecLogRepository defines the interface for SBI execution log persistence
type SBIExecLogRepository interface {
	// Save saves a new execution log entry
	Save(ctx context.Context, log *SBIExecLog) error

	// FindBySBIID retrieves all execution logs for a specific SBI, ordered by turn ASC
	FindBySBIID(ctx context.Context, sbiID string) ([]*SBIExecLog, error)

	// FindBySBIIDAndTurn retrieves a specific execution log entry
	FindBySBIIDAndTurn(ctx context.Context, sbiID string, turn int, step string) (*SBIExecLog, error)
}
