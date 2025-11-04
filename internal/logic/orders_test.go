package logic

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/models"
	"partivo_tickets/pkg/pagination"
	"partivo_tickets/pkg/snowflake"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

func TestOrderLogic_AddOrder(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		ticketRepo := newMockTicketsRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		outboxRepo := newMockOutboxRepository()
		paymentPublisher := &PaymentEventPublisher{
			outboxRepo:        outboxRepo,
			paymentEventTopic: PaymentEventTopic("payments"),
		}
		idGen, err := snowflake.NewGenerator(1)
		require.NoError(t, err)

		l := &orderLogic{
			orderRepo:             orderRepo,
			billRepo:              billRepo,
			ticketRepo:            ticketRepo,
			auditLogRepo:          auditLogRepo,
			paymentEventPublisher: paymentPublisher,
			relationClient:        nil,
			idGenerator:           idGen,
			logger:                zap.NewNop(),
		}

		user := &models.User{
			UserId: primitive.NewObjectID(),
			Name:   "Alice",
			Email:  "alice@example.com",
		}
		eventID := primitive.NewObjectID()
		sessionID := primitive.NewObjectID()
		ticketStockID := primitive.NewObjectID()
		ticketTypeID := primitive.NewObjectID()
		merchantID := primitive.NewObjectID()
		paymentMethodID := primitive.NewObjectID()
		price, _ := primitive.ParseDecimal128("100.00")

		tickets := []dto.TicketStockWithType{
			{
				Type: models.TicketType{
					ID:                  ticketTypeID,
					Event:               eventID,
					Name:                "VIP Pass",
					Price:               price,
					MaxQuantityPerOrder: 4,
					MinQuantityPerOrder: 1,
				},
				Stock: models.TicketStock{
					ID:       ticketStockID,
					Session:  sessionID,
					Quantity: 10,
				},
			},
		}

		ticketRepo.On("GetValidTicketsByIDs", mock.Anything, mock.MatchedBy(func(ids []primitive.ObjectID) bool {
			return len(ids) == 1 && ids[0] == ticketStockID
		})).Return(tickets, nil).Once()

		var createdOrder *models.Order
		orderRepo.On("CreateOrder", mock.Anything, mock.MatchedBy(func(order *models.Order) bool {
			createdOrder = order
			assert.Equal(t, eventID, order.Event)
			assert.Equal(t, sessionID, order.Session)
			return true
		})).Return(primitive.NilObjectID, nil).Once()

		orderRepo.On("CreateOrderItems", mock.Anything, mock.AnythingOfType("primitive.ObjectID"), mock.AnythingOfType("[]*models.OrderItem")).
			Run(func(args mock.Arguments) {
				orderID := args.Get(1).(primitive.ObjectID)
				items := args.Get(2).([]*models.OrderItem)
				assert.Equal(t, createdOrder.ID, orderID)
				assert.Len(t, items, 2)
			}).
			Return(nil).Once()

		ticketRepo.On("ReserveTickets", mock.Anything, mock.AnythingOfType("[]*models.OrderItem"), mock.AnythingOfType("*models.User")).
			Run(func(args mock.Arguments) {
				items := args.Get(1).([]*models.OrderItem)
				assert.Len(t, items, 2)
			}).
			Return(nil).Once()

		auditLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AuditLog")).Return(nil).Once()

		var createdBill *models.Bill
		billRepo.On("CreateBill", mock.Anything, mock.MatchedBy(func(b *models.Bill) bool {
			createdBill = b
			assert.Equal(t, createdOrder.ID, b.Order)
			assert.Equal(t, paymentMethodID, b.PaymentMethod.ID)
			return true
		})).Return(primitive.NilObjectID, nil).Once()

		outboxRepo.On("Create", mock.Anything, mock.MatchedBy(func(msg *models.OutboxMessage) bool {
			return msg.Topic == string(paymentPublisher.paymentEventTopic)
		})).Return(nil).Once()

		details := []*dto.OrderItem{
			dto.NewOrderItem(ticketStockID, 2),
		}
		req := dto.NewAddOrderRequest(
			eventID,
			merchantID,
			"200.00",
			"200.00",
			details,
			user,
			dto.NewPaymentMethodInfoRequest(paymentMethodID, "Credit Card"),
		)

		resp, err := l.AddOrder(context.Background(), req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, createdOrder.ID, resp.OrderID)
		assert.Equal(t, createdBill.ID, resp.BillID)

		orderRepo.AssertExpectations(t)
		ticketRepo.AssertExpectations(t)
		billRepo.AssertExpectations(t)
		auditLogRepo.AssertExpectations(t)
		outboxRepo.AssertExpectations(t)
	})

	t.Run("EventMismatch", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		ticketRepo := newMockTicketsRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		outboxRepo := newMockOutboxRepository()
		paymentPublisher := &PaymentEventPublisher{
			outboxRepo:        outboxRepo,
			paymentEventTopic: PaymentEventTopic("payments"),
		}
		idGen, err := snowflake.NewGenerator(1)
		require.NoError(t, err)

		l := &orderLogic{
			orderRepo:             orderRepo,
			billRepo:              billRepo,
			ticketRepo:            ticketRepo,
			auditLogRepo:          auditLogRepo,
			paymentEventPublisher: paymentPublisher,
			relationClient:        nil,
			idGenerator:           idGen,
			logger:                zap.NewNop(),
		}

		requestEventID := primitive.NewObjectID()
		ticketEventID := primitive.NewObjectID()
		sessionID := primitive.NewObjectID()
		ticketStockID := primitive.NewObjectID()
		user := &models.User{UserId: primitive.NewObjectID()}
		paymentMethodID := primitive.NewObjectID()
		price, _ := primitive.ParseDecimal128("50.00")

		tickets := []dto.TicketStockWithType{
			{
				Type: models.TicketType{
					ID:                  primitive.NewObjectID(),
					Event:               ticketEventID,
					Name:                "GA",
					Price:               price,
					MaxQuantityPerOrder: 5,
					MinQuantityPerOrder: 1,
				},
				Stock: models.TicketStock{
					ID:       ticketStockID,
					Session:  sessionID,
					Quantity: 5,
				},
			},
		}

		ticketRepo.On("GetValidTicketsByIDs", mock.Anything, mock.Anything).Return(tickets, nil).Once()

		req := dto.NewAddOrderRequest(
			requestEventID,
			primitive.NewObjectID(),
			"100.00",
			"100.00",
			[]*dto.OrderItem{dto.NewOrderItem(ticketStockID, 2)},
			user,
			dto.NewPaymentMethodInfoRequest(paymentMethodID, "Card"),
		)

		resp, err := l.AddOrder(context.Background(), req)

		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "event mismatch")

		ticketRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_GetOrder(t *testing.T) {
	orderRepo := newMockOrdersRepository()
	l := &orderLogic{
		orderRepo: orderRepo,
		logger:    zap.NewNop(),
	}

	t.Run("Success", func(t *testing.T) {
		id := primitive.NewObjectID()
		expectedOrder := &models.Order{ID: id, Status: constants.OrderStatusPaid.String()}

		orderRepo.On("GetOrderByID", mock.Anything, id).Return(expectedOrder, nil).Once()

		order, err := l.GetOrder(context.Background(), id)

		assert.NoError(t, err)
		assert.Equal(t, expectedOrder, order)

		orderRepo.AssertExpectations(t)
	})

	t.Run("RepositoryError", func(t *testing.T) {
		id := primitive.NewObjectID()
		repoErr := errors.New("not found")

		orderRepo.On("GetOrderByID", mock.Anything, id).Return(nil, repoErr).Once()

		order, err := l.GetOrder(context.Background(), id)

		assert.Nil(t, order)
		assert.Equal(t, repoErr, err)

		orderRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_GetOrdersByUser(t *testing.T) {
	t.Run("First page without next token", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		userID := primitive.NewObjectID()
		createdAt := time.Date(2024, time.March, 10, 12, 0, 0, 0, time.UTC)
		repoOrders := []*dto.OrderWithItems{
			{
				Order: &models.Order{
					ID:        primitive.NewObjectID(),
					CreatedAt: createdAt,
				},
			},
		}

		orderRepo.On("GetOrdersByUser", mock.Anything, mock.MatchedBy(func(params *repository.GetOrdersByUserParams) bool {
			return params.UserID == userID &&
				params.Limit == 5 &&
				params.CursorID.IsZero() &&
				params.CursorCreatedAt.IsZero()
		})).Return(repoOrders, nil).Once()

		orders, nextToken, err := l.GetOrdersByUser(context.Background(), userID, "")

		assert.NoError(t, err)
		assert.Equal(t, repoOrders, orders)
		assert.Equal(t, pagination.PageToken(""), nextToken)

		orderRepo.AssertExpectations(t)
	})

	t.Run("Subsequent page produces next token", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		userID := primitive.NewObjectID()
		cursorID := primitive.NewObjectID()
		//cursorTime := time.Date(2024, time.April, 1, 9, 0, 0, 0, time.UTC)
		cursorTime := time.Now()
		token, err := pagination.Page{CursorID: cursorID.Hex(), CursorTimestamp: cursorTime.Unix()}.Encode()
		require.NoError(t, err)

		lastOrderID := primitive.NewObjectID()
		lastOrderTime := time.Now()

		repoOrders := []*dto.OrderWithItems{
			{
				Order: &models.Order{
					ID:        primitive.NewObjectID(),
					CreatedAt: cursorTime.Add(1 * time.Hour),
				},
			},
			{
				Order: &models.Order{
					ID:        primitive.NewObjectID(),
					CreatedAt: cursorTime.Add(2 * time.Hour),
				},
			},
			{
				Order: &models.Order{
					ID:        primitive.NewObjectID(),
					CreatedAt: cursorTime.Add(3 * time.Hour),
				},
			},
			{
				Order: &models.Order{
					ID:        primitive.NewObjectID(),
					CreatedAt: cursorTime.Add(4 * time.Hour),
				},
			},
			{
				Order: &models.Order{
					ID:        lastOrderID,
					CreatedAt: lastOrderTime,
				},
			},
		}

		orderRepo.On("GetOrdersByUser", mock.Anything, mock.MatchedBy(func(params *repository.GetOrdersByUserParams) bool {
			return params.UserID == userID &&
				params.Limit == 5 &&
				params.CursorID == cursorID &&
				params.CursorCreatedAt.Equal(time.Unix(cursorTime.Unix(), 0))
		})).Return(repoOrders, nil).Once()

		orders, nextToken, err := l.GetOrdersByUser(context.Background(), userID, token)

		assert.NoError(t, err)
		assert.Equal(t, repoOrders, orders)
		assert.NotEmpty(t, nextToken)

		page, err := nextToken.Decode()
		assert.NoError(t, err)
		assert.Equal(t, lastOrderID.Hex(), page.CursorID)
		assert.Equal(t, lastOrderTime.Unix(), page.CursorTimestamp)

		orderRepo.AssertExpectations(t)
	})

	t.Run("Invalid base64 token", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		orders, nextToken, err := l.GetOrdersByUser(context.Background(), primitive.NewObjectID(), pagination.PageToken("$$bad"))

		assert.Nil(t, orders)
		assert.Equal(t, pagination.PageToken(""), nextToken)
		assert.ErrorIs(t, err, pagination.ErrInvalidToken)

		orderRepo.AssertNotCalled(t, "GetOrdersByUser", mock.Anything, mock.Anything)
	})

	t.Run("Invalid cursor id in token", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		badToken, err := pagination.Page{CursorID: "not_a_hex", CursorTimestamp: time.Now().Unix()}.Encode()
		require.NoError(t, err)

		orders, nextToken, err := l.GetOrdersByUser(context.Background(), primitive.NewObjectID(), badToken)

		assert.Nil(t, orders)
		assert.Equal(t, pagination.PageToken(""), nextToken)
		assert.ErrorIs(t, err, pagination.ErrInvalidToken)

		orderRepo.AssertNotCalled(t, "GetOrdersByUser", mock.Anything, mock.Anything)
	})

	t.Run("Repository error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		userID := primitive.NewObjectID()
		repoErr := errors.New("db unavailable")

		orderRepo.On("GetOrdersByUser", mock.Anything, mock.MatchedBy(func(params *repository.GetOrdersByUserParams) bool {
			return params.UserID == userID
		})).Return(nil, repoErr).Once()

		orders, nextToken, err := l.GetOrdersByUser(context.Background(), userID, "")

		assert.Nil(t, orders)
		assert.Equal(t, pagination.PageToken(""), nextToken)
		assert.Equal(t, repoErr, err)

		orderRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_CancelOrder(t *testing.T) {
	t.Run("Success for unpaid order", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		ticketRepo := newMockTicketsRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		outboxRepo := newMockOutboxRepository()
		paymentPublisher := &PaymentEventPublisher{
			outboxRepo:        outboxRepo,
			paymentEventTopic: PaymentEventTopic("payments"),
		}

		l := &orderLogic{
			orderRepo:             orderRepo,
			billRepo:              billRepo,
			ticketRepo:            ticketRepo,
			auditLogRepo:          auditLogRepo,
			paymentEventPublisher: paymentPublisher,
			logger:                zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		merchantID := primitive.NewObjectID()
		operator := &models.User{UserId: primitive.NewObjectID()}

		order := &models.Order{
			ID:         orderID,
			Status:     constants.OrderStatusUnpaid.String(),
			MerchantID: merchantID,
			User:       operator,
		}

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(order, nil).Once()

		items := []*models.OrderItem{
			{
				ID:          primitive.NewObjectID(),
				TicketStock: primitive.NewObjectID(),
			},
		}
		orderRepo.On("GetOrderItemsByOrderID", mock.Anything, orderID).Return(items, nil).Once()

		bill := &models.Bill{
			ID:     primitive.NewObjectID(),
			Status: constants.BillStatusPending.String(),
			PaymentMethod: &models.PaymentMethodInfo{
				ID: primitive.NewObjectID(),
			},
		}
		billRepo.On("GetBillsByOrderID", mock.Anything, orderID).Return([]*models.Bill{bill}, nil).Once()
		billRepo.On("UpdateBill", mock.Anything, bill.ID, mock.Anything).Return(nil).Once()
		outboxRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.OutboxMessage")).Return(nil).Once()

		orderRepo.On("UpdateItemsStatusByOrder", mock.Anything, orderID,
			constants.OrderItemStatusPendingPayment.String(),
			constants.OrderItemStatusCanceled.String(),
			operator).Return(int64(1), nil).Once()

		ticketRepo.On("ReleaseTickets", mock.Anything, items).Return(nil).Once()
		orderRepo.On("UpdateOrder", mock.Anything, orderID, mock.Anything).Return(nil).Once()
		auditLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AuditLog")).Return(nil).Once()

		req := dto.NewCancelOrderRequest(orderID, operator, "user requested")
		err := l.CancelOrder(context.Background(), req)

		assert.NoError(t, err)

		orderRepo.AssertExpectations(t)
		billRepo.AssertExpectations(t)
		ticketRepo.AssertExpectations(t)
		auditLogRepo.AssertExpectations(t)
		outboxRepo.AssertExpectations(t)
	})

	t.Run("GetOrder failure", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		expectedErr := errors.New("db down")

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(nil, expectedErr).Once()

		err := l.CancelOrder(context.Background(), dto.NewCancelOrderRequest(orderID, &models.User{}, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Unsupported order status", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		order := &models.Order{
			ID:     primitive.NewObjectID(),
			Status: constants.OrderStatusCanceled.String(),
		}
		orderRepo.On("GetOrderByID", mock.Anything, order.ID).Return(order, nil).Once()

		err := l.CancelOrder(context.Background(), dto.NewCancelOrderRequest(order.ID, &models.User{}, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be cancelled")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Fails to fetch order items", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		billRepo := newMockBillRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  billRepo,
			logger:    zap.NewNop(),
		}

		order := &models.Order{
			ID:     primitive.NewObjectID(),
			Status: constants.OrderStatusUnpaid.String(),
			User:   &models.User{},
		}

		orderRepo.On("GetOrderByID", mock.Anything, order.ID).Return(order, nil).Once()
		orderRepo.On("GetOrderItemsByOrderID", mock.Anything, order.ID).Return(nil, errors.New("items missing")).Once()

		err := l.CancelOrder(context.Background(), dto.NewCancelOrderRequest(order.ID, order.User, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order items for void")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Publish payment event failure", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		ticketRepo := newMockTicketsRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		outboxRepo := newMockOutboxRepository()
		paymentPublisher := &PaymentEventPublisher{
			outboxRepo:        outboxRepo,
			paymentEventTopic: PaymentEventTopic("payments"),
		}

		l := &orderLogic{
			orderRepo:             orderRepo,
			billRepo:              billRepo,
			ticketRepo:            ticketRepo,
			auditLogRepo:          auditLogRepo,
			paymentEventPublisher: paymentPublisher,
			logger:                zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		operator := &models.User{UserId: primitive.NewObjectID()}
		order := &models.Order{
			ID:         orderID,
			Status:     constants.OrderStatusUnpaid.String(),
			MerchantID: primitive.NewObjectID(),
			User:       operator,
		}

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(order, nil).Once()
		orderRepo.On("GetOrderItemsByOrderID", mock.Anything, orderID).Return([]*models.OrderItem{}, nil).Once()

		bill := &models.Bill{
			ID:     primitive.NewObjectID(),
			Status: constants.BillStatusPending.String(),
			PaymentMethod: &models.PaymentMethodInfo{
				ID: primitive.NewObjectID(),
			},
		}
		billRepo.On("GetBillsByOrderID", mock.Anything, orderID).Return([]*models.Bill{bill}, nil).Once()
		billRepo.On("UpdateBill", mock.Anything, bill.ID, mock.Anything).Return(nil).Once()

		outboxRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.OutboxMessage")).Return(errors.New("outbox failure")).Once()

		err := l.CancelOrder(context.Background(), dto.NewCancelOrderRequest(orderID, operator, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outbox failure")
	})
}

func TestOrderLogic_RequestOrderItemRefund(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		ticketRepo := newMockTicketsRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		l := &orderLogic{
			orderRepo:    orderRepo,
			ticketRepo:   ticketRepo,
			billRepo:     billRepo,
			auditLogRepo: auditLogRepo,
			logger:       zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()
		operator := &models.User{UserId: primitive.NewObjectID()}

		orderItemWithOrder := &dto.OrderItemWithOrder{
			OrderItem: &models.OrderItem{
				ID:     itemID,
				Order:  orderID,
				Status: constants.OrderItemStatusFulfilled.String(),
				Price:  func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("100"); return p }(),
			},
			OrderInfo: &models.Order{
				ID:     orderID,
				User:   operator,
				Status: constants.OrderStatusUnpaid.String(),
				Total:  func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("100"); return p }(),
				Paid:   func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("0"); return p }(),
			},
		}

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(orderItemWithOrder, nil).Once()
		orderRepo.On("UpdateOrderItemStatus", mock.Anything, mock.MatchedBy(func(params *repository.UpdateOrderItemStatusParams) bool {
			return params.ItemID == itemID &&
				params.Status == constants.OrderItemStatusReturnRequested.String() &&
				params.ReturnInfo != nil &&
				params.ReturnInfo.Reason == "defective"
		})).Return(nil).Once()

		orderRepo.On("GetOrderWithItemsByID", mock.Anything, orderID).Return(&dto.OrderWithItems{
			Order: &models.Order{
				ID:     orderID,
				Status: constants.OrderStatusUnpaid.String(),
				Total:  func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("100"); return p }(),
				Paid:   func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("0"); return p }(),
			},
			Items: []*models.OrderItem{
				{Status: constants.OrderItemStatusPendingPayment.String()},
			},
		}, nil).Once()

		auditLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AuditLog")).Return(nil).Once()

		req := dto.NewRequestOrderItemRefundRequest(orderID, itemID, operator, "defective")
		err := l.RequestOrderItemRefund(context.Background(), req)

		assert.NoError(t, err)

		orderRepo.AssertExpectations(t)
		auditLogRepo.AssertExpectations(t)
	})

	t.Run("Invalid status transition", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()
		operator := &models.User{}

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(&dto.OrderItemWithOrder{
			OrderItem: &models.OrderItem{
				ID:     itemID,
				Order:  orderID,
				Status: constants.OrderItemStatusCanceled.String(),
			},
			OrderInfo: &models.Order{ID: orderID, User: operator},
		}, nil).Once()

		err := l.RequestOrderItemRefund(context.Background(), dto.NewRequestOrderItemRefundRequest(orderID, itemID, operator, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Repository fetch error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		itemID := primitive.NewObjectID()
		repoErr := errors.New("dao error")

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(nil, repoErr).Once()

		err := l.RequestOrderItemRefund(context.Background(), dto.NewRequestOrderItemRefundRequest(primitive.NewObjectID(), itemID, &models.User{}, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order item")

		orderRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_IsEventHasOpenOrders(t *testing.T) {
	orderRepo := newMockOrdersRepository()
	l := &orderLogic{
		orderRepo: orderRepo,
		logger:    zap.NewNop(),
	}

	eventID := primitive.NewObjectID()

	orderRepo.On("IsEventHasOpenOrders", mock.Anything, eventID).Return(true, nil).Once()

	hasOpen, err := l.IsEventHasOpenOrders(context.Background(), eventID)

	assert.NoError(t, err)
	assert.True(t, hasOpen)

	orderRepo.AssertExpectations(t)
}

func TestOrderLogic_CreateRefundBill(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		outboxRepo := newMockOutboxRepository()
		paymentPublisher := &PaymentEventPublisher{
			outboxRepo:        outboxRepo,
			paymentEventTopic: PaymentEventTopic("payments"),
		}

		l := &orderLogic{
			orderRepo:             orderRepo,
			billRepo:              billRepo,
			auditLogRepo:          auditLogRepo,
			paymentEventPublisher: paymentPublisher,
			logger:                zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		orderItemID := primitive.NewObjectID()
		operator := &models.User{UserId: primitive.NewObjectID()}
		paymentID := primitive.NewObjectID()
		requestAmount, _ := primitive.ParseDecimal128("80")

		orderRepo.On("GetOrderItemByID", mock.Anything, orderItemID).Return(&models.OrderItem{
			ID:     orderItemID,
			Order:  orderID,
			Status: constants.OrderItemStatusReturnRequested.String(),
			Price:  func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("100"); return p }(),
		}, nil).Once()

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&models.Order{
			ID:         orderID,
			User:       operator,
			MerchantID: primitive.NewObjectID(),
		}, nil).Once()

		billRepo.On("GetRefundsByOrderItemID", mock.Anything, orderItemID).Return([]*models.Bill{}, nil).Once()

		var createdBill *models.Bill
		billRepo.On("CreateBill", mock.Anything, mock.MatchedBy(func(b *models.Bill) bool {
			createdBill = b
			return b.Order == orderID &&
				b.OrderItemID != nil &&
				*b.OrderItemID == orderItemID &&
				b.Amount == requestAmount &&
				b.Type == constants.BillTypeRefund &&
				b.PaymentMethod.ID == paymentID
		})).Return(primitive.NewObjectID(), nil).Once()

		outboxRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.OutboxMessage")).Return(nil).Once()

		auditLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AuditLog")).Return(nil).Once()

		orderRepo.On("UpdateOrderItemStatus", mock.Anything, mock.MatchedBy(func(params *repository.UpdateOrderItemStatusParams) bool {
			return params.ItemID == orderItemID && params.Status == constants.OrderItemStatusReturnApproved.String()
		})).Return(nil).Once()

		orderRepo.On("UpdateOrder", mock.Anything, orderID, mock.Anything).Return(nil).Once()

		req := dto.NewCreateRefundBillRequest(orderItemID, requestAmount, dto.NewPaymentMethodInfoRequest(paymentID, "Card"), "approved", operator)
		billID, err := l.CreateRefundBill(context.Background(), req)

		assert.NoError(t, err)
		assert.NotEqual(t, primitive.NilObjectID, billID)
		assert.NotNil(t, createdBill)

		orderRepo.AssertExpectations(t)
		billRepo.AssertExpectations(t)
		auditLogRepo.AssertExpectations(t)
		outboxRepo.AssertExpectations(t)
	})

	t.Run("Order item fetch error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  newMockBillRepository(),
			logger:    zap.NewNop(),
		}

		orderItemID := primitive.NewObjectID()
		repoErr := errors.New("dao failed")

		orderRepo.On("GetOrderItemByID", mock.Anything, orderItemID).Return(nil, repoErr).Once()

		billID, err := l.CreateRefundBill(context.Background(), dto.NewCreateRefundBillRequest(orderItemID, primitive.Decimal128{}, dto.NewPaymentMethodInfoRequest(primitive.NewObjectID(), "Card"), "", &models.User{}))

		assert.Equal(t, primitive.NilObjectID, billID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order item")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Invalid item status", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  newMockBillRepository(),
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		orderItemID := primitive.NewObjectID()

		orderRepo.On("GetOrderItemByID", mock.Anything, orderItemID).Return(&models.OrderItem{
			ID:     orderItemID,
			Order:  orderID,
			Status: constants.OrderItemStatusFulfilled.String(),
		}, nil).Once()

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&models.Order{
			ID:         orderID,
			User:       &models.User{},
			MerchantID: primitive.NewObjectID(),
		}, nil).Once()

		billID, err := l.CreateRefundBill(context.Background(), dto.NewCreateRefundBillRequest(orderItemID, primitive.Decimal128{}, dto.NewPaymentMethodInfoRequest(primitive.NewObjectID(), "Card"), "", &models.User{}))

		assert.Equal(t, primitive.NilObjectID, billID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be refunded")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Existing refund prevents new bill", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		billRepo := newMockBillRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  billRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		orderItemID := primitive.NewObjectID()

		orderRepo.On("GetOrderItemByID", mock.Anything, orderItemID).Return(&models.OrderItem{
			ID:     orderItemID,
			Order:  orderID,
			Status: constants.OrderItemStatusReturnRequested.String(),
		}, nil).Once()

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&models.Order{
			ID:         orderID,
			User:       &models.User{},
			MerchantID: primitive.NewObjectID(),
		}, nil).Once()

		pendingBill := &models.Bill{
			ID:     primitive.NewObjectID(),
			Status: constants.BillStatusPending.String(),
		}
		billRepo.On("GetRefundsByOrderItemID", mock.Anything, orderItemID).Return([]*models.Bill{pendingBill}, nil).Once()

		billID, err := l.CreateRefundBill(context.Background(), dto.NewCreateRefundBillRequest(orderItemID, primitive.Decimal128{}, dto.NewPaymentMethodInfoRequest(primitive.NewObjectID(), "Card"), "", &models.User{}))

		assert.Equal(t, primitive.NilObjectID, billID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be refunded again")

		orderRepo.AssertExpectations(t)
		billRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_GetOrdersByEvent(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		eventID := primitive.NewObjectID()
		pageReq := pagination.NewPageRequest(2, 20)
		mockOrders := []*dto.OrderWithItems{
			{
				Order: &models.Order{ID: primitive.NewObjectID()},
			},
		}
		var total int64 = 35

		orderRepo.On("GetOrdersByEvent", mock.Anything, mock.MatchedBy(func(params *repository.GetOrdersByEventParams) bool {
			return params.EventID == eventID &&
				params.Limit == pageReq.GetLimit() &&
				params.Offset == pageReq.GetOffset()
		})).Return(mockOrders, total, nil).Once()

		result, err := l.GetOrdersByEvent(context.Background(), eventID, pageReq)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, total, result.Total)
		assert.Equal(t, pageReq.Page, result.Page)
		assert.Equal(t, pageReq.PageSize, result.PageSize)

		casted, ok := result.Data.([]*dto.OrderWithItems)
		assert.True(t, ok)
		assert.Equal(t, mockOrders, casted)

		orderRepo.AssertExpectations(t)
	})

	t.Run("Repository error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		eventID := primitive.NewObjectID()
		pageReq := pagination.NewPageRequest(1, 10)
		expectedErr := errors.New("db unavailable")

		orderRepo.On("GetOrdersByEvent", mock.Anything, mock.Anything).Return(nil, int64(0), expectedErr).Once()

		result, err := l.GetOrdersByEvent(context.Background(), eventID, pageReq)

		assert.Nil(t, result)
		assert.Equal(t, expectedErr, err)

		orderRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_GetOrdersBySession(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		sessionID := primitive.NewObjectID()
		pageReq := pagination.NewPageRequest(3, 15)
		mockOrders := []*dto.OrderWithItems{
			{
				Order: &models.Order{ID: primitive.NewObjectID()},
			},
		}
		var total int64 = 52

		orderRepo.On("GetOrdersBySession", mock.Anything, mock.MatchedBy(func(params *repository.GetOrdersBySessionParams) bool {
			return params.SessionID == sessionID &&
				params.Limit == pageReq.GetLimit() &&
				params.Offset == pageReq.GetOffset()
		})).Return(mockOrders, total, nil).Once()

		result, err := l.GetOrdersBySession(context.Background(), sessionID, pageReq)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, total, result.Total)
		assert.Equal(t, pageReq.Page, result.Page)
		assert.Equal(t, pageReq.PageSize, result.PageSize)

		casted, ok := result.Data.([]*dto.OrderWithItems)
		assert.True(t, ok)
		assert.Equal(t, mockOrders, casted)

		orderRepo.AssertExpectations(t)
	})

	t.Run("Repository error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		sessionID := primitive.NewObjectID()
		pageReq := pagination.NewPageRequest(1, 10)
		expectedErr := errors.New("db unavailable")

		orderRepo.On("GetOrdersBySession", mock.Anything, mock.Anything).Return(nil, int64(0), expectedErr).Once()

		result, err := l.GetOrdersBySession(context.Background(), sessionID, pageReq)

		assert.Nil(t, result)
		assert.Equal(t, expectedErr, err)

		orderRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_UpdateOrderItemStatus(t *testing.T) {
	t.Run("Success to return requested", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		ticketRepo := newMockTicketsRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		l := &orderLogic{
			orderRepo:    orderRepo,
			ticketRepo:   ticketRepo,
			billRepo:     billRepo,
			auditLogRepo: auditLogRepo,
			logger:       zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()
		operator := &models.User{UserId: primitive.NewObjectID()}

		itemPrice, _ := primitive.ParseDecimal128("100")
		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(&dto.OrderItemWithOrder{
			OrderItem: &models.OrderItem{
				ID:     itemID,
				Order:  orderID,
				Status: constants.OrderItemStatusFulfilled.String(),
				Price:  itemPrice,
			},
			OrderInfo: &models.Order{
				ID:     orderID,
				User:   operator,
				Status: constants.OrderStatusUnpaid.String(),
				Total:  itemPrice,
				Paid:   func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("0"); return p }(),
			},
		}, nil).Once()

		orderRepo.On("UpdateOrderItemStatus", mock.Anything, mock.MatchedBy(func(params *repository.UpdateOrderItemStatusParams) bool {
			return params.ItemID == itemID &&
				params.Status == constants.OrderItemStatusReturnRequested.String() &&
				params.ReturnInfo != nil
		})).Return(nil).Once()

		orderRepo.On("GetOrderWithItemsByID", mock.Anything, orderID).Return(&dto.OrderWithItems{
			Order: &models.Order{
				ID:     orderID,
				Status: constants.OrderStatusUnpaid.String(),
				Total:  itemPrice,
				Paid:   func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("0"); return p }(),
			},
			Items: []*models.OrderItem{
				{Status: constants.OrderItemStatusReturnRequested.String()},
			},
		}, nil).Once()

		auditLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AuditLog")).Return(nil).Once()

		req := dto.NewUpdateOrderItemStatusRequest(itemID, orderID, constants.OrderItemStatusReturnRequested.String(), "defective", operator)
		err := l.UpdateOrderItemStatus(context.Background(), req)

		assert.NoError(t, err)

		orderRepo.AssertExpectations(t)
		auditLogRepo.AssertExpectations(t)
	})

	t.Run("Invalid transition", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(&dto.OrderItemWithOrder{
			OrderItem: &models.OrderItem{
				ID:     itemID,
				Order:  orderID,
				Status: constants.OrderItemStatusPendingPayment.String(),
			},
			OrderInfo: &models.Order{ID: orderID, User: &models.User{}},
		}, nil).Once()

		err := l.UpdateOrderItemStatus(context.Background(), dto.NewUpdateOrderItemStatusRequest(itemID, orderID, constants.OrderItemStatusReturnRequested.String(), "", &models.User{}))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Repository fetch error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		itemID := primitive.NewObjectID()
		repoErr := errors.New("dao failure")

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(nil, repoErr).Once()

		err := l.UpdateOrderItemStatus(context.Background(), dto.NewUpdateOrderItemStatusRequest(itemID, primitive.NewObjectID(), constants.OrderItemStatusReturnRequested.String(), "", &models.User{}))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order item")

		orderRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_GetOrderDetails(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		billRepo := newMockBillRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  billRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		order := &models.Order{ID: orderID}
		items := []*models.OrderItem{{ID: primitive.NewObjectID()}}
		bills := []*models.Bill{{ID: primitive.NewObjectID()}}

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(order, nil).Once()
		orderRepo.On("GetOrderItemsByOrderID", mock.Anything, orderID).Return(items, nil).Once()
		billRepo.On("GetBillsByOrderID", mock.Anything, orderID).Return(bills, nil).Once()

		details, err := l.GetOrderDetails(context.Background(), orderID)

		assert.NoError(t, err)
		assert.NotNil(t, details)
		assert.Equal(t, order, details.Order)
		assert.Equal(t, items, details.Items)
		assert.Equal(t, bills, details.Bills)

		orderRepo.AssertExpectations(t)
		billRepo.AssertExpectations(t)
	})

	t.Run("Order fetch error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		billRepo := newMockBillRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  billRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		expectedErr := errors.New("dao order fail")

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(nil, expectedErr).Once()
		orderRepo.On("GetOrderItemsByOrderID", mock.Anything, orderID).Maybe().Return(nil, nil)
		billRepo.On("GetBillsByOrderID", mock.Anything, orderID).Maybe().Return(nil, nil)
		details, err := l.GetOrderDetails(context.Background(), orderID)

		assert.Nil(t, details)
		assert.Error(t, err)
		fmt.Println("err: ", err.Error())
		assert.Contains(t, err.Error(), "failed to get order by id")

		orderRepo.AssertExpectations(t)
		billRepo.AssertExpectations(t)
	})

	t.Run("Items fetch error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		billRepo := newMockBillRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  billRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&models.Order{ID: orderID}, nil).Once()
		orderRepo.On("GetOrderItemsByOrderID", mock.Anything, orderID).Return(nil, errors.New("items error")).Once()
		billRepo.On("GetBillsByOrderID", mock.Anything, orderID).Maybe().Return(nil, nil)

		details, err := l.GetOrderDetails(context.Background(), orderID)

		assert.Nil(t, details)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order items by order id")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Bills fetch error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		billRepo := newMockBillRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			billRepo:  billRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()

		orderRepo.On("GetOrderByID", mock.Anything, orderID).Return(&models.Order{ID: orderID}, nil).Once()
		orderRepo.On("GetOrderItemsByOrderID", mock.Anything, orderID).Return([]*models.OrderItem{}, nil).Once()
		billRepo.On("GetBillsByOrderID", mock.Anything, orderID).Return(nil, errors.New("bills error")).Once()

		details, err := l.GetOrderDetails(context.Background(), orderID)

		assert.Nil(t, details)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get bills by order id")

		orderRepo.AssertExpectations(t)
		billRepo.AssertExpectations(t)
	})
}

