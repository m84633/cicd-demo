package logic

import (
	"context"
	"errors"
	"fmt"
	"partivo_tickets/internal/client/events"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/models"
	"partivo_tickets/pkg/jwt"
	"partivo_tickets/pkg/relation"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TicketLogic encapsulates business logic for tickets and holds its dependencies.
type TicketLogic struct {
	ticketRepo           repository.TicketsRepository
	orderRepo            repository.OrdersRepository
	auditLogRepo         repository.AuditLogRepository
	relationClient       *relation.Client
	logger               *zap.Logger
	jwtGenerator         *jwt.Manager
	eventClient          *events.Client
	ticketEventPublisher *TicketEventPublisher
}

// NewTicketLogic creates a new instance of TicketLogic.
func NewTicketLogic(ticketRepo repository.TicketsRepository, orderRepo repository.OrdersRepository, auditLogRepo repository.AuditLogRepository, relationClient *relation.Client, logger *zap.Logger, jwtGenerator *jwt.Manager, eventClient *events.Client, ticketEventPublisher *TicketEventPublisher) *TicketLogic {
	return &TicketLogic{
		ticketRepo:           ticketRepo,
		orderRepo:            orderRepo,
		auditLogRepo:         auditLogRepo,
		relationClient:       relationClient,
		logger:               logger.Named("TicketLogic"),
		jwtGenerator:         jwtGenerator,
		eventClient:          eventClient,
		ticketEventPublisher: ticketEventPublisher,
	}
}

// GenerateTicketsForOrder creates ticket instances for a given paid order.
func (l *TicketLogic) GenerateTicketsForOrder(ctx context.Context, orderID primitive.ObjectID) error {
	// 1. Fetch the order with its items to ensure it's valid and get details.
	orderWithItems, err := l.orderRepo.GetOrderWithItemsByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			l.logger.Warn("GenerateTicketsForOrder: Order not found, message might be stale.", zap.Stringer("orderID", orderID))
			return nil // Return nil to ACK the message and prevent retries.
		}
		return fmt.Errorf("failed to get order with items: %w", err)
	}

	// 2. Idempotency Check: Ensure the order is in PAID status.
	if orderWithItems.Order.Status != constants.OrderStatusPaid.String() {
		l.logger.Warn("GenerateTicketsForOrder: Order is not in PAID status.",
			zap.Stringer("orderID", orderID),
			zap.String("status", orderWithItems.Order.Status))
		return nil
	}

	// 3. Idempotency Check: Check if tickets have already been generated for this order.
	count, err := l.ticketRepo.CountTicketsByOrder(ctx, orderID)
	if err != nil {
		return fmt.Errorf("failed to check for existing tickets: %w", err)
	}
	if count > 0 {
		l.logger.Warn("GenerateTicketsForOrder: Tickets have already been generated for this order.", zap.Stringer("orderID", orderID))
		return nil // ACK the message, processing is already complete.
	}

	// 4. Generate ticket models for each order item.
	var ticketsToCreate []interface{}
	for _, item := range orderWithItems.Items {
		// We assume each item has a quantity of 1, as per the order creation logic.
		ticketID := primitive.NewObjectID()
		now := time.Now()

		session, err := l.eventClient.GetSessionByID(ctx, item.Session.Hex())
		if err != nil {
			st, ok := status.FromError(err)
			if ok && st.Code() == codes.NotFound {
				// This is a permanent error. The session for a paid order item does not exist.
				// We should not retry this message. Log as an error and return the permanent error.
				l.logger.Error("GenerateTicketsForOrder: Session not found for a paid order item. This is a permanent failure.",
					zap.Stringer("orderID", orderID),
					zap.Stringer("orderItemID", item.ID),
					zap.Stringer("sessionID", item.Session),
				)
				return ErrPermanent
			}
			// For other errors (e.g., network), return the error to trigger a retry.
			return fmt.Errorf("failed to get session for item %s: %w", item.ID.Hex(), err)
		}

		validFrom, err := time.Parse(time.RFC3339, session.GetStartTime())
		if err != nil {
			return fmt.Errorf("failed to parse GetStartTime to validFrom %s: %w", item.ID.Hex(), err)
		}
		validUntil, err := time.Parse(time.RFC3339, session.GetEndTime())
		if err != nil {
			return fmt.Errorf("failed to parse GetEndTime to validFrom %s: %w", item.ID.Hex(), err)
		}

		payload := map[string]interface{}{
			"id":        ticketID.Hex(),
			"event":     orderWithItems.Event.Hex(),
			"session":   item.Session.Hex(),
			"orderItem": item.Name,
		}

		qrCode, err := l.jwtGenerator.Generate(payload,
			jwt.WithNotBefore(validFrom),
			jwt.WithExpiresAt(validUntil),
		)
		if err != nil {
			return fmt.Errorf("failed to generate QR code for item %s: %w", item.ID.Hex(), err)
		}

		ticket := &models.Ticket{
			ID: ticketID,
			OrderItem: models.OrderItemInfo{
				ID:    item.ID,
				Price: item.Price,
				Name:  item.Name,
			},
			Order: orderWithItems.Serial,
			Customer: models.User{
				UserId: orderWithItems.User.UserId,
				Name:   orderWithItems.User.Name,
				Email:  orderWithItems.User.Email,
			},
			Event:       orderWithItems.Event,
			Session:     item.Session,
			TicketStock: item.TicketStock,
			Attributes:  nil,
			Status:      constants.TicketStatusValid.String(),
			QRCodeData:  qrCode,
			ValidFrom:   validFrom,
			ValidUntil:  validUntil,
			UsedAt:      nil,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		ticketsToCreate = append(ticketsToCreate, ticket)
	}

	if len(ticketsToCreate) == 0 {
		l.logger.Info("GenerateTicketsForOrder: No tickets to generate for order.", zap.Stringer("orderID", orderID))
		return nil
	}

	// 5. Batch insert the new tickets into the database.
	if err := l.ticketRepo.CreateManyTickets(ctx, ticketsToCreate); err != nil {
		return fmt.Errorf("failed to create tickets in repository: %w", err)
	}

	l.logger.Info("Successfully generated tickets", zap.Int("count", len(ticketsToCreate)), zap.Stringer("orderID", orderID))
	return nil
}

