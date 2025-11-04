package dto

import (
	"partivo_tickets/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func NewCreateBillRequest(orderID primitive.ObjectID, amount primitive.Decimal128, operator *models.User, payment *PaymentInfoRequest) *CreateBillRequest {
	return &CreateBillRequest{
		orderID:       orderID,
		amount:        amount,
		operator:      operator,
		paymentMethod: payment,
	}
}

func NewPaymentMethodInfoRequest(id primitive.ObjectID, name string) *PaymentInfoRequest {
	return &PaymentInfoRequest{
		id:   id,
		name: name,
	}
}

type PaymentInfoRequest struct {
	id   primitive.ObjectID
	name string
}

func (r *PaymentInfoRequest) ID() primitive.ObjectID {
	return r.id
}
func (r *PaymentInfoRequest) Name() string {
	return r.name
}

type CreateBillRequest struct {
	orderID       primitive.ObjectID
	amount        primitive.Decimal128
	operator      *models.User
	paymentMethod *PaymentInfoRequest
}

func (r CreateBillRequest) GetOrderID() primitive.ObjectID {
	return r.orderID
}

func (r CreateBillRequest) GetAmount() primitive.Decimal128 {
	return r.amount
}

func (r CreateBillRequest) GetOperator() *models.User {
	return r.operator
}

func (r CreateBillRequest) GetPaymentMethod() *PaymentInfoRequest { return r.paymentMethod }

// --- CancelBill DTOs ---

type CancelBillRequest struct {
	billID   primitive.ObjectID
	operator *models.User
	reason   string
}

func NewCancelBillRequest(billID primitive.ObjectID, operator *models.User, reason string) *CancelBillRequest {
	return &CancelBillRequest{
		billID:   billID,
		operator: operator,
		reason:   reason,
	}
}

func (r *CancelBillRequest) GetBillID() primitive.ObjectID {
	return r.billID
}

func (r *CancelBillRequest) GetOperator() *models.User {
	return r.operator
}

func (r *CancelBillRequest) GetReason() string {
	return r.reason
}

// --- CreateRefundBill DTOs ---

type CreateRefundBillRequest struct {
	orderItemID   primitive.ObjectID
	amount        primitive.Decimal128
	paymentMethod *PaymentInfoRequest
	reason        string
	operator      *models.User
}

func NewCreateRefundBillRequest(orderItemID primitive.ObjectID, amount primitive.Decimal128, paymentMethod *PaymentInfoRequest, reason string, operator *models.User) *CreateRefundBillRequest {
	return &CreateRefundBillRequest{
		orderItemID:   orderItemID,
		amount:        amount,
		paymentMethod: paymentMethod,
		reason:        reason,
		operator:      operator,
	}
}

func (r *CreateRefundBillRequest) GetOrderItemID() primitive.ObjectID {
	return r.orderItemID
}

func (r *CreateRefundBillRequest) GetAmount() primitive.Decimal128 {
	return r.amount
}

func (r *CreateRefundBillRequest) GetPaymentMethod() *PaymentInfoRequest {
	return r.paymentMethod
}

func (r *CreateRefundBillRequest) GetReason() string {
	return r.reason
}

func (r *CreateRefundBillRequest) GetOperator() *models.User {
	return r.operator
}