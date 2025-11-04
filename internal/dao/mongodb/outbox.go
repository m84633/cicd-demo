package mongodb

import (
	"context"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

func NewOutboxDAO(client *mongo.Client, cfg *conf.MongodbConfig) *OutboxDAO {
	db := client.Database(cfg.DB)
	return &OutboxDAO{
		outboxCollection: db.Collection(CollectionOutbox),
	}
}

type OutboxDAO struct {
	outboxCollection *mongo.Collection
}

func (d *OutboxDAO) Create(ctx context.Context, message *models.OutboxMessage) error {
	_, err := d.outboxCollection.InsertOne(ctx, message)
	if err != nil {
		zap.L().Error("mongodb/outbox@Create: InsertOne", zap.Error(err))
		return err
	}
	return nil
}

// ClaimAndFetchEvents uses a three-phase approach to efficiently and atomically
// claim a batch of pending events. This is more performant than claiming one by one.
func (d *OutboxDAO) ClaimAndFetchEvents(ctx context.Context, limit int) ([]*models.OutboxMessage, error) {
	// Phase 1: Find the IDs of potential candidate events.
	// This is a lightweight query that only fetches IDs.
	findOptions := options.Find().
		SetSort(bson.D{{"created_at", 1}}). // Process oldest first
		SetLimit(int64(limit)).
		SetProjection(bson.M{"_id": 1})

	filter := bson.M{"status": models.OutboxStatusPending}
	cursor, err := d.outboxCollection.Find(ctx, filter, findOptions)
	if err != nil {
		zap.L().Error("mongodb/outbox@ClaimAndFetchEvents: Phase 1 Find failed", zap.Error(err))
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []struct {
		ID primitive.ObjectID `bson:"_id"`
	}
	if err = cursor.All(ctx, &results); err != nil {
		zap.L().Error("mongodb/outbox@ClaimAndFetchEvents: Phase 1 cursor decoding failed", zap.Error(err))
		return nil, err
	}

	// If no pending documents were found, return immediately.
	if len(results) == 0 {
		return []*models.OutboxMessage{}, nil
	}

	// Extract IDs into a slice for the next phase.
	ids := make([]primitive.ObjectID, len(results))
	for i, res := range results {
		ids[i] = res.ID
	}

	// Phase 2: Atomically "claim" the events by updating their status.
	// The `status: models.OutboxStatusPending` part of the filter acts as an
	// optimistic lock to prevent race conditions with other workers.
	claimID := primitive.NewObjectID() // A unique id for this batch
	updateFilter := bson.M{
		"_id":    bson.M{"$in": ids},
		"status": models.OutboxStatusPending,
	}
	update := bson.M{
		"$set": bson.M{
			"status":     models.OutboxStatusProcessing,
			"claim_id":   claimID,
			"updated_at": time.Now(),
		},
	}
	updateResult, err := d.outboxCollection.UpdateMany(ctx, updateFilter, update)
	if err != nil {
		zap.L().Error("mongodb/outbox@ClaimAndFetchEvents: Phase 2 UpdateMany failed", zap.Error(err))
		return nil, err
	}

	// If nothing was modified, it means another worker claimed these events
	// between our Phase 1 and Phase 2. This is not an error.
	if updateResult.ModifiedCount == 0 {
		return []*models.OutboxMessage{}, nil
	}

	// Phase 3: Fetch the full documents that we successfully claimed.
	fetchFilter := bson.M{"claim_id": claimID}
	claimedCursor, err := d.outboxCollection.Find(ctx, fetchFilter)
	if err != nil {
		zap.L().Error("mongodb/outbox@ClaimAndFetchEvents: Phase 3 Find failed", zap.Error(err))
		return nil, err
	}

	var claimedMessages []*models.OutboxMessage
	if err = claimedCursor.All(ctx, &claimedMessages); err != nil {
		zap.L().Error("mongodb/outbox@ClaimAndFetchEvents: Phase 3 cursor decoding failed", zap.Error(err))
		return nil, err
	}

	return claimedMessages, nil
}

func (d *OutboxDAO) MarkAsProcessed(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status":       models.OutboxStatusProcessed,
			"processed_at": time.Now(),
		},
	}
	_, err := d.outboxCollection.UpdateOne(ctx, filter, update)
	return err
}

func (d *OutboxDAO) IncrementRetry(ctx context.Context, id primitive.ObjectID, errorMessage string) error {
	filter := bson.M{"_id": id}
	update := bson.M{
		"$set": bson.M{
			"status": models.OutboxStatusPending, // Reset to pending for retry
			"error":  errorMessage,
		},
		"$inc": bson.M{"retries": 1},
	}
	_, err := d.outboxCollection.UpdateOne(ctx, filter, update)
	return err
}
