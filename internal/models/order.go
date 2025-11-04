package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Order struct {
	ID                 primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	Event              primitive.ObjectID   `bson:"event" json:"event"`
	Session            primitive.ObjectID   `bson:"session"`
	Serial             uint64               `bson:"serial" json:"serial"`
	Subtotal           primitive.Decimal128 `bson:"subtotal" json:"subtotal"`
	Total              primitive.Decimal128 `bson:"total" json:"total"`
	OriginalTotal      primitive.Decimal128 `bson:"original_total" json:"original_total"`
	User               *User                `bson:"user" json:"user"`
	Status             string               `bson:"status" json:"status"`
	Note               string               `bson:"note" json:"note"`
	CreatedAt          time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt          time.Time            `bson:"updated_at" json:"updated_at"`
	UpdatedBy          *User                `bson:"updated_by,omitempty" json:"updated_by,omitempty"`
	Paid               primitive.Decimal128 `bson:"paid" json:"paid"`
	Returned           primitive.Decimal128 `bson:"returned" json:"returned"`
	ItemsCount         int                  `bson:"items_count" json:"items_count"`
	ReturnedItemsCount int                  `bson:"returned_items_count" json:"returned_items_count"`
	MerchantID         primitive.ObjectID   `bson:"merchant_id" json:"merchant_id"`
	Invoice            *InvoiceInfo         `bson:"invoice,omitempty"`
}

type ReturnInfo struct {
	Reason      string    `bson:"reason" json:"reason"`
	RequestedAt time.Time `bson:"requested_at" json:"requested_at"`
}

type OrderItem struct {
	ID          primitive.ObjectID   `bson:"_id,omitempty" json:"id"`
	Order       primitive.ObjectID   `bson:"order" json:"order"`
	TicketStock primitive.ObjectID   `bson:"ticket_stock" json:"ticket_stock"`
	Session     primitive.ObjectID   `bson:"session" json:"session"`
	Name        string               `bson:"name" json:"name"`
	Price       primitive.Decimal128 `bson:"price" json:"price"`
	Quantity    uint32               `bson:"quantity" json:"quantity"`
	Status      string               `bson:"status" json:"status"`
	CreatedAt   time.Time            `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time            `bson:"updated_at" json:"updated_at"`
	UpdatedBy   *User                `bson:"updated_by,omitempty" json:"updated_by,omitempty"`
	ReturnInfo  *ReturnInfo          `bson:"return_info,omitempty" json:"return_info,omitempty"` //只有申請退費時才會需要
}
