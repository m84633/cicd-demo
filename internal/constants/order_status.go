package constants

type OrderStatus int
type OrderItemStatus int

const (
	OrderStatusUnknown OrderStatus = iota
	OrderStatusUnpaid
	OrderStatusPartiallyPaid
	OrderStatusPendingRefund
	OrderStatusPaid
	OrderStatusCanceled
	OrderStatusCompleted
	OrderStatusPartiallyReturned
	OrderStatusReturned
	OrderStatusReturnInProgress
)

const (
	OrderItemStatusUnknown OrderItemStatus = iota
	OrderItemStatusPendingPayment
	OrderItemStatusPaid
	OrderItemStatusFulfilled
	OrderItemStatusReturnRequested
	OrderItemStatusReturned
	OrderItemStatusCanceled
	OrderItemStatusReturnApproved
)

func (s OrderStatus) String() string {
	switch s {
	case OrderStatusCanceled:
		return "canceled"
	case OrderStatusUnpaid:
		return "unpaid"
	case OrderStatusPartiallyPaid:
		return "partially_paid"
	case OrderStatusPendingRefund:
		return "pending_refund"
	case OrderStatusPaid:
		return "paid"
	case OrderStatusCompleted:
		return "completed"
	case OrderStatusPartiallyReturned:
		return "partially_returned"
	case OrderStatusReturnInProgress:
		return "return_in_progress"
	case OrderStatusReturned:
		return "returned"
	default:
		return "unknown"
	}
}

var orderStatusMap = map[string]OrderStatus{
	"unpaid":             OrderStatusUnpaid,
	"partially_paid":     OrderStatusPartiallyPaid,
	"paid":               OrderStatusPaid,
	"partially_returned": OrderStatusPartiallyReturned,
	"returned":           OrderStatusReturned,
	"completed":          OrderStatusCompleted,
	"canceled":           OrderStatusCanceled,
	"return_in_progress": OrderStatusReturnInProgress,
	"pending_refund":     OrderStatusPendingRefund,
	"unknown":            OrderStatusUnknown,
}

func ParseOrderStatus(s string) OrderStatus {
	if status, ok := orderStatusMap[s]; ok {
		return status
	}
	return OrderStatusUnknown
}

func (s OrderItemStatus) String() string {
	switch s {
	case OrderItemStatusPendingPayment:
		return "pending_payment"
	case OrderItemStatusPaid:
		return "paid"
	case OrderItemStatusFulfilled:
		return "delivered"
	case OrderItemStatusReturnRequested:
		return "return_requested"
	case OrderItemStatusReturned:
		return "returned"
	case OrderItemStatusCanceled:
		return "canceled"
	case OrderItemStatusReturnApproved:
		return "return_approved"
	default:
		return "unknown"
	}
}

var orderItemStatusMap = map[string]OrderItemStatus{
	"pending_payment":  OrderItemStatusPendingPayment,
	"paid":             OrderItemStatusPaid,
	"fulfilled":        OrderItemStatusFulfilled,
	"return_requested": OrderItemStatusReturnRequested,
	"returned":         OrderItemStatusReturned,
	"canceled":         OrderItemStatusCanceled,
	"unknown":          OrderItemStatusUnknown,
	"return_approved":  OrderItemStatusReturnApproved,
	"delivered":        OrderItemStatusFulfilled,
}

func ParseOrderItemStatus(s string) OrderItemStatus {
	if status, ok := orderItemStatusMap[s]; ok {
		return status
	}
	return OrderItemStatusUnknown
}