func (l *TicketLogic) AddTicketType(ctx context.Context, d *dto.AddTicketTypeRequest) (primitive.ObjectID, error) {
	createdby := d.GetCreatedBy()

	//always display
	var ad bool
	v := d.GetVisibility()
	start := d.GetStartShowingOn()
	end := d.GetStopShowingOn()
	if v && start == nil && end == nil {
		ad = true
	}

	//ticket type
	tc := &models.TicketType{
		ID:                  primitive.NewObjectID(),
		Event:               d.GetEvent(),
		Name:                d.GetName(),
		Quantity:            d.GetQuantity(),
		Price:               d.GetPrice(),
		Desc:                d.GetDescription(),
		Visible:             v,
		AlwaysDisplay:       ad,
		StartShowAt:         d.GetStartShowingOn(),
		Pick:                d.GetPick(),
		StopShowAt:          d.GetStopShowingOn(),
		UpdatedAt:           nil,
		CreatedAt:           time.Now(),
		CreatedBy:           *createdby,
		UpdatedBy:           nil,
		MinQuantityPerOrder: d.GetMinQuantityPerOrder(),
		MaxQuantityPerOrder: d.GetMaxQuantityPerOrder(),
		DueDuration:         d.GetDueDuration(),
		Enable:              d.GetEnable(),
		SaleSetting:         d.GetSaleTimeSetting(),
	}
	tcid, err := l.ticketRepo.AddTicketType(ctx, tc)
	if err != nil {
		l.logger.Error("_addTicketType: Failed to add ticket type", zap.Error(err), zap.Any("ticketType", tc))
		return primitive.NilObjectID, err
	}

	//add ticket
	sessions := d.GetSessions()
	for _, s := range sessions {
		t := &models.TicketStock{
			ID:          primitive.NewObjectID(),
			Session:     s.ID(),
			Type:        tcid,
			SaleStartAt: s.SaleStartAt(),
			SaleEndAt:   s.SaleEndAt(),
			Quantity:    tc.Quantity,
			UpdatedAt:   tc.CreatedAt,
			UpdatedBy:   tc.CreatedBy,
		}
		err := l.ticketRepo.AddTicketStock(ctx, t)
		if err != nil {
			l.logger.Error("_addTicketType: Failed to add ticket stock", zap.Error(err), zap.Any("ticketStock", t))
			return primitive.NilObjectID, err
		}
	}

	// --- Create and save the audit log ---
	auditLog := buildAddTicketTypeAuditLog(&tc.CreatedBy, tc)
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("_addTicketType: Failed to create audit log", zap.Error(err))
		return primitive.NilObjectID, err // Roll back transaction
	}

	// relation
	//TODO:等resource創完再開
	//err = l.relationClient.AddUserResourceRole(ctx, createdby.UserId.Hex(), constants.ResourceTicket, tc.ID.Hex(), relation.RoleOwner)
	//if err != nil {
	//	return primitive.NilObjectID, err
	//}

	return tc.ID, nil
}

