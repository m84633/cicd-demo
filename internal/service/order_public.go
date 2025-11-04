package service

import (
	"context"
	"errors"
	"partivo_tickets/api"
	"partivo_tickets/api/orders"
	"partivo_tickets/internal/db"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/logic"
	"partivo_tickets/internal/models"
	"partivo_tickets/pkg/pagination"
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

var ErrPermissionDenied = errors.New("permission denied")

type OrderService struct {
	orders.UnimplementedOrderServiceServer
	logic  logic.OrderLogic
	logger *zap.Logger
	tm     db.TransactionManager
}

func NewOrderService(l logic.OrderLogic, logger *zap.Logger, tm db.TransactionManager) *OrderService {
	return &OrderService{
		logic:  l,
		logger: logger.Named("OrderService"),
		tm:     tm,
	}
}

// authorizeOrderConsumer verifies the user from the context owns the order.
// It returns the fetched order, the user's ID, and a standard error on failure.
func (s *OrderService) authorizeOrderConsumer(ctx context.Context, oid primitive.ObjectID) (primitive.ObjectID, error) {
	uid, err := getUserId(ctx)
	if err != nil {
		return primitive.NilObjectID, err // This err is already codes.Unauthenticated
	}

	order, err := s.logic.GetOrder(ctx, oid)
	if err != nil {
		return uid, err // This could be db.ErrNotFound or other internal errors
	}

	if order.User.UserId != uid {
		s.logger.Warn("authorizeOrderConsumer: Permission denied",
			zap.Stringer("user_id", uid),
			zap.Stringer("order_id", oid),
			zap.Stringer("owner_id", order.User.UserId))
		return uid, ErrPermissionDenied
	}

	return uid, nil
}

func (s *OrderService) GetUserOrders(ctx context.Context, in *orders.GetUserOrdersRequest) (*api.Response, error) {
	uid, err := getUserId(ctx)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}

	// Get the token from the incoming request
	token := pagination.PageToken(in.GetAfter())

	// Call the logic layer, passing the token.
	// This now expects GetOrdersByUser to handle pagination and return a nextToken.
	ol, nextToken, err := s.logic.GetOrdersByUser(ctx, uid, token)
	if err != nil {
		if errors.Is(err, pagination.ErrInvalidToken) {
			s.logger.Warn("GetUserOrders: Invalid pagination token", zap.Error(err), zap.String("token", string(token)))
			return ResponseError(codes.InvalidArgument, err)
		}
		s.logger.Error("GetUserOrders: logic.GetOrdersByUser failed", zap.Error(err), zap.Stringer("user_id", uid))
		return ResponseError(codes.Internal, err)
	}

	//response
	res := make([]*orders.GetUserOrdersResponse_GetOrdersOrder, 0, len(ol))
	for _, o := range ol {
		items := make([]*orders.GetUserOrdersResponse_GetOrdersOrder_Item, 0, len(o.Items))
		for _, d := range o.Items {
			items = append(items, &orders.GetUserOrdersResponse_GetOrdersOrder_Item{
				TicketStock: d.TicketStock.Hex(),
				Session:     d.Session.Hex(),
				Quantity:    int64(d.Quantity),
				Price:       d.Price.String(),
				Name:        d.Name,
			})
		}
		res = append(res, &orders.GetUserOrdersResponse_GetOrdersOrder{
			Id:        o.Order.ID.Hex(),
			Items:     items,
			Total:     o.Order.Total.String(),
			Event:     o.Order.Event.Hex(),
			Status:    o.Order.Status,
			CreatedAt: helper.ConvertTimeToProtoTimestamp(o.Order.CreatedAt),
			Serial:    strconv.FormatUint(o.Order.Serial, 10),
		})
	}

	return ResponseSuccess(&orders.GetUserOrdersResponse{
		Data:  res,
		After: string(nextToken), // Use the token from the logic layer
	})
}

