package output

import (
	"context"
)

// TransactionManager manages transactions across repositories
// This ensures ACID properties for multi-repository operations
type TransactionManager interface {
	// InTransaction executes a function within a transaction
	// If the function returns an error, the transaction is rolled back
	InTransaction(ctx context.Context, fn func(txCtx context.Context) error) error

	// BeginTransaction starts a new transaction
	BeginTransaction(ctx context.Context) (Transaction, error)
}

// Transaction represents an active transaction
type Transaction interface {
	// Commit commits the transaction
	Commit() error

	// Rollback rolls back the transaction
	Rollback() error

	// Context returns the transaction context
	Context() context.Context
}