func (l *TicketLogic) GetPublisher() *TicketEventPublisher {
	return l.ticketEventPublisher
}

func (l *TicketLogic) GetTicketTypesByEvent(ctx context.Context, sid primitive.ObjectID) ([]dto.TicketTypeWithStock, error) {
	return l.ticketRepo.GetTicketTypesWithStockByEvent(ctx, sid)
}

func (l *TicketLogic) DeleteTicketType(ctx context.Context, tcid primitive.ObjectID, operator *models.User) (*models.TicketType, error) {
	//TODO:要先確定為draft狀態
	// 1. 刪除票種設定並取得被刪除的物件
	deletedTicket, err := l.ticketRepo.DeleteTicketTypeByID(ctx, tcid)
	if err != nil {
		return nil, err
	}
	// 如果沒有文件被刪除，直接返回
	if deletedTicket == nil {
		return nil, nil
	}

	// 2. 刪除與該票種設定相關的所有票券實例
	if err := l.ticketRepo.DeleteTicketStockByType(ctx, tcid); err != nil {
		return nil, err
	}

	// 3. 建立並儲存稽核紀錄
	auditLog := buildDeleteTicketTypeAuditLog(operator, deletedTicket)
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("_deleteTicketType: Failed to create audit log", zap.Error(err))
		return nil, err
	}

	return deletedTicket, nil
}

func (l *TicketLogic) UpdateTicketType(ctx context.Context, d *dto.UpdateTicketTypeRequest) error {
	ticketID := d.GetID()

	// 1. 取得更新前的狀態
	ticketBefore, err := l.ticketRepo.GetTicketTypeByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("failed to get ticket before update: %w", err)
	}

	// --- 核心更新邏輯 ---
	n := time.Now()
	var ad bool
	v := d.GetVisibility()
	start := d.GetStartShowingOn()
	end := d.GetStopShowingOn()
	if v && start == nil && end == nil {
		ad = true
	}

	m := &models.TicketType{
		ID:                  ticketID,
		Name:                d.GetName(),
		Quantity:            d.GetQuantity(),
		Price:               d.GetPrice(),
		Desc:                d.GetDescription(),
		Visible:             d.GetVisibility(),
		AlwaysDisplay:       ad,
		StartShowAt:         d.GetStartShowingOn(),
		StopShowAt:          d.GetStopShowingOn(),
		UpdatedAt:           &n,
		UpdatedBy:           d.GetUpdatedBy(),
		MinQuantityPerOrder: d.GetMinQuantityPerOrder(),
		MaxQuantityPerOrder: d.GetMaxQuantityPerOrder(),
		DueDuration:         d.GetDueDuration(),
		Enable:              d.GetEnable(),
		SaleSetting:         d.GetSaleTimeSetting(),
	}
	if err := l.ticketRepo.UpdateTicketType(ctx, m); err != nil {
		return err
	}

	// Sync sessions: delete old ones, add new ones.
	if err := l.ticketRepo.DeleteTicketStockByType(ctx, ticketID); err != nil {
		return err
	}

	updatedBy := d.GetUpdatedBy()
	for _, s := range d.GetSessions() {
		t := &models.TicketStock{
			ID:          primitive.NewObjectID(),
			Session:     s.ID(),
			Type:        ticketID,
			SaleStartAt: s.SaleStartAt(),
			SaleEndAt:   s.SaleEndAt(),
			Quantity:    m.Quantity,
			UpdatedAt:   n,
			UpdatedBy:   *updatedBy,
		}
		if err := l.ticketRepo.AddTicketStock(ctx, t); err != nil {
			return err
		}
	}

	// --- 稽核紀錄 ---
	// 2. 取得更新後的狀態
	ticketAfter, err := l.ticketRepo.GetTicketTypeByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("failed to get ticket after update: %w", err)
	}

	// 3. 建立並儲存稽核紀錄
	auditLog := buildUpdateTicketTypeAuditLog(d.GetUpdatedBy(), ticketBefore, ticketAfter)
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("_updateTicketType: Failed to create audit log", zap.Error(err))
		return err // Roll back transaction
	}

	return nil
}

