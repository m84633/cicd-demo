package repository

import (
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"partivo_tickets/internal/dao/fields"
	"partivo_tickets/internal/models"
	"time"
)

// ------------------- GetTicketsOptions -------------------

// GetTicketsOptions holds the optional parameters for the GetTickets query.
type GetTicketsOptions struct {
	Display   *bool
	IsShowing *bool
}

type GetTicketsOption func(*GetTicketsOptions)

// WithDisplay is an option to filter tickets by their visibility.
func WithDisplay(visible bool) GetTicketsOption {
	return func(opts *GetTicketsOptions) {
		opts.Display = &visible
	}
}

// WithIsShowing is an option to filter tickets by their showing status.
func WithIsShowing(isShowing bool) GetTicketsOption {
	return func(opts *GetTicketsOptions) {
		opts.IsShowing = &isShowing
	}
}

// ------------------- UpdateOptions -------------------

// UpdateOptions is an exported struct that holds the fields for a MongoDB update operation.
// It is used with the Functional Options pattern.
type UpdateOptions struct {
	SetFields bson.M
	IncFields bson.M
}

// NewUpdateOptions creates a new instance of UpdateOptions.
func NewUpdateOptions() *UpdateOptions {
	return &UpdateOptions{
		SetFields: bson.M{},
		IncFields: bson.M{},
	}
}

// UpdateOption defines a function that can modify the UpdateOptions.
type UpdateOption func(*UpdateOptions)

// WithStatus is an option to update the order's status field.
func WithStatus(status string) UpdateOption {
	return func(o *UpdateOptions) {
		o.SetFields[fields.FieldStatus] = status
	}
}

// WithItemsCount is an option to update the order's items_count field.
func WithItemsCount(count int) UpdateOption {
	return func(o *UpdateOptions) {
		o.SetFields[fields.FieldOrderItemsCount] = count
	}
}

// WithTotal is an option to update the order's total field.
func WithTotal(total primitive.Decimal128) UpdateOption {
	return func(o *UpdateOptions) {
		o.SetFields[fields.FieldOrderTotal] = total
	}
}

// WithIncTotal is an option to increment the order's total field by a given amount.
func WithIncTotal(amount primitive.Decimal128) UpdateOption {
	return func(o *UpdateOptions) {
		o.IncFields[fields.FieldOrderTotal] = amount
	}
}

// WithIncSubtotal is an option to increment the order's subtotal field by a given amount.
func WithIncSubtotal(amount primitive.Decimal128) UpdateOption {
	return func(o *UpdateOptions) {
		o.IncFields[fields.FieldOrderSubtotal] = amount
	}
}

// WithIncReturned is an option to increment the order's returned field by a given amount.
func WithIncReturned(amount primitive.Decimal128) UpdateOption {
	return func(o *UpdateOptions) {
		o.IncFields[fields.FieldOrderReturned] = amount
	}
}

// WithIncPaid is an option to increment the order's paid field by a given amount.
func WithIncPaid(amount primitive.Decimal128) UpdateOption {
	return func(o *UpdateOptions) {
		o.IncFields[fields.FieldOrderPaid] = amount
	}
}

// WithIncReturnedItemsCount is an option to increment the order's returned_items_count field by a given amount.
func WithIncReturnedItemsCount(count int) UpdateOption {
	return func(o *UpdateOptions) {
		o.IncFields[fields.FieldOrderReturnedItemsCount] = count
	}
}

// WithUpdatedBy is an option to update the order's updated_by field.
func WithUpdatedBy(user *models.User) UpdateOption {
	return func(o *UpdateOptions) {
		o.SetFields[fields.FieldOrderUpdatedBy] = user
	}
}

// WithPaymentID is an option to update the bill's payment ID field.
func WithPaymentID(paymentID *primitive.ObjectID) UpdateOption {
	return func(o *UpdateOptions) {
		o.SetFields[fields.FieldBillPayment] = paymentID
	}
}

// WithUpdatedAt is an option to update the updated_at field.
func WithUpdatedAt(t time.Time) UpdateOption {
	return func(o *UpdateOptions) {
		o.SetFields[fields.FieldUpdatedAt] = t
	}
}

func WithRefundingPaymentID(paymentID *primitive.ObjectID) UpdateOption {
	return func(o *UpdateOptions) {
		o.SetFields[fields.FieldBillRefundingPaymentID] = paymentID
	}
}