func TestOrderLogic_RejectOrderItemRefund(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		ticketRepo := newMockTicketsRepository()
		billRepo := newMockBillRepository()
		auditLogRepo := newMockAuditLogRepository()
		l := &orderLogic{
			orderRepo:    orderRepo,
			ticketRepo:   ticketRepo,
			billRepo:     billRepo,
			auditLogRepo: auditLogRepo,
			logger:       zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()
		operator := &models.User{UserId: primitive.NewObjectID()}

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(&dto.OrderItemWithOrder{
			OrderItem: &models.OrderItem{
				ID:     itemID,
				Order:  orderID,
				Status: constants.OrderItemStatusReturnRequested.String(),
			},
			OrderInfo: &models.Order{ID: orderID},
		}, nil).Once()

		orderRepo.On("UpdateOrderItemStatus", mock.Anything, mock.MatchedBy(func(params *repository.UpdateOrderItemStatusParams) bool {
			return params.ItemID == itemID &&
				params.Status == constants.OrderItemStatusFulfilled.String()
		})).Return(nil).Once()

		orderRepo.On("GetOrderWithItemsByID", mock.Anything, orderID).Return(&dto.OrderWithItems{
			Order: &models.Order{
				ID:     orderID,
				Status: constants.OrderStatusPaid.String(),
				Total:  primitive.Decimal128{},
				Paid:   primitive.Decimal128{},
			},
			Items: []*models.OrderItem{{Status: constants.OrderItemStatusPaid.String()}},
		}, nil).Once()

		orderRepo.On("UpdateOrder", mock.Anything, orderID, mock.Anything).Return(nil).Once()
		orderRepo.On("UpdateItemsStatusByOrder", mock.Anything, orderID, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), nil).Maybe()

		auditLogRepo.On("Create", mock.Anything, mock.AnythingOfType("*models.AuditLog")).Return(nil).Maybe()

		req := dto.NewRejectOrderItemRefundRequest(orderID, itemID, operator, "invalid reason")
		err := l.RejectOrderItemRefund(context.Background(), req)

		assert.NoError(t, err)

		orderRepo.AssertExpectations(t)
		auditLogRepo.AssertExpectations(t)
	})

	t.Run("Invalid status transition", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(&dto.OrderItemWithOrder{
			OrderItem: &models.OrderItem{
				ID:     itemID,
				Order:  orderID,
				Status: constants.OrderItemStatusFulfilled.String(),
			},
			OrderInfo: &models.Order{ID: orderID},
		}, nil).Once()

		err := l.RejectOrderItemRefund(context.Background(), dto.NewRejectOrderItemRefundRequest(orderID, itemID, &models.User{}, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid status transition")

		orderRepo.AssertExpectations(t)
	})

	t.Run("Fetch error", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    zap.NewNop(),
		}

		itemID := primitive.NewObjectID()
		repoErr := errors.New("dao fail")

		orderRepo.On("GetOrderItemWithOrder", mock.Anything, itemID).Return(nil, repoErr).Once()

		err := l.RejectOrderItemRefund(context.Background(), dto.NewRejectOrderItemRefundRequest(primitive.NewObjectID(), itemID, &models.User{}, "reason"))

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get order item")

		orderRepo.AssertExpectations(t)
	})
}
func TestOrderLogic_ExportEventOrdersByMonth(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		logger := zap.NewNop()

		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    logger,
		}

		eventID := primitive.NewObjectID()
		year, month := 2024, 11
		price, _ := primitive.ParseDecimal128("150")
		createdAt := time.Date(2024, time.November, 5, 15, 30, 0, 0, time.UTC)

		orderRepo.On("GetOrdersByEventAndMonth", mock.Anything, eventID, year, month).Return([]*dto.OrderWithItems{
			{
				Order: &models.Order{
					Serial:    123456789,
					Status:    constants.OrderStatusPaid.String(),
					CreatedAt: createdAt,
					User: &models.User{
						Name:  "Alice",
						Email: "alice@example.com",
					},
				},
				Items: []*models.OrderItem{
					{
						Name:   "VIP Pass",
						Price:  price,
						Status: constants.OrderItemStatusPaid.String(),
					},
				},
			},
		}, nil).Once()

		filename, data, err := l.ExportEventOrdersByMonth(context.Background(), eventID, year, month)

		assert.NoError(t, err)
		assert.Equal(t, "orders-2024-11.csv", filename)
		assert.NotEmpty(t, data)

		reader := csv.NewReader(bytes.NewReader(data))
		records, err := reader.ReadAll()
		assert.NoError(t, err)
		assert.Len(t, records, 2)
		assert.Equal(t, []string{"訂單編號", "訂單狀態", "建立日期", "消費者姓名", "電子信箱", "購買票券", "票券價格", "繳費狀態"}, records[0])
		assert.Equal(t, []string{"123456789", constants.OrderStatusPaid.String(), createdAt.Format(time.RFC3339), "Alice", "alice@example.com", "VIP Pass", price.String(), constants.OrderItemStatusPaid.String()}, records[1])

		orderRepo.AssertExpectations(t)
	})

	t.Run("RepoError", func(t *testing.T) {
		orderRepo := newMockOrdersRepository()
		logger := zap.NewNop()

		l := &orderLogic{
			orderRepo: orderRepo,
			logger:    logger,
		}

		eventID := primitive.NewObjectID()
		expectedErr := errors.New("db down")

		orderRepo.On("GetOrdersByEventAndMonth", mock.Anything, eventID, 2024, 10).Return(nil, expectedErr).Once()

		filename, data, err := l.ExportEventOrdersByMonth(context.Background(), eventID, 2024, 10)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get orders for export")
		assert.Equal(t, expectedErr, errors.Unwrap(err))
		assert.Equal(t, "", filename)
		assert.Nil(t, data)

		orderRepo.AssertExpectations(t)
	})
}