func (l *TicketLogic) GetValidTicketTypesByEvent(ctx context.Context, eid primitive.ObjectID) ([]dto.TicketTypeWithStock, error) {
	//return l.ticketRepo.GetTicketTypesWithStockByEvent(ctx, eid, mongodb.WithDisplay(true), mongodb.WithIsShowing(true))
	return l.ticketRepo.GetTicketTypesWithStockByEvent(ctx, eid, repository.WithDisplay(true), repository.WithIsShowing(true))
}

func (l *TicketLogic) UpdateTicketTypesOrder(ctx context.Context, r *dto.UpdateTicketTypesOrderRequest) error {
	eventID := r.GetEvent()
	operator := r.GetUpdatedBy()
	newOrderIDs := r.GetIDs()

	// 1. Get "before" state (the current order of ticket IDs)
	ticketsBefore, err := l.ticketRepo.GetTicketTypesWithStockByEvent(ctx, eventID)
	if err != nil {
		return fmt.Errorf("failed to get tickets before update: %w", err)
	}
	var idsBefore []primitive.ObjectID
	for _, t := range ticketsBefore {
		idsBefore = append(idsBefore, t.ID)
	}

	// 2. Execute the update
	if err := l.ticketRepo.UpdateTicketTypesOrder(ctx, eventID, newOrderIDs, operator); err != nil {
		return err
	}

	// 3. Create and save the audit log
	// The "after" state is simply the list of IDs we just passed to the update function.
	auditLog := buildUpdateTicketTypesOrderAuditLog(operator, eventID, idsBefore, newOrderIDs)
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("_updateTicketTypesOrder: Failed to create audit log", zap.Error(err))
		return err // Roll back transaction
	}

	return nil
}

func (l *TicketLogic) GetTicketStockBySession(ctx context.Context, sid primitive.ObjectID) ([]dto.TicketStockWithType, error) {
	return l.ticketRepo.GetTicketStockWithTypeBySession(ctx, sid)
}

// VoidTicketByOrderItem finds a ticket by its corresponding order item ID and voids it.
func (l *TicketLogic) VoidTicketByOrderItem(ctx context.Context, orderItemID primitive.ObjectID) error {
	// 1. Find the ticket associated with the order item.
	ticket, err := l.ticketRepo.GetTicketByOrderItemID(ctx, orderItemID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			// This might not be an error in some cases, e.g., if ticket generation is asynchronous.
			// For a refund, however, the ticket should exist. Log a warning.
			l.logger.Warn("Could not find a ticket to void for order item", zap.Stringer("orderItemID", orderItemID))
			return nil // Return nil to not fail the entire transaction.
		}
		return fmt.Errorf("failed to get ticket by order item id: %w", err)
	}

	//2. Release the stock back to the pool.
	itemToRelease := &models.OrderItem{
		TicketStock: ticket.TicketStock,
		Quantity:    1,
	}
	if err := l.ticketRepo.ReleaseTickets(ctx, []*models.OrderItem{itemToRelease}); err != nil {
		//Log the error but consider if this should be a fatal error for the transaction.
		// If releasing stock fails, we might have an inconsistent state.
		l.logger.Error("Failed to release ticket stock when voiding ticket",
			zap.Stringer("ticketID", ticket.ID),
			zap.Error(err),
		)
		return fmt.Errorf("failed to release ticket stock: %w", err)
	}
	// 3. Update the ticket status to Invalidated.
	if err := l.ticketRepo.UpdateTicketStatus(ctx, ticket.ID, constants.TicketInvalidated.String()); err != nil {
		return fmt.Errorf("failed to update ticket status to invalidated: %w", err)
	}

	l.logger.Info("Successfully invalidated ticket for refunded order item", zap.Stringer("ticketID", ticket.ID), zap.Stringer("orderItemID", orderItemID))
	return nil
}

