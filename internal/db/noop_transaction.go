package db

import "context"

// NoOpTransactionManager is a transaction manager that does nothing.
// It's useful for development and testing environments where transactions are not needed.
type NoOpTransactionManager struct{}

// NewNoOpTransactionManager creates a new NoOpTransactionManager.
func NewNoOpTransactionManager() TransactionManager {
	return &NoOpTransactionManager{}
}

// WithTransaction simply executes the function without a real transaction.
func (n *NoOpTransactionManager) WithTransaction(ctx context.Context, fn func(sessCtx context.Context) (interface{}, error)) (interface{}, error) {
	// Directly execute the function, passing the original context.
	return fn(ctx)
}
