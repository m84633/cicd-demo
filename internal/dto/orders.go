package dto

import (
	"partivo_tickets/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// OrderWithItems is a DTO that combines an Order with its associated OrderItems.
// Through embedding models.Order, it can directly have all the fields of Order.
// It's used to return rich order information in a single structure.
type OrderWithItems struct {
	*models.Order `bson:",inline"`
	Items         []*models.OrderItem `json:"items" bson:"items"`
}

// OrderDetails is a DTO that combines an Order with its items and bills.
type OrderDetails struct {
	*models.Order
	Items []*models.OrderItem `json:"items"`
	Bills []*models.Bill      `json:"bills"`
}

// OrderItemWithOrder is a DTO that combines an OrderItem with its parent Order.
type OrderItemWithOrder struct {
	*models.OrderItem `bson:",inline"`
	OrderInfo         *models.Order `bson:"order_info"`
}

func NewAddOrderRequest(e, mid primitive.ObjectID, s, t string, ds []*OrderItem, u *models.User, pm *PaymentInfoRequest) *AddOrderRequest {
	return &AddOrderRequest{
		event:         e,
		subtotal:      s,
		total:         t,
		details:       ds,
		user:          u,
		merchantID:    mid,
		paymentMethod: pm,
	}
}

func NewOrderItem(t primitive.ObjectID, q uint32) *OrderItem {
	return &OrderItem{
		ticketStock: t,
		quantity:    q,
	}
}

type OrderItem struct {
	ticketStock primitive.ObjectID
	quantity    uint32
}

func (o *OrderItem) TicketStock() primitive.ObjectID {
	return o.ticketStock
}

func (o *OrderItem) Quantity() uint32 {
	return o.quantity
}

type AddOrderRequest struct {
	event         primitive.ObjectID
	subtotal      string
	total         string
	details       []*OrderItem
	user          *models.User
	merchantID    primitive.ObjectID
	paymentMethod *PaymentInfoRequest
}

func (r AddOrderRequest) GetEvent() primitive.ObjectID {
	return r.event
}

func (r AddOrderRequest) GetSubtotal() string {
	return r.subtotal
}

func (r AddOrderRequest) GetTotal() string {
	return r.total
}

func (r AddOrderRequest) GetDetails() []*OrderItem {
	return r.details
}

func (r AddOrderRequest) GetUser() *models.User {
	return r.user
}
func (r AddOrderRequest) GetMerchantID() primitive.ObjectID     { return r.merchantID }
func (r AddOrderRequest) GetPaymentMethod() *PaymentInfoRequest { return r.paymentMethod }

func NewChangeOrderStatusRequest(o primitive.ObjectID, s string, oper *models.User) *ChangeOrderStatusRequest {
	return &ChangeOrderStatusRequest{
		order:    o,
		status:   s,
		operator: oper,
	}
}

type ChangeOrderStatusRequest struct {
	order    primitive.ObjectID
	status   string
	operator *models.User
}

func (r ChangeOrderStatusRequest) GetOrder() primitive.ObjectID {
	return r.order
}

func (r ChangeOrderStatusRequest) GetStatus() string {
	return r.status
}

func (r ChangeOrderStatusRequest) GetOperator() *models.User {
	return r.operator
}

func NewUpdateOrderItemStatusRequest(orderItemID, orderID primitive.ObjectID, newStatus, reason string, operator *models.User) *UpdateOrderItemStatusRequest {
	return &UpdateOrderItemStatusRequest{
		orderItemID: orderItemID,
		orderID:     orderID,
		newStatus:   newStatus,
		operator:    operator,
		reason:      reason,
	}
}

type UpdateOrderItemStatusRequest struct {
	orderItemID primitive.ObjectID
	orderID     primitive.ObjectID
	newStatus   string
	operator    *models.User
	reason      string
}

func (r UpdateOrderItemStatusRequest) GetOrderItemID() primitive.ObjectID {
	return r.orderItemID
}

func (r UpdateOrderItemStatusRequest) GetOrderID() primitive.ObjectID {
	return r.orderID
}

func (r UpdateOrderItemStatusRequest) GetNewStatus() string {
	return r.newStatus
}

func (r UpdateOrderItemStatusRequest) GetOperator() *models.User {
	return r.operator
}
func (r UpdateOrderItemStatusRequest) GetReason() string { return r.reason }

// AddOrderResponse defines the data returned after successfully creating an order.
type AddOrderResponse struct {
	OrderID primitive.ObjectID `json:"order_id"`
	BillID  primitive.ObjectID `json:"bill_id"`
}

// CancelOrderRequest is a DTO for cancelling an order.
type CancelOrderRequest struct {
	orderID primitive.ObjectID
	user    *models.User
	reason  string
}

func NewCancelOrderRequest(orderID primitive.ObjectID, user *models.User, reason string) *CancelOrderRequest {
	return &CancelOrderRequest{
		orderID: orderID,
		user:    user,
		reason:  reason,
	}
}

func (r *CancelOrderRequest) GetOrderID() primitive.ObjectID {
	return r.orderID
}

func (r *CancelOrderRequest) GetUser() *models.User {
	return r.user
}

func (r *CancelOrderRequest) GetReason() string {
	return r.reason
}

func NewRequestOrderItemRefundRequest(orderID, orderItemID primitive.ObjectID, user *models.User, reason string) *RequestOrderItemRefundRequest {
	return &RequestOrderItemRefundRequest{
		orderID:     orderID,
		orderItemID: orderItemID,
		operator:    user,
		reason:      reason,
	}
}

type RequestOrderItemRefundRequest struct {
	orderID     primitive.ObjectID
	orderItemID primitive.ObjectID
	operator    *models.User
	reason      string
}

func (r *RequestOrderItemRefundRequest) GetOrderID() primitive.ObjectID {
	return r.orderID
}

func (r *RequestOrderItemRefundRequest) GetOrderItemID() primitive.ObjectID {
	return r.orderItemID
}

func (r *RequestOrderItemRefundRequest) GetOperator() *models.User {
	return r.operator
}

func (r *RequestOrderItemRefundRequest) GetReason() string {
	return r.reason
}

func NewRejectOrderItemRefundRequest(orderID, orderItemID primitive.ObjectID, user *models.User, reason string) *RejectOrderItemRefundRequest {
	return &RejectOrderItemRefundRequest{
		orderID:     orderID,
		orderItemID: orderItemID,
		operator:    user,
		reason:      reason,
	}
}

type RejectOrderItemRefundRequest struct {
	orderID     primitive.ObjectID
	orderItemID primitive.ObjectID
	operator    *models.User
	reason      string
}

func (r *RejectOrderItemRefundRequest) GetOrderID() primitive.ObjectID {
	return r.orderID
}

func (r *RejectOrderItemRefundRequest) GetOrderItemID() primitive.ObjectID {
	return r.orderItemID
}

func (r *RejectOrderItemRefundRequest) GetOperator() *models.User {
	return r.operator
}

func (r *RejectOrderItemRefundRequest) GetReason() string {
	return r.reason
}