// mockOrdersRepository implements repository.OrdersRepository using testify/mock.
type mockOrdersRepository struct {
	mock.Mock
}

func newMockOrdersRepository() *mockOrdersRepository {
	return &mockOrdersRepository{}
}

func (m *mockOrdersRepository) CreateOrder(ctx context.Context, order *models.Order) (primitive.ObjectID, error) {
	args := m.Called(ctx, order)
	if oid := args.Get(0); oid != nil {
		return oid.(primitive.ObjectID), args.Error(1)
	}
	return primitive.NilObjectID, args.Error(1)
}

func (m *mockOrdersRepository) CreateOrderItems(ctx context.Context, orderID primitive.ObjectID, items []*models.OrderItem) error {
	args := m.Called(ctx, orderID, items)
	return args.Error(0)
}

func (m *mockOrdersRepository) IsEventHasOpenOrders(ctx context.Context, eventID primitive.ObjectID) (bool, error) {
	args := m.Called(ctx, eventID)
	return args.Bool(0), args.Error(1)
}

func (m *mockOrdersRepository) GetOrdersByEvent(ctx context.Context, params *repository.GetOrdersByEventParams) ([]*dto.OrderWithItems, int64, error) {
	args := m.Called(ctx, params)
	var orders []*dto.OrderWithItems
	if v := args.Get(0); v != nil {
		orders = v.([]*dto.OrderWithItems)
	}
	return orders, args.Get(1).(int64), args.Error(2)
}

