package service

import (
	"context"
	"errors"
	"fmt"
	"partivo_tickets/api"
	"partivo_tickets/api/orders"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/db"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/logic"
	"partivo_tickets/internal/models"
	"partivo_tickets/pkg/pagination"
	"strconv"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type OrdersAdminService struct {
	orders.UnimplementedOrdersAdminServiceServer
	logic  logic.OrderLogic
	logger *zap.Logger
	tm     db.TransactionManager
}

func NewOrdersAdminService(l logic.OrderLogic, logger *zap.Logger, tm db.TransactionManager) *OrdersAdminService {
	return &OrdersAdminService{
		logic:  l,
		logger: logger.Named("OrdersAdminService"), // Add a name to the logger for context
		tm:     tm,
	}
}

func (s *OrdersAdminService) IsEventHasOpenOrders(c context.Context, in *orders.IsEventHasOrdersRequest) (*api.Response, error) {
	//TODO:Event Editor
	eid, err := primitive.ObjectIDFromHex(in.GetEvent())
	if err != nil {
		s.logger.Warn("IsEventHasOpenOrders: Invalid event_id format", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.InvalidArgument, err)
	}

	b, err := s.logic.IsEventHasOpenOrders(c, eid)
	if err != nil {
		s.logger.Error("IsEventHasOpenOrders: logic.IsEventHasOpenOrders failed", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.Internal, err)
	}

	return ResponseSuccess(wrapperspb.Bool(b))
}

func (s *OrdersAdminService) GetOrdersByEvent(c context.Context, in *orders.GetOrdersByEventRequest) (*api.Response, error) {
	//TODO:Event Viewer
	const pageSize = 10 // pageSize is defined in the backend.
	eid, err := primitive.ObjectIDFromHex(in.GetEvent())
	if err != nil {
		s.logger.Warn("GetOrdersByEvent: Invalid event_id format", zap.Error(err), zap.String("event_id", in.GetEvent()))
		return ResponseError(codes.InvalidArgument, err)
	}

	// 1. Create PageRequest.
	pageReq := pagination.NewPageRequest(int(in.GetPage()), pageSize) // Assuming in.GetPage() exists

	// 2. Call logic layer.
	pageResult, err := s.logic.GetOrdersByEvent(c, eid, pageReq)
	if err != nil {
		s.logger.Error("GetOrdersByEvent: logic.GetOrdersByEvent failed", zap.Error(err), zap.Any("request", in))
		return ResponseError(codes.Internal, err)
	}

	// 3. Convert data and create paginated response.
	ordersData, ok := pageResult.Data.([]*dto.OrderWithItems)
	if !ok {
		err := errors.New("internal error: unexpected data type from logic layer")
		s.logger.Error("GetOrdersByEvent: Failed to assert pageResult.Data type", zap.Error(err))
		return ResponseError(codes.Internal, err)
	}
	resorders := convertOrdersToResponseForm(ordersData)

	return ResponseSuccess(&orders.GetOrdersResponse{
		Data: resorders,
		PageInfo: &api.PageInfo{
			Total:      pageResult.Total,
			Page:       int32(pageResult.Page),
			PageSize:   int32(pageResult.PageSize),
			TotalPages: int32(pageResult.TotalPages),
		},
	})
}

func (s *OrdersAdminService) GetOrdersBySession(c context.Context, in *orders.GetOrdersBySessionRequest) (*api.Response, error) {
	//TODO:Event Viewer or Session Viewer
	const pageSize = 10 // As per your request, pageSize is defined in the backend.
	sid, err := primitive.ObjectIDFromHex(in.GetSession())
	if err != nil {
		s.logger.Warn("GetOrdersBySession: Invalid session_id format", zap.Error(err), zap.String("session_id", in.GetSession()))
		return ResponseError(codes.InvalidArgument, err)
	}

	// 1. Create PageRequest. We get `page` from the request and use the fixed `pageSize`.
	pageReq := pagination.NewPageRequest(int(in.GetPage()), pageSize)

	// 2. Call logic layer with the pagination request.
	pageResult, err := s.logic.GetOrdersBySession(c, sid, pageReq)
	if err != nil {
		s.logger.Error("GetOrdersBySession: logic.GetOrdersBySession failed", zap.Error(err), zap.Any("request", in))
		return ResponseError(codes.Internal, err)
	}

	// 3. Convert the data and create the paginated gRPC response.
	ordersData, ok := pageResult.Data.([]*dto.OrderWithItems)
	if !ok {
		// This would be an internal error, as the logic layer should always return the correct type.
		err := errors.New("internal error: unexpected data type from logic layer")
		s.logger.Error("GetOrdersBySession: Failed to assert pageResult.Data type", zap.Error(err))
		return ResponseError(codes.Internal, err)
	}
	resorders := convertOrdersToResponseForm(ordersData)

	return ResponseSuccess(&orders.GetOrdersResponse{
		Data: resorders,
		PageInfo: &api.PageInfo{
			Total:      pageResult.Total,
			Page:       int32(pageResult.Page),
			PageSize:   int32(pageResult.PageSize),
			TotalPages: int32(pageResult.TotalPages),
		},
	})
}

//func (s *OrdersAdminService) ChangeOrderStatus(c context.Context, in *orders.ChangeOrderStatusRequest) (*api.Response, error) {
//	//TODO:要有更新訂單的權限
//	uid, err := getUserId(c)
//	if err != nil {
//		return ResponseError(codes.Unauthenticated, err)
//	}
//
//	oid, err := primitive.ObjectIDFromHex(in.GetOrder())
//	if err != nil {
//		return ResponseError(codes.InvalidArgument, err)
//	}
//
//	st, err := convertProtoStatusToAppStatus(in.GetStatus())
//	if err != nil {
//		return ResponseError(codes.InvalidArgument, err)
//	}
//	d := dto.NewChangeOrderStatusRequest(oid, st, &models.User{
//		UserId: uid,
//		name:   getUserName(c),
//	})
//
//	if s.cfg.Mode == "dev" || s.cfg.Mode == "test" {
//		err = s.logic.ChangeOrderStatusWithoutTransaction(c, d)
//	} else {
//		err = s.logic.ChangeOrderStatus(c, d)
//	}
//
//	if err != nil {
//		return ResponseError(codes.Internal, err)
//	}
//
//	return ResponseSuccess(nil)
//}

func (s *OrdersAdminService) ChangeOrderItemStatus(c context.Context, in *orders.ChangeOrderItemStatusRequest) (*api.Response, error) {
	//TODO:Order Editor

	// 1. Get operator info from context (the user performing the action)
	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}
	operator := &models.User{
		UserId: uid,
		Name:   getUserName(c),
	}

	// 2. Validate and parse inputs from request
	orderID, err := primitive.ObjectIDFromHex(in.GetOrder()) // Assuming GetOrderId from proto
	if err != nil {
		s.logger.Warn("ChangeOrderItemStatus: Invalid order_id format", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_id: %w", err))
	}
	orderItemID, err := primitive.ObjectIDFromHex(in.GetItem())
	if err != nil {
		s.logger.Warn("ChangeOrderItemStatus: Invalid order_item_id format", zap.Error(err), zap.String("order_item_id", in.GetItem()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_item_id: %w", err))
	}

	newStatus, err := convertProtoOrderItemStatusToAppStatus(in.GetStatus())
	if err != nil {
		s.logger.Warn("ChangeOrderItemStatus: Invalid status format", zap.Error(err), zap.String("status", in.GetStatus().String()))
		return ResponseError(codes.InvalidArgument, err)
	}

	// TODO: Authorization check here!
	// We should verify that the order (orderID) belongs to the user (uid).
	// This is a critical security step.

	// 3. Create DTO for logic layer
	// NOTE: We will need to create this DTO struct and constructor.
	d := dto.NewUpdateOrderItemStatusRequest(orderItemID, orderID, newStatus, in.GetReason(), operator)

	// 4. Call logic layer within a transaction
	_, err = s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.logic.UpdateOrderItemStatus(sessCtx, d)
	})
	if err != nil {
		s.logger.Error("ChangeOrderItemStatus: s.logic.UpdateOrderItemStatus failed", zap.Error(err), zap.Any("requestDto", d))
		return ResponseError(codes.Internal, err)
	}

	// 5. Return success
	return ResponseSuccess(nil)
}

func (s *OrdersAdminService) GetOrderByID(ctx context.Context, in *orders.GetOrderByIDRequest) (*api.Response, error) {
	//TODO:Order Viewer
	// 1. Get user id from context for authorization
	uid, err := getUserId(ctx)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}

	// 2. Parse order id from request
	orderID, err := primitive.ObjectIDFromHex(in.GetOrder())
	if err != nil {
		s.logger.Warn("GetOrderByID: Invalid order_id format", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_id: %w", err))
	}

	// 3. Call logic layer to get all details
	orderDetails, err := s.logic.GetOrderDetails(ctx, orderID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			s.logger.Warn("GetOrderByID: Order not found", zap.Error(err), zap.String("order_id", in.GetOrder()))
			return ResponseError(codes.NotFound, fmt.Errorf("order not found"))
		}
		s.logger.Error("GetOrderByID: logic.GetOrderDetails failed", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.Internal, err)
	}

	// 4. Authorization Check: Ensure the user is requesting their own order.
	if orderDetails.Order.User.UserId != uid {
		// Return NotFound instead of Forbidden to avoid leaking information about order existence.
		s.logger.Warn("GetOrderByID: User attempted to access unauthorized order",
			zap.String("requester_id", uid.Hex()),
			zap.String("order_id", orderID.Hex()),
			zap.String("owner_id", orderDetails.Order.User.UserId.Hex()))
		return ResponseError(codes.NotFound, fmt.Errorf("order not found"))
	}

	// 5. Convert DTO to protobuf response
	res := convertOrderDetailsToProto(orderDetails)

	return ResponseSuccess(res)
}

