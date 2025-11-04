package repository

import (
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// --- Parameter Structs ---

type GetOrdersByUserParams struct {
	UserID          primitive.ObjectID
	Limit           int64
	CursorCreatedAt time.Time
	CursorID        primitive.ObjectID
}

type UpdateOrderStatusParams struct {
	OrderID primitive.ObjectID
	Status  string
	User    *models.User
}

type GetOrdersBySessionParams struct {
	SessionID primitive.ObjectID
	Limit     int
	Offset    int
}

type GetOrdersByEventParams struct {
	EventID primitive.ObjectID
	Limit   int
	Offset  int
}

type UpdateOrderItemStatusParams struct {
	ItemID     primitive.ObjectID
	Status     string
	ReturnInfo *models.ReturnInfo
}