func (m *mockOrdersRepository) GetOrdersBySession(ctx context.Context, params *repository.GetOrdersBySessionParams) ([]*dto.OrderWithItems, int64, error) {
	args := m.Called(ctx, params)
	var orders []*dto.OrderWithItems
	if v := args.Get(0); v != nil {
		orders = v.([]*dto.OrderWithItems)
	}
	return orders, args.Get(1).(int64), args.Error(2)
}

func (m *mockOrdersRepository) GetOrderByID(ctx context.Context, id primitive.ObjectID) (*models.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *mockOrdersRepository) GetOrderItemByID(ctx context.Context, itemID primitive.ObjectID) (*models.OrderItem, error) {
	args := m.Called(ctx, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OrderItem), args.Error(1)
}

func (m *mockOrdersRepository) GetOrderItemsByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.OrderItem, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.OrderItem), args.Error(1)
}

func (m *mockOrdersRepository) UpdateOrderItemStatus(ctx context.Context, params *repository.UpdateOrderItemStatusParams) error {
	args := m.Called(ctx, params)
	return args.Error(0)
}

func (m *mockOrdersRepository) UpdateOrderStatus(ctx context.Context, params *repository.UpdateOrderStatusParams) (*models.Order, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

func (m *mockOrdersRepository) GetOrdersByUser(ctx context.Context, params *repository.GetOrdersByUserParams) ([]*dto.OrderWithItems, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.OrderWithItems), args.Error(1)
}

