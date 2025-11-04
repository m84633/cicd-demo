package service

import (
	"context"
	"errors"
	"partivo_tickets/api/orders"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/models"
	"partivo_tickets/pkg/pagination"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// MockOrderLogic is a mock for logic.OrderLogic
type MockOrderLogic struct {
	mock.Mock
}

// GetOrdersByUser mocks the GetOrdersByUser method
func (m *MockOrderLogic) GetOrdersByUser(ctx context.Context, userID primitive.ObjectID, token pagination.PageToken) ([]*dto.OrderWithItems, pagination.PageToken, error) {
	args := m.Called(ctx, userID, token)
	var res0 []*dto.OrderWithItems
	if args.Get(0) != nil {
		res0 = args.Get(0).([]*dto.OrderWithItems)
	}
	var res1 pagination.PageToken
	if args.Get(1) != nil {
		res1 = args.Get(1).(pagination.PageToken)
	}
	return res0, res1, args.Error(2)
}

// AddOrder mocks the AddOrder method
func (m *MockOrderLogic) AddOrder(ctx context.Context, req *dto.AddOrderRequest) (*dto.AddOrderResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.AddOrderResponse), args.Error(1)
}

// CancelOrder mocks the CancelOrder method
func (m *MockOrderLogic) CancelOrder(ctx context.Context, req *dto.CancelOrderRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

// GetOrder mocks the GetOrder method
func (m *MockOrderLogic) GetOrder(ctx context.Context, oid primitive.ObjectID) (*models.Order, error) {
	args := m.Called(ctx, oid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Order), args.Error(1)
}

// RequestOrderItemRefund mocks the RequestOrderItemRefund method
func (m *MockOrderLogic) RequestOrderItemRefund(ctx context.Context, req *dto.RequestOrderItemRefundRequest) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockOrderLogic) IsEventHasOpenOrders(ctx context.Context, eid primitive.ObjectID) (bool, error) {
	args := m.Called(ctx, eid)
	return args.Bool(0), args.Error(1)
}

func (m *MockOrderLogic) CreateRefundBill(ctx context.Context, d *dto.CreateRefundBillRequest) (primitive.ObjectID, error) {
	args := m.Called(ctx, d)
	return args.Get(0).(primitive.ObjectID), args.Error(1)
}

func (m *MockOrderLogic) GetOrdersByEvent(ctx context.Context, eid primitive.ObjectID, pageReq *pagination.PageRequest) (*pagination.PageResult, error) {
	args := m.Called(ctx, eid, pageReq)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pagination.PageResult), args.Error(1)
}

func (m *MockOrderLogic) GetOrdersBySession(ctx context.Context, sid primitive.ObjectID, pageReq *pagination.PageRequest) (*pagination.PageResult, error) {
	args := m.Called(ctx, sid, pageReq)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*pagination.PageResult), args.Error(1)
}

func (m *MockOrderLogic) UpdateOrderItemStatus(ctx context.Context, d *dto.UpdateOrderItemStatusRequest) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}

func (m *MockOrderLogic) GetOrderDetails(ctx context.Context, orderID primitive.ObjectID) (*dto.OrderDetails, error) {
	args := m.Called(ctx, orderID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.OrderDetails), args.Error(1)
}

func (m *MockOrderLogic) RejectOrderItemRefund(ctx context.Context, d *dto.RejectOrderItemRefundRequest) error {
	args := m.Called(ctx, d)
	return args.Error(0)
}

func (m *MockOrderLogic) ExportEventOrdersByMonth(ctx context.Context, eventID primitive.ObjectID, year int, month int) (string, []byte, error) {
	args := m.Called(ctx, eventID, year, month)
	var r0 string
	if args.Get(0) != nil {
		r0 = args.Get(0).(string)
	}
	var r1 []byte
	if args.Get(1) != nil {
		r1 = args.Get(1).([]byte)
	}
	return r0, r1, args.Error(2)
}

// MockTransactionManager is a mock for db.TransactionManager
type MockTransactionManager struct {
	mock.Mock
}

func (m *MockTransactionManager) WithTransaction(ctx context.Context, fn func(sessCtx context.Context) (interface{}, error)) (interface{}, error) {
	// For unit tests, we can just execute the function directly without a real transaction.
	// The 'fn' contains the call to the logic layer mock.
	return fn(ctx)
}

