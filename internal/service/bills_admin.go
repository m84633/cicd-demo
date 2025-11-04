package service

import (
	"context"
	"fmt"
	"partivo_tickets/api"
	"partivo_tickets/api/bills"
	"partivo_tickets/internal/db"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/logic"
	"partivo_tickets/internal/models"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
)

type BillsAdminService struct {
	bills.UnimplementedBillAdminServiceServer
	billLogic  *logic.BillLogic
	orderLogic logic.OrderLogic // Add new dependency
	logger     *zap.Logger
	tm         db.TransactionManager
}

// NewBillsAdminService is the constructor
func NewBillsAdminService(billLogic *logic.BillLogic, orderLogic logic.OrderLogic, logger *zap.Logger, tm db.TransactionManager) *BillsAdminService {
	return &BillsAdminService{
		billLogic:  billLogic,
		orderLogic: orderLogic,
		logger:     logger.Named("BillsAdminService"),
		tm:         tm,
	}
}

func (s *BillsAdminService) CreateBill(c context.Context, in *bills.CreateBillRequest) (*api.Response, error) {
	//TODO:Order Editor
	// 1. Get operator info from context
	uid, err := getUserId(c)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}
	operator := &models.User{
		UserId: uid,
		Name:   getUserName(c),
	}

	// 2. Validate and parse inputs
	orderID, err := primitive.ObjectIDFromHex(in.GetOrder())
	if err != nil {
		s.logger.Warn("CreateBill: Invalid order_id format", zap.Error(err), zap.String("order_id", in.GetOrder()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_id: %w", err))
	}

	amount, err := primitive.ParseDecimal128(in.GetAmount())
	if err != nil {
		s.logger.Warn("CreateBill: Invalid amount format", zap.Error(err), zap.String("amount", in.GetAmount()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid amount: %w", err))
	}
	pr := in.GetPaymentMethod()
	pmid, err := primitive.ObjectIDFromHex(pr.GetId())
	if err != nil {
		s.logger.Warn("CreateBill: Invalid payment_method_id format", zap.Error(err), zap.String("payment_method_id", pr.GetId()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid amount: %w", err))
	}
	pm := dto.NewPaymentMethodInfoRequest(pmid, pr.GetName())

	// 3. Create DTO for logic layer
	d := dto.NewCreateBillRequest(orderID, amount, operator, pm)

	// 4. Call logic layer within a transaction
	result, err := s.tm.WithTransaction(c, func(sessCtx context.Context) (interface{}, error) {
		return s.billLogic.CreateBill(sessCtx, d)
	})
	if err != nil {
		s.logger.Error("CreateBill: logic.CreateBill failed", zap.Error(err), zap.Any("requestDto", d))
		return ResponseError(codes.Internal, err)
	}
	billID := result.(primitive.ObjectID)

	// 5. Return success with the new bill id
	return ResponseSuccess(&api.ID{Id: billID.Hex()})
}

func (s *BillsAdminService) CancelBill(ctx context.Context, req *bills.CancelBillRequest) (*api.Response, error) {
	//TODO:Bill Owner
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
	billID, err := primitive.ObjectIDFromHex(req.GetId())
	if err != nil {
		s.logger.Warn("CancelBill: Invalid bill_id format", zap.Error(err), zap.String("bill_id", req.GetId()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid bill_id: %w", err))
	}

	// 3. Create DTO for logic layer
	cancelDto := dto.NewCancelBillRequest(billID, operator, req.GetReason())

	// 4. Call logic layer within a transaction
	_, err = s.tm.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		return nil, s.billLogic.CancelBill(sessCtx, cancelDto)
	})
	if err != nil {
		s.logger.Error("CancelBill: logic.CancelBill failed", zap.Error(err), zap.Any("requestDto", cancelDto))
		// TODO: Map logic errors to specific gRPC status codes
		return ResponseError(codes.Internal, err)
	}

	// 5. Return success
	return ResponseSuccess(nil)
}

func (s *BillsAdminService) CreateRefundBill(ctx context.Context, req *bills.CreateRefundBillRequest) (*api.Response, error) {
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
	orderItemID, err := primitive.ObjectIDFromHex(req.GetOrderItemId())
	if err != nil {
		s.logger.Warn("CreateRefundBill: Invalid order_item_id format", zap.Error(err), zap.String("order_item_id", req.GetOrderItemId()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_item_id: %w", err))
	}

	amount, err := primitive.ParseDecimal128(req.GetAmount())
	if err != nil {
		s.logger.Warn("CreateRefundBill: Invalid amount format", zap.Error(err), zap.String("amount", req.GetAmount()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid amount: %w", err))
	}

	pmReq := req.GetPaymentMethod()
	pmid, err := primitive.ObjectIDFromHex(pmReq.GetId())
	if err != nil {
		s.logger.Warn("CreateRefundBill: Invalid payment_method_id format", zap.Error(err), zap.String("payment_method_id", pmReq.GetId()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid payment_method_id: %w", err))
	}
	paymentMethodInfo := dto.NewPaymentMethodInfoRequest(pmid, pmReq.GetName())

	// 3. Create DTO for logic layer
	refundDto := dto.NewCreateRefundBillRequest(orderItemID, amount, paymentMethodInfo, req.GetReason(), operator)

	// 4. Call logic layer within a transaction
	result, err := s.tm.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		return s.orderLogic.CreateRefundBill(sessCtx, refundDto)
	})
	if err != nil {
		s.logger.Error("CreateRefundBill: logic.CreateRefundBill failed", zap.Error(err), zap.Any("requestDto", refundDto))
		return ResponseError(codes.Internal, err)
	}
	billID := result.(primitive.ObjectID)

	// 5. Return success
	return ResponseSuccess(&api.ID{Id: billID.Hex()})
}

func (s *BillsAdminService) GetBillsByOrder(ctx context.Context, req *bills.GetBillsByOrderRequest) (*api.Response, error) {
	//TODO:Order Viewer
	// 1. Validate and parse inputs
	orderID, err := primitive.ObjectIDFromHex(req.GetOrderId())
	if err != nil {
		s.logger.Warn("GetBillsByOrder: Invalid order_id format", zap.Error(err), zap.String("order_id", req.GetOrderId()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_id: %w", err))
	}

	// 2. Call logic layer
	billModels, err := s.billLogic.GetBillsByOrder(ctx, orderID)
	if err != nil {
		s.logger.Error("GetBillsByOrder: logic.GetBillsByOrder failed", zap.Error(err), zap.String("order_id", req.GetOrderId()))
		return ResponseError(codes.Internal, err)
	}

	// 3. Map models to protobuf response
	protoBills := make([]*bills.Bill, 0, len(billModels))
	for _, b := range billModels {
		var pm *bills.PaymentMethod
		if b.PaymentMethod != nil {
			pm = &bills.PaymentMethod{
				Id:   b.PaymentMethod.ID.Hex(),
				Name: b.PaymentMethod.Name,
			}
		}
		var paymentID string
		if b.Payment != nil {
			paymentID = b.Payment.Hex()
		}
		protoBills = append(protoBills, &bills.Bill{
			Id:            b.ID.Hex(),
			Amount:        b.Amount.String(),
			Type:          b.Type,
			Status:        b.Status,
			CreatedAt:     helper.ConvertTimeToProtoTimestamp(b.CreatedAt),
			Note:          b.Note,
			PaymentMethod: pm,
			PaymentId:     paymentID,
		})
	}

	return ResponseSuccess(&bills.GetBillsByOrderResponse{Bills: protoBills})
}