func (m *mockOrdersRepository) GetOrderWithItemsByID(ctx context.Context, orderID primitive.ObjectID) (*dto.OrderWithItems, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.OrderWithItems), args.Error(1)
}

func (m *mockOrdersRepository) UpdateOrder(ctx context.Context, orderID primitive.ObjectID, opts ...repository.UpdateOption) error {
	args := m.Called(ctx, orderID, opts)
	return args.Error(0)
}

func (m *mockOrdersRepository) UpdateItemsStatusByOrder(ctx context.Context, orderID primitive.ObjectID, currentStatus string, newStatus string, operator *models.User) (int64, error) {
	args := m.Called(ctx, orderID, currentStatus, newStatus, operator)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockOrdersRepository) GetOrderItemWithOrder(ctx context.Context, orderItemID primitive.ObjectID) (*dto.OrderItemWithOrder, error) {
	args := m.Called(ctx, orderItemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.OrderItemWithOrder), args.Error(1)
}

func (m *mockOrdersRepository) GetOrdersByEventAndMonth(ctx context.Context, eventID primitive.ObjectID, year int, month int) ([]*dto.OrderWithItems, error) {
	args := m.Called(ctx, eventID, year, month)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.OrderWithItems), args.Error(1)
}

type mockTicketsRepository struct {
	mock.Mock
}