func TestOrderService_GetUserOrders(t *testing.T) {
	logger := zap.NewNop() // Use a no-op logger for tests

	t.Run("Success", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		orderService := NewOrderService(mockLogic, logger, mockTM)

		testUserID := primitive.NewObjectID()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(userID, testUserID.Hex()))

		// Prepare mock data
		mockOrder := &dto.OrderWithItems{
			Order: &models.Order{
				ID:    primitive.NewObjectID(),
				Total: func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("100"); return p }(),
				Event: primitive.NewObjectID(),
			},
			Items: []*models.OrderItem{
				{
					TicketStock: primitive.NewObjectID(),
					Session:     primitive.NewObjectID(),
					Quantity:    2,
					Price:       func() primitive.Decimal128 { p, _ := primitive.ParseDecimal128("50"); return p }(),
					Name:        "General Admission",
				},
			},
		}
		mockOrders := []*dto.OrderWithItems{mockOrder}
		var mockToken pagination.PageToken = "next_page_token"

		// Define mock expectation
		mockLogic.On("GetOrdersByUser", mock.Anything, testUserID, pagination.PageToken("")).
			Return(mockOrders, mockToken, nil).
			Once() // Expect this to be called once

		// Execute the method
		req := &orders.GetUserOrdersRequest{}
		res, err := orderService.GetUserOrders(ctx, req)

		// Assertions
		assert.NoError(t, err)
		assert.NotNil(t, res)

		// Unpack the response
		var resBody orders.GetUserOrdersResponse
		err = res.Data.UnmarshalTo(&resBody)
		assert.NoError(t, err)

		assert.Len(t, resBody.Data, 1)
		assert.Equal(t, mockOrder.Order.ID.Hex(), resBody.Data[0].Id)
		assert.Equal(t, "next_page_token", resBody.After)

		// Verify that the mock was called as expected
		mockLogic.AssertExpectations(t)
	})

	t.Run("LogicError", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		orderService := NewOrderService(mockLogic, logger, mockTM)

		testUserID := primitive.NewObjectID()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(userID, testUserID.Hex()))

		expectedErr := errors.New("database is down")
		mockLogic.On("GetOrdersByUser", mock.Anything, testUserID, pagination.PageToken("")).
			Return(nil, nil, expectedErr).
			Once()

		// Execute the method
		req := &orders.GetUserOrdersRequest{}
		res, err := orderService.GetUserOrders(ctx, req)

		// Assertions
		assert.Nil(t, res)
		assert.Error(t, err)

		// Check gRPC status code
		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Equal(t, expectedErr.Error(), st.Message())

		// Verify that the mock was called as expected
		mockLogic.AssertExpectations(t)
	})

	t.Run("InvalidToken", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		orderService := NewOrderService(mockLogic, logger, mockTM)

		testUserID := primitive.NewObjectID()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(userID, testUserID.Hex()))

		mockLogic.On("GetOrdersByUser", mock.Anything, testUserID, pagination.PageToken("")).
			Return(nil, nil, pagination.ErrInvalidToken).
			Once()

		req := &orders.GetUserOrdersRequest{}
		res, err := orderService.GetUserOrders(ctx, req)

		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Equal(t, pagination.ErrInvalidToken.Error(), st.Message())

		mockLogic.AssertExpectations(t)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		orderService := NewOrderService(mockLogic, logger, mockTM)

		req := &orders.GetUserOrdersRequest{}
		res, err := orderService.GetUserOrders(context.Background(), req)

		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())

		mockLogic.AssertNotCalled(t, "GetOrdersByUser", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestOrderService_AddOrder(t *testing.T) {
	logger := zap.NewNop()

	newAuthContext := func(uid primitive.ObjectID, kv ...string) context.Context {
		metadataPairs := append([]string{userID, uid.Hex()}, kv...)
		return metadata.NewIncomingContext(context.Background(), metadata.Pairs(metadataPairs...))
	}

	buildRequest := func(eventID, merchantID, paymentID primitive.ObjectID, ticketIDs []primitive.ObjectID, quantity uint32) *orders.AddOrderRequest {
		items := make([]*orders.AddOrderRequest_Item, 0, len(ticketIDs))
		for _, tid := range ticketIDs {
			items = append(items, &orders.AddOrderRequest_Item{
				Ticket:   tid.Hex(),
				Quantity: quantity,
			})
		}
		return &orders.AddOrderRequest{
			Event:      eventID.Hex(),
			MerchantId: merchantID.Hex(),
			Subtotal:   "100.00",
			Total:      "120.00",
			Items:      items,
			PaymentMethod: &orders.AddOrderRequest_PaymentMethod{
				Id:   paymentID.Hex(),
				Name: "Credit Card",
			},
		}
	}

	t.Run("Success", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		userID := primitive.NewObjectID()
		eventID := primitive.NewObjectID()
		merchantID := primitive.NewObjectID()
		paymentID := primitive.NewObjectID()
		ticketID := primitive.NewObjectID()

		req := buildRequest(eventID, merchantID, paymentID, []primitive.ObjectID{ticketID}, 2)
		ctx := newAuthContext(userID, userName, "Alice", userEmail, "alice@example.com")

		addOrderResp := &dto.AddOrderResponse{
			OrderID: primitive.NewObjectID(),
			BillID:  primitive.NewObjectID(),
		}

		mockLogic.On("AddOrder", mock.Anything, mock.MatchedBy(func(d *dto.AddOrderRequest) bool {
			if d == nil {
				return false
			}
			if d.GetEvent() != eventID || d.GetMerchantID() != merchantID {
				return false
			}
			if d.GetSubtotal() != req.GetSubtotal() || d.GetTotal() != req.GetTotal() {
				return false
			}
			if d.GetUser() == nil || d.GetUser().UserId != userID {
				return false
			}
			if pm := d.GetPaymentMethod(); pm == nil || pm.ID() != paymentID || pm.Name() != req.GetPaymentMethod().GetName() {
				return false
			}
			if len(d.GetDetails()) != 1 {
				return false
			}
			detail := d.GetDetails()[0]
			if detail.TicketStock() != ticketID || detail.Quantity() != req.GetItems()[0].GetQuantity() {
				return false
			}
			return true
		})).Return(addOrderResp, nil).Once()

		res, err := service.AddOrder(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		var body orders.AddOrderResponse
		err = res.Data.UnmarshalTo(&body)
		assert.NoError(t, err)
		assert.Equal(t, addOrderResp.OrderID.Hex(), body.GetOrderId())
		assert.Equal(t, addOrderResp.BillID.Hex(), body.GetBillId())

		mockLogic.AssertExpectations(t)
	})

	t.Run("LogicError", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		userID := primitive.NewObjectID()
		eventID := primitive.NewObjectID()
		merchantID := primitive.NewObjectID()
		paymentID := primitive.NewObjectID()
		ticketID := primitive.NewObjectID()

		req := buildRequest(eventID, merchantID, paymentID, []primitive.ObjectID{ticketID}, 1)
		ctx := newAuthContext(userID)

		expectedErr := errors.New("failed to add order")
		mockLogic.On("AddOrder", mock.Anything, mock.AnythingOfType("*dto.AddOrderRequest")).
			Return(nil, expectedErr).
			Once()

		res, err := service.AddOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Equal(t, expectedErr.Error(), st.Message())

		mockLogic.AssertExpectations(t)
	})

	t.Run("InvalidEventID", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		req := &orders.AddOrderRequest{
			Event:      "invalid",
			MerchantId: primitive.NewObjectID().Hex(),
			Subtotal:   "100.00",
			Total:      "120.00",
			Items: []*orders.AddOrderRequest_Item{
				{
					Ticket:   primitive.NewObjectID().Hex(),
					Quantity: 1,
				},
			},
			PaymentMethod: &orders.AddOrderRequest_PaymentMethod{
				Id:   primitive.NewObjectID().Hex(),
				Name: "Credit Card",
			},
		}

		res, err := service.AddOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		mockLogic.AssertNotCalled(t, "AddOrder", mock.Anything, mock.Anything)
	})

	t.Run("InvalidMerchantID", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		req := &orders.AddOrderRequest{
			Event:      primitive.NewObjectID().Hex(),
			MerchantId: "invalid",
			Subtotal:   "100.00",
			Total:      "120.00",
			Items: []*orders.AddOrderRequest_Item{
				{
					Ticket:   primitive.NewObjectID().Hex(),
					Quantity: 1,
				},
			},
			PaymentMethod: &orders.AddOrderRequest_PaymentMethod{
				Id:   primitive.NewObjectID().Hex(),
				Name: "Credit Card",
			},
		}

		res, err := service.AddOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		mockLogic.AssertNotCalled(t, "AddOrder", mock.Anything, mock.Anything)
	})

	t.Run("InvalidPaymentMethodID", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		req := &orders.AddOrderRequest{
			Event:      primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
			Subtotal:   "100.00",
			Total:      "120.00",
			Items: []*orders.AddOrderRequest_Item{
				{
					Ticket:   primitive.NewObjectID().Hex(),
					Quantity: 1,
				},
			},
			PaymentMethod: &orders.AddOrderRequest_PaymentMethod{
				Id:   "invalid",
				Name: "Credit Card",
			},
		}

		res, err := service.AddOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		mockLogic.AssertNotCalled(t, "AddOrder", mock.Anything, mock.Anything)
	})

	t.Run("InvalidTicketID", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		req := &orders.AddOrderRequest{
			Event:      primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
			Subtotal:   "100.00",
			Total:      "120.00",
			Items: []*orders.AddOrderRequest_Item{
				{
					Ticket:   "invalid",
					Quantity: 1,
				},
			},
			PaymentMethod: &orders.AddOrderRequest_PaymentMethod{
				Id:   primitive.NewObjectID().Hex(),
				Name: "Credit Card",
			},
		}

		res, err := service.AddOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		mockLogic.AssertNotCalled(t, "AddOrder", mock.Anything, mock.Anything)
	})

	t.Run("DuplicateTicket", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		ticketID := primitive.NewObjectID().Hex()
		req := &orders.AddOrderRequest{
			Event:      primitive.NewObjectID().Hex(),
			MerchantId: primitive.NewObjectID().Hex(),
			Subtotal:   "100.00",
			Total:      "120.00",
			Items: []*orders.AddOrderRequest_Item{
				{
					Ticket:   ticketID,
					Quantity: 1,
				},
				{
					Ticket:   ticketID,
					Quantity: 2,
				},
			},
			PaymentMethod: &orders.AddOrderRequest_PaymentMethod{
				Id:   primitive.NewObjectID().Hex(),
				Name: "Credit Card",
			},
		}

		res, err := service.AddOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
		assert.Equal(t, "duplicate ticket", st.Message())

		mockLogic.AssertNotCalled(t, "AddOrder", mock.Anything, mock.Anything)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		req := &orders.AddOrderRequest{}
		res, err := service.AddOrder(context.Background(), req)

		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())

		mockLogic.AssertNotCalled(t, "AddOrder", mock.Anything, mock.Anything)
	})
}