func (s *OrdersAdminService) CancelOrder(c context.Context, in *orders.CancelOrderRequest) (*api.Response, error) {
	//TODO:Order Owner
	// 1. Get operator info from context (the user performing the action)
	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}
	operator := &models.User{
		UserId: uid,
		Name:   getUserName(c),
	}

	// 2. Validate and parse inputs from request
	orderID, err := primitive.ObjectIDFromHex(in.GetOrder())
	if err != nil {
		s.logger.Warn("CancelOrder: Invalid order_id format", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_id: %w", err))
	}

	// TODO: Authorization check: Ensure the user has permission to cancel this order.
	// This might involve checking ownership or roles (e.g., admin, event manager).

	// 3. Create DTO for logic layer
	d := dto.NewCancelOrderRequest(orderID, operator, in.GetReason())

	// 4. Call logic layer within a transaction
	_, err = s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.logic.CancelOrder(sessCtx, d)
	})
	if err != nil {
		s.logger.Error("CancelOrder: s.logic.CancelOrder failed", zap.Error(err), zap.Any("request", in))
		return ResponseError(codes.Internal, err)
	}

	// 5. Return success
	return ResponseSuccess(nil)
}

func (s *OrdersAdminService) RejectOrderItemRefund(ctx context.Context, in *orders.RejectOrderItemRefundRequest) (*api.Response, error) {
	//TODO:Order Editor
	// 1. Get operator info from context
	uid, err := getUserId(ctx)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}
	operator := &models.User{
		UserId: uid,
		Name:   getUserName(ctx),
	}

	// 2. Validate and parse inputs
	orderID, err := primitive.ObjectIDFromHex(in.GetOrder())
	if err != nil {
		s.logger.Warn("RejectOrderItemRefund: Invalid order_id format", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_id: %w", err))
	}
	orderItemID, err := primitive.ObjectIDFromHex(in.GetItem())
	if err != nil {
		s.logger.Warn("RejectOrderItemRefund: Invalid item_id format", zap.Error(err), zap.String("item_id", in.GetItem()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid item_id: %w", err))
	}

	// 3. Create DTO for logic layer
	d := dto.NewRejectOrderItemRefundRequest(orderID, orderItemID, operator, in.GetReason())

	// 4. Call logic layer within a transaction
	_, err = s.tm.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.logic.RejectOrderItemRefund(sessCtx, d)
	})
	if err != nil {
		s.logger.Error("RejectOrderItemRefund: s.logic.RejectOrderItemRefund failed", zap.Error(err), zap.Any("requestDto", d))
		return ResponseError(codes.Internal, err)
	}

	// 5. Return success
	return ResponseSuccess(nil)
}