func newMockTicketsRepository() *mockTicketsRepository {
	return &mockTicketsRepository{}
}

func (m *mockTicketsRepository) AddTicketType(ctx context.Context, t *models.TicketType) (primitive.ObjectID, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) AddTicketStock(ctx context.Context, t *models.TicketStock) error {
	panic("not implemented")
}

func (m *mockTicketsRepository) GetTicketTypesWithStockByEvent(ctx context.Context, eid primitive.ObjectID, opts ...repository.GetTicketsOption) ([]dto.TicketTypeWithStock, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) DeleteTicketTypeByID(ctx context.Context, tcid primitive.ObjectID) (*models.TicketType, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) DeleteTicketStockByType(ctx context.Context, tcid primitive.ObjectID) error {
	panic("not implemented")
}

func (m *mockTicketsRepository) UpdateTicketType(ctx context.Context, t *models.TicketType) error {
	panic("not implemented")
}

func (m *mockTicketsRepository) UpdateTicketTypesOrder(ctx context.Context, eid primitive.ObjectID, ids []primitive.ObjectID, oper *models.User) error {
	panic("not implemented")
}

func (m *mockTicketsRepository) GetTicketTypeByID(ctx context.Context, tid primitive.ObjectID) (*models.TicketType, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) GetValidTicketsByIDs(ctx context.Context, ids []primitive.ObjectID) ([]dto.TicketStockWithType, error) {
	args := m.Called(ctx, ids)
	var tickets []dto.TicketStockWithType
	if v := args.Get(0); v != nil {
		tickets = v.([]dto.TicketStockWithType)
	}
	return tickets, args.Error(1)
}