func TestOrderService_CancelOrder(t *testing.T) {
	logger := zap.NewNop()

	newAuthContext := func(uid primitive.ObjectID, kv ...string) context.Context {
		pairs := append([]string{userID, uid.Hex()}, kv...)
		return metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...))
	}

	t.Run("Success", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		userID := primitive.NewObjectID()
		ctx := newAuthContext(userID, userName, "Alice", userEmail, "alice@example.com", userAvatar, "avatar.png")

		mockLogic.On("GetOrder", mock.Anything, orderID).Return(&models.Order{
			ID: orderID,
			User: &models.User{
				UserId: userID,
			},
		}, nil).Once()

		mockLogic.On("CancelOrder", mock.Anything, mock.MatchedBy(func(req *dto.CancelOrderRequest) bool {
			if req == nil {
				return false
			}
			if req.GetOrderID() != orderID {
				return false
			}
			if req.GetReason() != "No longer needed" {
				return false
			}
			user := req.GetUser()
			if user == nil {
				return false
			}
			if user.UserId != userID {
				return false
			}
			if user.Name != "Alice" {
				return false
			}
			if user.Email != "alice@example.com" {
				return false
			}
			if user.Avatar != "avatar.png" {
				return false
			}
			return true
		})).Return(nil).Once()

		errMessage := "No longer needed"
		req := &orders.CancelOrderRequest{
			Order:  orderID.Hex(),
			Reason: &errMessage,
		}

		res, err := service.CancelOrder(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Nil(t, res.Data)

		mockLogic.AssertExpectations(t)
	})

	t.Run("InvalidOrderID", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		req := &orders.CancelOrderRequest{Order: "invalid"}

		res, err := service.CancelOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		mockLogic.AssertNotCalled(t, "GetOrder", mock.Anything, mock.Anything)
		mockLogic.AssertNotCalled(t, "CancelOrder", mock.Anything, mock.Anything)
	})

	t.Run("AuthorizePermissionDenied", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		userID := primitive.NewObjectID()
		ctx := newAuthContext(userID)

		mockLogic.On("GetOrder", mock.Anything, orderID).Return(&models.Order{
			ID: orderID,
			User: &models.User{
				UserId: primitive.NewObjectID(),
			},
		}, nil).Once()

		req := &orders.CancelOrderRequest{
			Order: orderID.Hex(),
		}

		res, err := service.CancelOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code())
		assert.Equal(t, ErrPermissionDenied.Error(), st.Message())

		mockLogic.AssertExpectations(t)
		mockLogic.AssertNotCalled(t, "CancelOrder", mock.Anything, mock.Anything)
	})

	t.Run("AuthorizeGetOrderError", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		userID := primitive.NewObjectID()
		ctx := newAuthContext(userID)

		expectedErr := errors.New("order lookup failed")
		mockLogic.On("GetOrder", mock.Anything, orderID).Return(nil, expectedErr).Once()

		req := &orders.CancelOrderRequest{
			Order: orderID.Hex(),
		}

		res, err := service.CancelOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code())
		assert.Equal(t, expectedErr.Error(), st.Message())

		mockLogic.AssertExpectations(t)
		mockLogic.AssertNotCalled(t, "CancelOrder", mock.Anything, mock.Anything)
	})

	t.Run("LogicError", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		userID := primitive.NewObjectID()
		ctx := newAuthContext(userID)

		mockLogic.On("GetOrder", mock.Anything, orderID).Return(&models.Order{
			ID: orderID,
			User: &models.User{
				UserId: userID,
			},
		}, nil).Once()

		expectedErr := errors.New("cancel failed")
		mockLogic.On("CancelOrder", mock.Anything, mock.AnythingOfType("*dto.CancelOrderRequest")).Return(expectedErr).Once()

		errMessage := "Cleanup"
		req := &orders.CancelOrderRequest{
			Order:  orderID.Hex(),
			Reason: &errMessage,
		}

		res, err := service.CancelOrder(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Equal(t, expectedErr.Error(), st.Message())

		mockLogic.AssertExpectations(t)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		req := &orders.CancelOrderRequest{
			Order: orderID.Hex(),
		}

		res, err := service.CancelOrder(context.Background(), req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code())

		mockLogic.AssertNotCalled(t, "GetOrder", mock.Anything, mock.Anything)
		mockLogic.AssertNotCalled(t, "CancelOrder", mock.Anything, mock.Anything)
	})
}