func (s *OrderService) AddOrder(c context.Context, in *orders.AddOrderRequest) (*api.Response, error) {
	uid, err := getUserId(c)
	if err != nil {
		// No logger here as unauthenticated requests are common and noisy.
		return ResponseError(codes.Unauthenticated, err)
	}

	eid, err := primitive.ObjectIDFromHex(in.GetEvent())
	if err != nil {
		s.logger.Warn("AddOrder: Invalid event_id format", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.InvalidArgument, err)
	}

	mid, err := primitive.ObjectIDFromHex(in.GetMerchantId())
	if err != nil {
		return ResponseError(codes.InvalidArgument, err)
	}

	pid, err := primitive.ObjectIDFromHex(in.GetPaymentMethod().GetId())
	if err != nil {
		return ResponseError(codes.InvalidArgument, err)
	}

	//subtotal and total check in logic
	var details = make([]*dto.OrderItem, 0, len(in.GetItems()))
	var flag = make(map[primitive.ObjectID]struct{})
	for _, item := range in.GetItems() {
		tid, err := primitive.ObjectIDFromHex(item.GetTicket())
		if err != nil {
			s.logger.Warn("AddOrder: Invalid ticket_id format in items", zap.Error(err), zap.Any("item", item))
			return ResponseError(codes.InvalidArgument, err)
		}
		_, ok := flag[tid]
		if ok {
			err := errors.New("duplicate ticket")
			s.logger.Warn("AddOrder: Duplicate ticket id in request", zap.Error(err), zap.String("ticket_id", item.GetTicket()))
			return ResponseError(codes.FailedPrecondition, err)
		}
		flag[tid] = struct{}{}
		oi := dto.NewOrderItem(tid, item.GetQuantity())
		details = append(details, oi)
	}

	paymentMethod := dto.NewPaymentMethodInfoRequest(pid, in.GetPaymentMethod().GetName())
	d := dto.NewAddOrderRequest(eid, mid, in.GetSubtotal(), in.GetTotal(), details, &models.User{
		UserId: uid,
		Name:   getUserName(c),
		Email:  getUserEmail(c),
		Avatar: getUserAvatar(c),
	}, paymentMethod)

	result, err := s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return s.logic.AddOrder(sessCtx, d)
	})
	if err != nil {
		s.logger.Error("AddOrder: logic.AddOrder failed", zap.Error(err), zap.Any("requestDto", d))
		return ResponseError(codes.Internal, err)
	}
	addOrderResponse := result.(*dto.AddOrderResponse)

	//TODO:新增參與者

	return ResponseSuccess(&orders.AddOrderResponse{
		OrderId: addOrderResponse.OrderID.Hex(),
		BillId:  addOrderResponse.BillID.Hex(),
	})
}

func (s *OrderService) CancelOrder(ctx context.Context, in *orders.CancelOrderRequest) (*api.Response, error) {
	oid, err := primitive.ObjectIDFromHex(in.GetOrder())
	if err != nil {
		s.logger.Warn("CancelOrder: Invalid order_id format", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, err)
	}

	// Authorize and get user ID
	uid, authErr := s.authorizeOrderConsumer(ctx, oid)
	if authErr != nil {
		return ResponseError(codes.PermissionDenied, authErr)
	}

	// Construct the user model for the DTO
	user := &models.User{
		UserId: uid,
		Name:   getUserName(ctx),
		Email:  getUserEmail(ctx),
		Avatar: getUserAvatar(ctx),
	}

	// Create the DTO for the logic layer
	req := dto.NewCancelOrderRequest(oid, user, in.GetReason())

	_, err = s.tm.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.logic.CancelOrder(sessCtx, req)
	})

	if err != nil {
		// TODO: handle specific errors from logic layer, e.g., order not found, permission denied
		s.logger.Error("CancelOrder: logic.CancelOrder failed", zap.Error(err), zap.Stringer("user_id", uid), zap.Stringer("order_id", oid))
		return ResponseError(codes.Internal, err)
	}

	return ResponseSuccess(nil)
}

func (s *OrderService) RequestOrderItemRefund(ctx context.Context, in *orders.RequestOrderItemRefundRequest) (*api.Response, error) {
	uid, err := getUserId(ctx)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}

	oid, err := primitive.ObjectIDFromHex(in.GetOrder())
	if err != nil {
		s.logger.Warn("RequestOrderItemRefund: Invalid order_id format", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, err)
	}

	iid, err := primitive.ObjectIDFromHex(in.GetItem())
	if err != nil {
		s.logger.Warn("RequestOrderItemRefund: Invalid item_id format", zap.Error(err), zap.String("item_id", in.GetItem()))
		return ResponseError(codes.InvalidArgument, err)
	}

	// Authorize and get user ID
	uid, authErr := s.authorizeOrderConsumer(ctx, oid)
	if authErr != nil {
		if errors.Is(authErr, ErrPermissionDenied) {
			return ResponseError(codes.PermissionDenied, authErr)
		}
	}

	user := &models.User{
		UserId: uid,
		Name:   getUserName(ctx),
		Email:  getUserEmail(ctx),
		Avatar: getUserAvatar(ctx),
	}

	req := dto.NewRequestOrderItemRefundRequest(oid, iid, user, in.GetReason())

	_, err = s.tm.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.logic.RequestOrderItemRefund(sessCtx, req)
	})

	if err != nil {
		s.logger.Error("RequestOrderItemRefund: logic.RequestOrderItemRefund failed", zap.Error(err), zap.Any("req", req))
		return ResponseError(codes.Internal, err)
	}

	return ResponseSuccess(nil)
}
