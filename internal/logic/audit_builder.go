package logic

import (
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AuditLogOption defines a function that configures an AuditLog object.
type AuditLogOption func(*models.AuditLog)

// WithReason is an option to add a reason to an audit log.
func WithReason(reason string) AuditLogOption {
	return func(log *models.AuditLog) {
		if reason != "" {
			log.Reason = reason
		}
	}
}

// NewAuditLog is a shared constructor for creating standardized audit log objects using the Option Pattern.
func NewAuditLog(user *models.User, action, entityType string, entityID primitive.ObjectID, before, after interface{}, opts ...AuditLogOption) *models.AuditLog {
	log := &models.AuditLog{
		ID:         primitive.NewObjectID(),
		UserID:     user.UserId,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Changes: map[string]interface{}{
			"before": before,
			"after":  after,
		},
		Timestamp: time.Now(),
	}

	// Apply all the options
	for _, opt := range opts {
		opt(log)
	}

	return log
}

// buildCreateOrderAuditLog uses NewAuditLog to create an audit object for a new order.
func buildCreateOrderAuditLog(orderModel *models.Order, orderItems []*models.OrderItem) *models.AuditLog {
	// This call remains compatible with the new NewAuditLog signature.
	auditLog := NewAuditLog(orderModel.User, "CREATE_ORDER", "order", orderModel.ID, nil, orderModel)

	// Prepare stock change information
	stockChanges := make([]map[string]interface{}, len(orderItems))
	for i, item := range orderItems {
		stockChanges[i] = map[string]interface{}{
			"ticket_id":   item.TicketStock,
			"ticket_name": item.Name,
			"quantity":    -item.Quantity,
		}
	}

	// Restructure the Changes field
	auditLog.Changes = map[string]interface{}{
		"order_details": auditLog.Changes,
		"stock_changes": stockChanges,
	}

	return auditLog
}

// buildUpdateOrderStatusAuditLog creates an audit object for an order status change.
func buildUpdateOrderStatusAuditLog(operator *models.User, before, after *models.Order) *models.AuditLog {
	// This call remains compatible.
	return NewAuditLog(operator, "UPDATE_ORDER_STATUS", "order", before.ID, before, after)
}

// buildUpdateOrderItemStatusAuditLog creates an audit object for an order item status change.
func buildUpdateOrderItemStatusAuditLog(operator *models.User, reason string, before *models.OrderItem, newStatus string) *models.AuditLog {
	after := *before         // Create a shallow copy
	after.Status = newStatus // Update the status for the "after" state

	// Use the new Option Pattern to include the reason.
	return NewAuditLog(operator, "UPDATE_ORDER_ITEM_STATUS", "order_item", before.ID, before, &after, WithReason(reason))
}

// buildAddTicketTypeAuditLog creates an audit object for a new ticket type.
func buildAddTicketTypeAuditLog(operator *models.User, ticketType *models.TicketType) *models.AuditLog {
	// This call remains compatible.
	return NewAuditLog(operator, "CREATE_TICKET_TYPE", "ticket_type", ticketType.ID, nil, ticketType)
}

// buildDeleteTicketTypeAuditLog creates an audit object for deleting a ticket type.
func buildDeleteTicketTypeAuditLog(operator *models.User, deletedTicket *models.TicketType) *models.AuditLog {
	// This call remains compatible.
	return NewAuditLog(operator, "DELETE_TICKET_TYPE", "ticket_type", deletedTicket.ID, deletedTicket, nil)
}

// buildUpdateTicketTypeAuditLog creates an audit object for updating a ticket type.
func buildUpdateTicketTypeAuditLog(operator *models.User, before, after *models.TicketType) *models.AuditLog {
	// This call remains compatible.
	return NewAuditLog(operator, "UPDATE_TICKET_TYPE", "ticket_type", before.ID, before, after)
}

// buildUpdateTicketTypesOrderAuditLog creates an audit object for reordering ticket types.
func buildUpdateTicketTypesOrderAuditLog(operator *models.User, eventID primitive.ObjectID, before, after []primitive.ObjectID) *models.AuditLog {
	// This call remains compatible.
	return NewAuditLog(operator, "UPDATE_TICKET_TYPES_ORDER", "ticket_type", eventID,
		map[string]interface{}{"ordered_ids": before},
		map[string]interface{}{"ordered_ids": after},
	)
}

// buildCheckInTicketAuditLog creates an audit log entry for a ticket check-in event.
func buildCheckInTicketAuditLog(operator *models.User, before, after *models.Ticket) *models.AuditLog {
	return NewAuditLog(operator, "CHECK_IN_TICKET", "ticket", before.ID, before, after)
}

// buildOrderStatusChangeAuditLog creates an audit object for an order status change triggered by recalculateOrderStatus.
func buildOrderStatusChangeAuditLog(operator *models.User, orderID primitive.ObjectID, oldStatus, newStatus, reason string) *models.AuditLog {
	before := map[string]interface{}{"status": oldStatus}
	after := map[string]interface{}{"status": newStatus}
	
	return NewAuditLog(operator, "UPDATE_ORDER_STATUS", "order", orderID, before, after, WithReason(reason))
}

// buildCancelOrderAuditLog creates an audit object for an order cancellation.
func buildCancelOrderAuditLog(operator *models.User, before *models.Order, reason string) *models.AuditLog {
	after := *before // Create a shallow copy
	after.Status = constants.OrderStatusCanceled.String() // Update the status for the "after" state

	// Use the Option Pattern to include the reason.
	return NewAuditLog(operator, "CANCEL_ORDER", "order", before.ID, before, &after, WithReason(reason))
}
