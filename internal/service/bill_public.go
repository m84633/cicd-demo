package service

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"partivo_tickets/api"
	"partivo_tickets/api/bills"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/logic"
)

type BillService struct {
	bills.UnimplementedBillServiceServer
	billLogic *logic.BillLogic
	logger    *zap.Logger
}

func NewBillService(billLogic *logic.BillLogic, logger *zap.Logger) *BillService {
	return &BillService{
		billLogic: billLogic,
		logger:    logger.Named("BillService"),
	}
}

func (s *BillService) GetTransactionHistory(ctx context.Context, req *bills.GetTransactionHistoryRequest) (*api.Response, error) {
	// 1. Get user from context (authentication)
	uid, err := getUserId(ctx)
	if err != nil {
		return ResponseError(codes.Unauthenticated, err)
	}

	// 2. Validate and parse inputs
	orderID, err := primitive.ObjectIDFromHex(req.GetOrderId())
	if err != nil {
		s.logger.Warn("GetTransactionHistory: Invalid order_id format", zap.Error(err), zap.String("order_id", req.GetOrderId()))
		return ResponseError(codes.InvalidArgument, fmt.Errorf("invalid order_id: %w", err))
	}

	// 3. Authorization check is SKIPPED (to be handled by Ory Keto middleware/interceptor later)

	// 4. Call logic layer
	billModels, err := s.billLogic.GetBillsForConsumerByOrder(ctx, orderID, uid)
	if err != nil {
		s.logger.Error("GetTransactionHistory: logic.GetBillsByOrder failed", zap.Error(err), zap.String("order_id", req.GetOrderId()))
		return ResponseError(codes.Internal, err)
	}

	// 5. Map models to protobuf response
	transactions := make([]*bills.Transaction, 0, len(billModels))
	for _, b := range billModels {
		var pm *bills.PaymentMethod
		if b.PaymentMethod != nil {
			pm = &bills.PaymentMethod{
				Id:   b.PaymentMethod.ID.Hex(),
				Name: b.PaymentMethod.Name,
			}
		}
		transactions = append(transactions, &bills.Transaction{
			Id:            b.ID.Hex(),
			Amount:        b.Amount.String(),
			Type:          b.Type,
			Status:        b.Status,
			Timestamp:     helper.ConvertTimeToProtoTimestamp(b.CreatedAt),
			PaymentMethod: pm,
		})
	}

	return ResponseSuccess(&bills.GetTransactionHistoryResponse{Transactions: transactions})
}
