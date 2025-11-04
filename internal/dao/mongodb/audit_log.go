package mongodb

import (
	"context"
	"partivo_tickets/internal/models"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func NewAuditLogDAO(db *mongo.Database, logger *zap.Logger) *AuditLogDAO {
	return &AuditLogDAO{
		collection: db.Collection(CollectionAuditLogs),
		logger:     logger.Named("AuditLogDAO"),
	}
}

type AuditLogDAO struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

func (d *AuditLogDAO) Create(ctx context.Context, log *models.AuditLog) error {
	_, err := d.collection.InsertOne(ctx, log)
	if err != nil {
		// We log this error but typically don't return it to the caller,
		// as audit logging failure should not fail the main business logic.
		d.logger.Error("Create: InsertOne failed", zap.Error(err))
	}
	return nil
}
