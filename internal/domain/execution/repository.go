package execution

// SBIExecutionRepository defines the interface for SBI execution persistence
type SBIExecutionRepository interface {
	// Save persists an SBI execution
	Save(execution *SBIExecution) error

	// FindByID retrieves an execution by its ID
	FindByID(id ExecutionID) (*SBIExecution, error)

	// FindBySBIID retrieves the latest execution for an SBI
	FindBySBIID(sbiID string) (*SBIExecution, error)

	// FindActive retrieves all active (not completed) executions
	FindActive() ([]*SBIExecution, error)

	// FindByStatus retrieves executions with a specific status
	FindByStatus(status ExecutionStatus) ([]*SBIExecution, error)

	// Update updates an existing execution
	Update(execution *SBIExecution) error

	// Delete removes an execution (for cleanup/testing)
	Delete(id ExecutionID) error
}