// CheckInTicket attempts to use a ticket. It returns the ticket's current state,
// a boolean indicating if the ticket was just successfully checked in, and an error
// if the ticket is not found or a system error occurs.
func (l *TicketLogic) CheckInTicket(ctx context.Context, ticketID primitive.ObjectID, operator *models.User) (*models.Ticket, bool, error) {
	// 1. Fetch the ticket from the repository.
	ticket, err := l.ticketRepo.GetTicketByID(ctx, ticketID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, false, ErrTicketNotFound
		}
		return nil, false, fmt.Errorf("failed to get ticket by id: %w", err)
	}

	// 2. If the ticket is not in a 'Valid' state, it cannot be checked in. Return the current state.
	if ticket.Status != constants.TicketStatusValid.String() {
		return ticket, false, nil
	}

	// 3. Check if the ticket is within its validity period.
	now := time.Now()
	if now.Before(ticket.ValidFrom) || now.After(ticket.ValidUntil) {
		return ticket, false, ErrTicketNotWithinValidityPeriod
	}

	// 4. The ticket is valid and within the time window. Atomically mark it as used.
	err = l.ticketRepo.MarkTicketAsUsed(ctx, ticketID, now, operator)
	if err != nil {
		// If the error is mongo.ErrNoDocuments, it means our atomic update failed
		// because the ticket was not 'Valid' anymore (race condition).
		if errors.Is(err, mongo.ErrNoDocuments) {
			l.logger.Info("CheckInTicket: Race condition detected. Ticket status changed before update.", zap.Stringer("ticketID", ticketID))
			// Fetch the latest state of the ticket to return to the user.
			latestTicket, fetchErr := l.ticketRepo.GetTicketByID(ctx, ticketID)
			return latestTicket, false, fetchErr
		}
		// For other database errors, return the error.
		return nil, false, fmt.Errorf("failed to mark ticket as used: %w", err)
	}

	// 5. Fetch the updated ticket to return the latest state including the UsedAt timestamp.
	updatedTicket, err := l.ticketRepo.GetTicketByID(ctx, ticketID)
	if err != nil {
		l.logger.Error("CheckInTicket: failed to get updated ticket after status update", zap.Error(err), zap.Stringer("ticketID", ticketID))
		return nil, false, fmt.Errorf("failed to get updated ticket by id: %w", err)
	}

	// --- Create and save the audit log ---
	auditLog := buildCheckInTicketAuditLog(operator, ticket, updatedTicket)
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("CheckInTicket: Failed to create audit log", zap.Error(err))
		return nil, false, err // Roll back transaction
	}

	l.logger.Info("Ticket checked in successfully", zap.Stringer("ticketID", ticketID))
	return updatedTicket, true, nil
}

func (l *TicketLogic) ExpireOverdueTickets(ctx context.Context) (int64, error) {
	now := time.Now()
	count, err := l.ticketRepo.ExpireTickets(ctx, now)
	if err != nil {
		l.logger.Error("ExpireOverdueTickets: failed to expire tickets", zap.Error(err))
		return 0, err
	}

	if count > 0 {
		l.logger.Info("Expired overdue tickets", zap.Int64("count", count))
	}

	return count, nil
}