func convertOrdersToResponseForm(ol []*dto.OrderWithItems) []*orders.GetOrdersResponse_GetOrdersOrder {
	resorders := make([]*orders.GetOrdersResponse_GetOrdersOrder, 0, len(ol))
	for _, o := range ol {
		resitems := make([]*orders.GetOrdersResponse_GetOrdersOrder_Item, 0, len(o.Items))
		for _, d := range o.Items {
			var returnRequest *orders.GetOrdersResponse_GetOrdersOrder_ReturnRequest
			if d.ReturnInfo != nil {
				returnRequest = &orders.GetOrdersResponse_GetOrdersOrder_ReturnRequest{
					Reason:      d.ReturnInfo.Reason,
					RequestedAt: helper.ConvertTimeToProtoTimestamp(d.ReturnInfo.RequestedAt),
				}
			}
			resitems = append(resitems, &orders.GetOrdersResponse_GetOrdersOrder_Item{
				TicketStock:   d.TicketStock.Hex(),
				Session:       d.Session.Hex(),
				Quantity:      int64(d.Quantity),
				Price:         d.Price.String(),
				Name:          d.Name,
				Status:        d.Status,
				ReturnRequest: returnRequest,
			})
		}
		resuser := &api.User{
			UserId: o.User.UserId.Hex(),
			Name:   o.User.Name,
			Email:  o.User.Email,
			Avatar: o.User.Avatar,
		}
		resorder := &orders.GetOrdersResponse_GetOrdersOrder{
			Id:        o.ID.Hex(),
			Items:     resitems,
			Subtotal:  o.Subtotal.String(),
			Total:     o.Total.String(),
			Event:     o.Event.Hex(),
			Status:    o.Status,
			Note:      o.Note,
			User:      resuser,
			CreatedAt: helper.ConvertTimeToProtoTimestamp(o.CreatedAt),
			UpdatedAt: helper.ConvertTimeToProtoTimestamp(o.UpdatedAt),
			Serial:    strconv.FormatUint(o.Serial, 10),
		}
		resorders = append(resorders, resorder)
	}
	return resorders
}