func (m *mockTicketsRepository) ReleaseTickets(ctx context.Context, items []*models.OrderItem) error {
	args := m.Called(ctx, items)
	return args.Error(0)
}

func (m *mockTicketsRepository) ReserveTickets(ctx context.Context, items []*models.OrderItem, user *models.User) error {
	args := m.Called(ctx, items, user)
	return args.Error(0)
}

func (m *mockTicketsRepository) GetTicketStockWithTypeBySession(ctx context.Context, sessionID primitive.ObjectID) ([]dto.TicketStockWithType, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) CountTicketsByOrder(ctx context.Context, orderID primitive.ObjectID) (int64, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) CreateManyTickets(ctx context.Context, tickets []interface{}) error {
	panic("not implemented")
}

func (m *mockTicketsRepository) GetTicketByOrderItemID(ctx context.Context, orderItemID primitive.ObjectID) (*models.Ticket, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) UpdateTicketStatus(ctx context.Context, ticketID primitive.ObjectID, status string) error {
	panic("not implemented")
}

func (m *mockTicketsRepository) GetTicketByID(ctx context.Context, ticketID primitive.ObjectID) (*models.Ticket, error) {
	panic("not implemented")
}

func (m *mockTicketsRepository) MarkTicketAsUsed(ctx context.Context, ticketID primitive.ObjectID, usedAt time.Time, operator *models.User) error {
	panic("not implemented")
}

