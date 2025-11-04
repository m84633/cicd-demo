package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// 後面兩個狀態當作後續優化
const (
	OutboxStatusPending    = "PENDING"
	OutboxStatusProcessing = "PROCESSING"
	OutboxStatusProcessed  = "PROCESSED"
	OutboxStatusFailed     = "FAILED"
	OutboxStatusDeadLetter = "DEAD_LETTER"
)

type OutboxMessage struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Topic       string             `bson:"topic"`
	Payload     string             `bson:"payload"` // Storing payload as a JSON string
	Status      string             `bson:"status"`
	Retries     int                `bson:"retries"`
	ClaimID     primitive.ObjectID `bson:"claim_id,omitempty"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   *time.Time         `bson:"updated_at,omitempty"`
	ProcessedAt *time.Time         `bson:"processed_at,omitempty"`
	Error       string             `bson:"error,omitempty"`
}