func TestOrderService_RequestOrderItemRefund(t *testing.T) {
	logger := zap.NewNop()

	newAuthContext := func(uid primitive.ObjectID, kv ...string) context.Context {
		pairs := append([]string{userID, uid.Hex()}, kv...)
		return metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...))
	}

	t.Run("Success", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()
		userID := primitive.NewObjectID()
		ctx := newAuthContext(userID, userName, "Alice", userEmail, "alice@example.com", userAvatar, "avatar.png")

		mockLogic.On("GetOrder", mock.Anything, orderID).Return(&models.Order{
			ID: orderID,
			User: &models.User{
				UserId: userID,
			},
		}, nil).Once()

		mockLogic.On("RequestOrderItemRefund", mock.Anything, mock.MatchedBy(func(req *dto.RequestOrderItemRefundRequest) bool {
			if req == nil {
				return false
			}
			if req.GetOrderID() != orderID {
				return false
			}
			if req.GetOrderItemID() != itemID {
				return false
			}
			if req.GetReason() != "Customer requested refund" {
				return false
			}
			user := req.GetOperator()
			if user == nil {
				return false
			}
			if user.UserId != userID {
				return false
			}
			if user.Name != "Alice" || user.Email != "alice@example.com" || user.Avatar != "avatar.png" {
				return false
			}
			return true
		})).Return(nil).Once()

		req := &orders.RequestOrderItemRefundRequest{
			Order:  orderID.Hex(),
			Item:   itemID.Hex(),
			Reason: "Customer requested refund",
		}

		res, err := service.RequestOrderItemRefund(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Nil(t, res.Data)

		mockLogic.AssertExpectations(t)
	})

	t.Run("InvalidOrderID", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		req := &orders.RequestOrderItemRefundRequest{
			Order: "invalid",
			Item:  primitive.NewObjectID().Hex(),
		}

		res, err := service.RequestOrderItemRefund(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		mockLogic.AssertNotCalled(t, "GetOrder", mock.Anything, mock.Anything)
		mockLogic.AssertNotCalled(t, "RequestOrderItemRefund", mock.Anything, mock.Anything)
	})

	t.Run("InvalidItemID", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		ctx := newAuthContext(primitive.NewObjectID())
		req := &orders.RequestOrderItemRefundRequest{
			Order: primitive.NewObjectID().Hex(),
			Item:  "invalid",
		}

		res, err := service.RequestOrderItemRefund(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())

		mockLogic.AssertNotCalled(t, "GetOrder", mock.Anything, mock.Anything)
		mockLogic.AssertNotCalled(t, "RequestOrderItemRefund", mock.Anything, mock.Anything)
	})

	t.Run("PermissionDenied", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()
		userID := primitive.NewObjectID()
		ctx := newAuthContext(userID)

		mockLogic.On("GetOrder", mock.Anything, orderID).Return(&models.Order{
			ID: orderID,
			User: &models.User{
				UserId: primitive.NewObjectID(),
			},
		}, nil).Once()

		req := &orders.RequestOrderItemRefundRequest{
			Order: orderID.Hex(),
			Item:  itemID.Hex(),
		}

		res, err := service.RequestOrderItemRefund(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code())
		assert.Equal(t, ErrPermissionDenied.Error(), st.Message())

		mockLogic.AssertExpectations(t)
		mockLogic.AssertNotCalled(t, "RequestOrderItemRefund", mock.Anything, mock.Anything)
	})

	t.Run("LogicError", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		orderID := primitive.NewObjectID()
		itemID := primitive.NewObjectID()
		userID := primitive.NewObjectID()
		ctx := newAuthContext(userID)

		mockLogic.On("GetOrder", mock.Anything, orderID).Return(&models.Order{
			ID: orderID,
			User: &models.User{
				UserId: userID,
			},
		}, nil).Once()

		expectedErr := errors.New("refund failed")
		mockLogic.On("RequestOrderItemRefund", mock.Anything, mock.AnythingOfType("*dto.RequestOrderItemRefundRequest")).Return(expectedErr).Once()

		req := &orders.RequestOrderItemRefundRequest{
			Order:  orderID.Hex(),
			Item:   itemID.Hex(),
			Reason: "Out of stock",
		}

		res, err := service.RequestOrderItemRefund(ctx, req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Internal, st.Code())
		assert.Equal(t, expectedErr.Error(), st.Message())

		mockLogic.AssertExpectations(t)
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		mockLogic := new(MockOrderLogic)
		mockTM := new(MockTransactionManager)
		service := NewOrderService(mockLogic, logger, mockTM)

		req := &orders.RequestOrderItemRefundRequest{
			Order: primitive.NewObjectID().Hex(),
			Item:  primitive.NewObjectID().Hex(),
		}

		res, err := service.RequestOrderItemRefund(context.Background(), req)
		assert.Nil(t, res)
		assert.Error(t, err)

		st, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())

		mockLogic.AssertNotCalled(t, "GetOrder", mock.Anything, mock.Anything)
		mockLogic.AssertNotCalled(t, "RequestOrderItemRefund", mock.Anything, mock.Anything)
	})
}
