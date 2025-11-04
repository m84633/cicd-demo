package db

import "context"

// TransactionManager defines the interface for running operations in a transaction.
type TransactionManager interface {
	WithTransaction(ctx context.Context, fn func(sessCtx context.Context) (interface{}, error)) (interface{}, error)
}
