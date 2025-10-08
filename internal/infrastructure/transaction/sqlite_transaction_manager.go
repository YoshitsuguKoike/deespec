package transaction

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/YoshitsuguKoike/deespec/internal/application/port/output"
)

// SQLiteTransactionManager manages SQLite transactions
type SQLiteTransactionManager struct {
	db *sql.DB
}

// NewSQLiteTransactionManager creates a new SQLite transaction manager
func NewSQLiteTransactionManager(db *sql.DB) *SQLiteTransactionManager {
	return &SQLiteTransactionManager{db: db}
}

// InTransaction executes a function within a transaction
func (m *SQLiteTransactionManager) InTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	// Start transaction
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction failed: %w", err)
	}

	// Create transaction context
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Execute function
	err = fn(txCtx)
	if err != nil {
		// Rollback on error
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	// Commit on success
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

// BeginTransaction starts a new transaction
func (m *SQLiteTransactionManager) BeginTransaction(ctx context.Context) (output.Transaction, error) {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin transaction failed: %w", err)
	}

	return &sqliteTransaction{
		tx:  tx,
		ctx: context.WithValue(ctx, txKey{}, tx),
	}, nil
}

// txKey is used as a key for storing transaction in context
type txKey struct{}

// sqliteTransaction implements output.Transaction
type sqliteTransaction struct {
	tx  *sql.Tx
	ctx context.Context
}

// Commit commits the transaction
func (t *sqliteTransaction) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// Rollback rolls back the transaction
func (t *sqliteTransaction) Rollback() error {
	if err := t.tx.Rollback(); err != nil {
		return fmt.Errorf("rollback failed: %w", err)
	}
	return nil
}

// Context returns the transaction context
func (t *sqliteTransaction) Context() context.Context {
	return t.ctx
}

// GetTxFromContext retrieves a transaction from context
// This is a helper function for repositories to use
func GetTxFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(*sql.Tx)
	return tx, ok
}
