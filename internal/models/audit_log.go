package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AuditLog struct {
	ID         primitive.ObjectID     `bson:"_id,omitempty"`
	UserID     primitive.ObjectID     `bson:"user_id"`
	Action     string                 `bson:"action"`
	EntityType string                 `bson:"entity_type"`
	EntityID   primitive.ObjectID     `bson:"entity_id"`
	Changes    map[string]interface{} `bson:"changes"`
	Reason     string                 `bson:"reason,omitempty"`
	Timestamp  time.Time              `bson:"timestamp"`
}
