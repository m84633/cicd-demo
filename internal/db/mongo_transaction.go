package db

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
)

// MongoTransactionManager implements the TransactionManager for MongoDB.
type MongoTransactionManager struct {
	client *mongo.Client
}

// NewMongoTransactionManager creates a new MongoTransactionManager.
func NewMongoTransactionManager(client *mongo.Client) TransactionManager {
	return &MongoTransactionManager{client: client}
}

// WithTransaction executes the given function within a real MongoDB transaction.
func (m *MongoTransactionManager) WithTransaction(ctx context.Context, fn func(sessCtx context.Context) (interface{}, error)) (interface{}, error) {
	session, err := m.client.StartSession()
	if err != nil {
		return nil, err
	}
	defer session.EndSession(ctx)

	// The mongo driver's WithTransaction method expects a callback that accepts a mongo.SessionContext.
	// Our TransactionManager interface, however, defines a callback that accepts a standard context.Context
	// for better abstraction. Since mongo.SessionContext implements context.Context, we can create this
	// simple wrapper to make the function signatures compatible without changing the interface.
	callback := func(sessCtx mongo.SessionContext) (interface{}, error) {
		return fn(sessCtx)
	}

	return session.WithTransaction(ctx, callback)
}