//func convertProtoStatusToAppStatus(ps orders.OrderStatus) (string, error) {
//	switch ps {
//	case orders.OrderStatus_PROCESSING:
//		return constants.OrderStatusPartiallyPaid.String(), nil
//	case orders.OrderStatus_COMPLETED:
//		return constants.OrderStatusPaid.String(), nil
//	case orders.OrderStatus_CANCELLED:
//		return constants.OrderStatusPartiallyReturned.String(), nil
//	case orders.OrderStatus_REFUNDED:
//		return constants.OrderStatusReturned.String(), nil
//	default:
//		return "", fmt.Errorf("invalid order status: %s", ps.String())
//	}
//}

func convertProtoOrderItemStatusToAppStatus(ps orders.ChangeOrderItemStatusRequest_OrderItemStatus) (string, error) {
	switch ps {
	case orders.ChangeOrderItemStatusRequest_CANCELED:
		return constants.OrderItemStatusCanceled.String(), nil
	case orders.ChangeOrderItemStatusRequest_RETURN_REQUESTED:
		return constants.OrderItemStatusReturnRequested.String(), nil
	case orders.ChangeOrderItemStatusRequest_RETURN_APPROVED:
		return constants.OrderItemStatusReturnApproved.String(), nil
	default:
		return "", fmt.Errorf("invalid order item status: %s", ps.String())
	}
}

// convertOrderDetailsToProto converts the OrderDetails DTO to the protobuf response message.
func convertOrderDetailsToProto(details *dto.OrderDetails) *orders.GetOrderResponse {
	// Convert OrderItems
	items := make([]*orders.GetOrderResponse_Item, len(details.Items))
	for i, item := range details.Items {
		var returnRequset *orders.GetOrderResponse_Item_ReturnRequest
		if item.ReturnInfo != nil {
			returnRequset = &orders.GetOrderResponse_Item_ReturnRequest{
				Reason:      item.ReturnInfo.Reason,
				RequestedAt: helper.ConvertTimeToProtoTimestamp(item.ReturnInfo.RequestedAt),
			}
		}
		items[i] = &orders.GetOrderResponse_Item{
			Id:            item.ID.Hex(),
			TicketStock:   item.TicketStock.Hex(),
			Session:       item.Session.Hex(),
			Quantity:      int64(item.Quantity),
			Price:         item.Price.String(),
			Name:          item.Name,
			Status:        item.Status,
			ReturnRequest: returnRequset,
		}
	}

	// Convert Bills
	bills := make([]*orders.GetOrderResponse_Bill, len(details.Bills))
	for i, bill := range details.Bills {
		var paymentMethod string
		if bill.PaymentMethod != nil {
			// This is a placeholder. You might need to fetch payment method details.
			paymentMethod = bill.PaymentMethod.Name
		}
		bills[i] = &orders.GetOrderResponse_Bill{
			Id:        bill.ID.Hex(),
			Amount:    bill.Amount.String(),
			Type:      bill.Type,
			Status:    bill.Status,
			Payment:   paymentMethod,
			CreatedAt: helper.ConvertTimeToProtoTimestamp(bill.CreatedAt),
		}
	}

	// Main Order part
	orderProto := &orders.GetOrderResponse_Order{
		Id:       details.Order.ID.Hex(),
		Subtotal: details.Order.Subtotal.String(),
		Total:    details.Order.Total.String(),
		Event:    details.Order.Event.Hex(),
		Status:   details.Order.Status,
		Note:     details.Order.Note,
		User: &api.User{
			UserId: details.Order.User.UserId.Hex(),
			Name:   details.Order.User.Name,
			Email:  details.Order.User.Email,
			Avatar: details.Order.User.Avatar,
		},
		CreatedAt: helper.ConvertTimeToProtoTimestamp(details.Order.CreatedAt),
		UpdatedAt: helper.ConvertTimeToProtoTimestamp(details.Order.UpdatedAt),
		Serial:    strconv.FormatUint(details.Order.Serial, 10),
	}

	return &orders.GetOrderResponse{
		Order: orderProto,
		Items: items,
		Bills: bills,
	}
}
