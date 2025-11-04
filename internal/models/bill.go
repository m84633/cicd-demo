package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Bill struct {
	ID                 primitive.ObjectID   `bson:"_id,omitempty"`
	Order              primitive.ObjectID   `bson:"order,omitempty"`
	OrderItemID        *primitive.ObjectID  `bson:"order_item_id,omitempty"` // For refund bills, to link to the specific item
	Customer           *User                `bson:"customer,omitempty"`
	Amount             primitive.Decimal128 `bson:"amount,omitempty"`
	Type               string               `bson:"type,omitempty"`
	Status             string               `bson:"status,omitempty"`
	PaymentMethod      *PaymentMethodInfo   `bson:"payment_method,omitempty"`
	CreatedAt          time.Time            `bson:"created_at"`
	UpdatedAt          time.Time            `bson:"updated_at"`
	Payment            *primitive.ObjectID  `bson:"payment,omitempty"`
	RefundingPaymentID *primitive.ObjectID  `bson:"refunding_payment_id,omitempty"` // For refund bills, to link to the original payment transaction
	Note               string               `bson:"note,omitempty"`
}

type PaymentMethodInfo struct {
	ID   primitive.ObjectID `bson:"id,omitempty"`
	Name string             `bson:"name,omitempty"`
}

// InvoiceInfo holds the information about the invoice related to a bill.
type InvoiceInfo struct {
	ID     string `bson:"id,omitempty"`     // The unique invoice number/ID from the invoice system
	Status string `bson:"status,omitempty"` // e.g., Pending, Issued, Failed
	URL    string `bson:"url,omitempty"`    // A link to the invoice PDF, if available
}