func (m *mockTicketsRepository) ExpireTickets(ctx context.Context, now time.Time) (int64, error) {
	panic("not implemented")
}

type mockBillRepository struct {
	mock.Mock
}

func newMockBillRepository() *mockBillRepository {
	return &mockBillRepository{}
}

func (m *mockBillRepository) CreateBill(ctx context.Context, bill *models.Bill) (primitive.ObjectID, error) {
	args := m.Called(ctx, bill)
	return args.Get(0).(primitive.ObjectID), args.Error(1)
}

func (m *mockBillRepository) GetBillsByOrderID(ctx context.Context, orderID primitive.ObjectID) ([]*models.Bill, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Bill), args.Error(1)
}

func (m *mockBillRepository) GetBillByID(ctx context.Context, id primitive.ObjectID) (*models.Bill, error) {
	panic("not implemented")
}

func (m *mockBillRepository) UpdateBill(ctx context.Context, id primitive.ObjectID, opts ...repository.UpdateOption) error {
	args := m.Called(ctx, id, opts)
	return args.Error(0)
}

func (m *mockBillRepository) GetRefundsByOrderItemID(ctx context.Context, orderItemID primitive.ObjectID) ([]*models.Bill, error) {
	args := m.Called(ctx, orderItemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Bill), args.Error(1)
}

type mockAuditLogRepository struct {
	mock.Mock
}

func newMockAuditLogRepository() *mockAuditLogRepository {
	return &mockAuditLogRepository{}
}

func (m *mockAuditLogRepository) Create(ctx context.Context, log *models.AuditLog) error {
	args := m.Called(ctx, log)
	return args.Error(0)
}

type mockOutboxRepository struct {
	mock.Mock
}

func newMockOutboxRepository() *mockOutboxRepository {
	return &mockOutboxRepository{}
}

func (m *mockOutboxRepository) Create(ctx context.Context, message *models.OutboxMessage) error {
	args := m.Called(ctx, message)
	return args.Error(0)
}

func (m *mockOutboxRepository) ClaimAndFetchEvents(ctx context.Context, limit int) ([]*models.OutboxMessage, error) {
	panic("not implemented")
}

func (m *mockOutboxRepository) MarkAsProcessed(ctx context.Context, id primitive.ObjectID) error {
	panic("not implemented")
}

func (m *mockOutboxRepository) IncrementRetry(ctx context.Context, id primitive.ObjectID, errorMessage string) error {
	panic("not implemented")
}
