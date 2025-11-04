package repository

import (
	"context"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TicketsRepository interface {
	AddTicketType(ctx context.Context, t *models.TicketType) (primitive.ObjectID, error)
	AddTicketStock(ctx context.Context, t *models.TicketStock) error
	GetTicketTypesWithStockByEvent(ctx context.Context, eid primitive.ObjectID, opts ...GetTicketsOption) ([]dto.TicketTypeWithStock, error)
	DeleteTicketTypeByID(ctx context.Context, tcid primitive.ObjectID) (*models.TicketType, error)
	DeleteTicketStockByType(ctx context.Context, tcid primitive.ObjectID) error
	UpdateTicketType(ctx context.Context, t *models.TicketType) error
	UpdateTicketTypesOrder(ctx context.Context, eid primitive.ObjectID, ids []primitive.ObjectID, oper *models.User) error
	GetTicketTypeByID(ctx context.Context, tid primitive.ObjectID) (*models.TicketType, error)
	GetValidTicketsByIDs(ctx context.Context, ids []primitive.ObjectID) ([]dto.TicketStockWithType, error)
	ReleaseTickets(ctx context.Context, items []*models.OrderItem) error
	ReserveTickets(ctx context.Context, items []*models.OrderItem, user *models.User) error
	GetTicketStockWithTypeBySession(ctx context.Context, sessionID primitive.ObjectID) ([]dto.TicketStockWithType, error)
	CountTicketsByOrder(ctx context.Context, orderID primitive.ObjectID) (int64, error)
	CreateManyTickets(ctx context.Context, tickets []interface{}) error
	GetTicketByOrderItemID(ctx context.Context, orderItemID primitive.ObjectID) (*models.Ticket, error)
	UpdateTicketStatus(ctx context.Context, ticketID primitive.ObjectID, status string) error
	GetTicketByID(ctx context.Context, ticketID primitive.ObjectID) (*models.Ticket, error)
	MarkTicketAsUsed(ctx context.Context, ticketID primitive.ObjectID, usedAt time.Time, operator *models.User) error
	ExpireTickets(ctx context.Context, now time.Time) (int64, error)
}

type OrdersRepository interface {
	CreateOrder(ctx context.Context, order *models.Order) (primitive.ObjectID, error)
	CreateOrderItems(ctx context.Context, orderID primitive.ObjectID, items []*models.OrderItem) error
	IsEventHasOpenOrders(ctx context.Context, eventID primitive.ObjectID) (bool, error)
	GetOrdersByEvent(ctx context.Context, params *GetOrdersByEventParams) ([]*dto.OrderWithItems, int64, error)
	GetOrdersBySession(ctx context.Context, params *GetOrdersBySessionParams) ([]*dto.OrderWithItems, int64, error)
	GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error)
	GetOrderItemByID(ctx context.Context, itemID primitive.ObjectID) (*models.OrderItem, error)
	GetOrderItemsByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderItem, error)
	UpdateOrderItemStatus(ctx context.Context, params *UpdateOrderItemStatusParams) error
	UpdateOrderStatus(ctx context.Context, params *UpdateOrderStatusParams) (*models.Order, error)
	GetOrdersByUser(ctx context.Context, params *GetOrdersByUserParams) ([]*dto.OrderWithItems, error)
	GetOrderWithItemsByID(ctx context.Context, orderID primitive.ObjectID) (*dto.OrderWithItems, error)
	UpdateOrder(ctx context.Context, orderID primitive.ObjectID, opts ...UpdateOption) error
	UpdateItemsStatusByOrder(ctx context.Context, orderID primitive.ObjectID, currentStatus string, newStatus string, operator *models.User) (int64, error)
	GetOrderItemWithOrder(ctx context.Context, orderItemID primitive.ObjectID) (*dto.OrderItemWithOrder, error)
	GetOrdersByEventAndMonth(ctx context.Context, eventID primitive.ObjectID, year int, month int) ([]*dto.OrderWithItems, error)
}

type BillRepository interface {
	CreateBill(ctx context.Context, bill *models.Bill) (primitive.ObjectID, error)
	GetBillsByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.Bill, error)
	GetBillByID(ctx context.Context, id primitive.ObjectID) (*models.Bill, error)
	UpdateBill(ctx context.Context, id primitive.ObjectID, opts ...UpdateOption) error
	GetRefundsByOrderItemID(ctx context.Context, orderItemID primitive.ObjectID) ([]*models.Bill, error)
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *models.AuditLog) error
}

type OutboxRepository interface {
	Create(ctx context.Context, message *models.OutboxMessage) error
	ClaimAndFetchEvents(ctx context.Context, limit int) ([]*models.OutboxMessage, error)
	MarkAsProcessed(ctx context.Context, id primitive.ObjectID) error
	IncrementRetry(ctx context.Context, id primitive.ObjectID, errorMessage string) error
}
