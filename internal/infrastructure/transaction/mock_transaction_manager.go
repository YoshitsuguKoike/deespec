package transaction

import (
	"context"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// MockTransactionManager is a mock implementation of TransactionManager for Phase 4
// This will be replaced with actual SQLite transaction manager in Phase 5
type MockTransactionManager struct{}

// NewMockTransactionManager creates a new mock transaction manager
func NewMockTransactionManager() *MockTransactionManager {
	return &MockTransactionManager{}
}

// InTransaction executes a function within a "transaction"
// For mock implementation, this just executes the function directly
func (m *MockTransactionManager) InTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	// In mock mode, we don't actually manage transactions
	// Just execute the function with the same context
	return fn(ctx)
}

// BeginTransaction starts a new "transaction"
// Returns a mock transaction that does nothing
func (m *MockTransactionManager) BeginTransaction(ctx context.Context) (output.Transaction, error) {
	return &mockTransaction{ctx: ctx}, nil
}

// mockTransaction is a mock transaction implementation
type mockTransaction struct {
	ctx context.Context
}

// Commit does nothing in mock mode
func (t *mockTransaction) Commit() error {
	return nil
}

// Rollback does nothing in mock mode
func (t *mockTransaction) Rollback() error {
	return nil
}

// Context returns the transaction context
func (t *mockTransaction) Context() context.Context {
	return t.ctx
}
