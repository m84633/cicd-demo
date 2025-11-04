package mongodb

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"partivo_tickets/internal/conf"
)

// NewMongoDB creates a new MongoDB client and a cleanup function.
func NewMongoDB(cfg *conf.MongodbConfig) (*mongo.Client, func(), error) {
	var dsn string
	if cfg.User != "" {
		dsn = fmt.Sprintf("mongodb://%s:%s@%s:%d", cfg.User, cfg.Password, cfg.Host, cfg.Port)
	} else {
		dsn = fmt.Sprintf("mongodb://%s:%d", cfg.Host, cfg.Port)
	}

	clientOptions := options.Client().ApplyURI(dsn)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to mongodb: %w", err)
	}

	// Ping the primary
	if err := client.Ping(context.Background(), nil); err != nil {
		return nil, nil, fmt.Errorf("failed to ping mongodb: %w", err)
	}

	// Run migrations
	if err := Migrate(client, cfg); err != nil {
		return nil, nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	cleanup := func() {
		if err := client.Disconnect(context.Background()); err != nil {
			// Log this error, but don't panic
			fmt.Printf("failed to disconnect from mongodb: %v\n", err)
		}
	}

	return client, cleanup, nil
}
