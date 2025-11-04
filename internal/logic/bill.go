package logic

import (
	"context"
	"fmt"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/dao/repository"
	"partivo_tickets/pkg/relation"

	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/helper"
	"partivo_tickets/internal/models"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

type BillLogic struct {
	billRepo              repository.BillRepository
	orderRepo             repository.OrdersRepository
	auditLogRepo          repository.AuditLogRepository
	paymentEventPublisher *PaymentEventPublisher
	logger                *zap.Logger
	relationClient        *relation.Client
}

func NewBillLogic(billRepo repository.BillRepository, orderRepo repository.OrdersRepository, auditLogRepo repository.AuditLogRepository, paymentEventPublisher *PaymentEventPublisher, logger *zap.Logger, relationClient *relation.Client) *BillLogic {
	return &BillLogic{
		billRepo:              billRepo,
		orderRepo:             orderRepo,
		auditLogRepo:          auditLogRepo,
		paymentEventPublisher: paymentEventPublisher,
		logger:                logger.Named("BillLogic"),
		relationClient:        relationClient,
	}
}

func (l *BillLogic) CreateBill(ctx context.Context, d *dto.CreateBillRequest) (primitive.ObjectID, error) {
	// 1. Get the parent order
	order, err := l.orderRepo.GetOrderByID(ctx, d.GetOrderID())
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to get order for bill creation: %w", err)
	}

	// 2. VALIDATION LOGIC
	// 2a. Check if the order status allows new payments (Whitelist approach).
	orderStatus := constants.ParseOrderStatus(order.Status)
	if orderStatus != constants.OrderStatusUnpaid && orderStatus != constants.OrderStatusPartiallyPaid {
		return primitive.NilObjectID, fmt.Errorf("order status is '%s', cannot create new bill", order.Status)
	}

	// 2b. Check for overpayment by calculating the sum of all non-failed bills.
	bills, err := l.billRepo.GetBillsByOrderID(ctx, d.GetOrderID())
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to get existing bills for validation: %w", err)
	}

	committedAmount, _ := primitive.ParseDecimal128("0")
	for _, bill := range bills {
		billStatus := constants.ParseBillStatus(bill.Status)
		if billStatus == constants.BillStatusPaid || billStatus == constants.BillStatusPending {
			committedAmount, err = helper.AddDecimal128(committedAmount, bill.Amount)
			if err != nil {
				return primitive.NilObjectID, fmt.Errorf("failed to sum committed amount: %w", err)
			}
		}
	}

	amountOwed, err := helper.SubDecimal128(order.Total, committedAmount)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to calculate amount owed: %w", err)
	}

	cmp, err := helper.CompareDecimal128(d.GetAmount(), amountOwed)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to compare amounts: %w", err)
	}
	if cmp > 0 {
		return primitive.NilObjectID, fmt.Errorf("payment amount (%s) exceeds amount owed (%s)", d.GetAmount().String(), amountOwed.String())
	}

	// 3. Construct the new bill model
	bill := &models.Bill{
		ID:       primitive.NewObjectID(),
		Order:    d.GetOrderID(),
		Customer: order.User,
		Amount:   d.GetAmount(),
		Type:     constants.BillTypePayment,
		Status:   constants.BillStatusPending.String(),
		PaymentMethod: &models.PaymentMethodInfo{
			ID:   d.GetPaymentMethod().ID(),
			Name: d.GetPaymentMethod().Name(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Payment:   nil,
	}

	// 4. Call the repository to save the bill
	billID, err := l.billRepo.CreateBill(ctx, bill)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to create bill in repository: %w", err)
	}

	// 5. Create and save audit log
	auditLog := l.buildCreateBillAuditLog(d.GetOperator(), bill)
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		// Log the error but don't fail the whole transaction, as audit logging is secondary.
		l.logger.Error("CreateBill: Failed to create audit log", zap.Error(err))
	}

	// 7. relation
	err = l.relationClient.AddUserResourceRole(ctx, order.MerchantID.Hex(), constants.ResourceBill, bill.ID.Hex(), relation.RoleOwner)
	if err != nil {
		l.logger.Error("addOrder: Failed to add merchant resource role", zap.Error(err))
	}

	// 6. Publish payment creation event via OutboxPublisher
	if err := l.paymentEventPublisher.PublishPaymentEvent(ctx, constants.PaymentActionCreate, bill, order.MerchantID.Hex()); err != nil {
		l.logger.Error("CreateBill: Failed to publish payment event", zap.Error(err), zap.Stringer("billID", bill.ID))
		return primitive.NilObjectID, err // Return error to rollback transaction
	}

	return billID, nil
}

func (l *BillLogic) buildCreateBillAuditLog(operator *models.User, bill *models.Bill) *models.AuditLog {
	return NewAuditLog(operator, "CREATE_BILL", "bill", bill.ID, nil, bill)
}

func (l *BillLogic) CancelBill(ctx context.Context, d *dto.CancelBillRequest) error {
	// 1. Get the bill
	bill, err := l.billRepo.GetBillByID(ctx, d.GetBillID())
	if err != nil {
		return fmt.Errorf("failed to get bill: %w", err)
	}

	// 2. Validate status: Only pending bills can be cancelled.
	if bill.Status != constants.BillStatusPending.String() {
		return fmt.Errorf("bill with status '%s' cannot be cancelled", bill.Status)
	}

	// 3. Get the parent Order to retrieve the merchantID
	order, err := l.orderRepo.GetOrderByID(ctx, bill.Order)
	if err != nil {
		return fmt.Errorf("failed to get order '%s' for bill cancellation: %w", bill.Order.Hex(), err)
	}

	// 4. Update bill status to Canceled
	// This assumes a generic UpdateStatus method exists on the repository.
	//if err := l.billRepo.UpdateBill(ctx, bill.ID, constants.BillStatusCanceled.String()); err != nil {
	//	return fmt.Errorf("failed to update bill status: %w", err)
	//}

	if err := l.billRepo.UpdateBill(ctx, bill.ID, repository.WithStatus(constants.BillStatusCanceled.String())); err != nil {
		return fmt.Errorf("failed to update bill status: %w", err)
	}

	// 5. Publish payment cancellation event
	if err := l.paymentEventPublisher.PublishPaymentEvent(ctx, constants.PaymentActionCancel, bill, order.MerchantID.Hex()); err != nil {
		l.logger.Error("CancelBill: Failed to publish payment cancellation event", zap.Error(err), zap.Stringer("billID", bill.ID))
		return err // Return error to rollback transaction
	}

	// 6. Create audit log
	auditLog := l.buildCancelBillAuditLog(d.GetOperator(), bill, d.GetReason())
	if err := l.auditLogRepo.Create(ctx, auditLog); err != nil {
		l.logger.Error("CancelBill: Failed to create audit log", zap.Error(err))
		// Do not fail transaction for audit log failure.
	}

	return nil
}

func (l *BillLogic) buildCancelBillAuditLog(operator *models.User, bill *models.Bill, reason string) *models.AuditLog {
	details := map[string]interface{}{
		"reason": reason,
	}
	return NewAuditLog(operator, "CANCEL_BILL", "bill", bill.ID, details, nil)
}

// ProcessPaymentUpdate handles the result of a payment attempt for a specific bill.
// It updates the bill's status and the parent order's paid/returned amount.
// It returns the processed Bill model for the caller to handle subsequent actions.
// This method expects to be called within a transaction.
//func (l *BillLogic) ProcessPaymentUpdate(ctx context.Context, billID primitive.ObjectID, isSuccessful bool, paidAt time.Time) (*models.Bill, error) {
//	// 1. Get the bill from the database.
//	bill, err := l.billRepo.GetBillByID(ctx, billID)
//	if err != nil {
//		return nil, fmt.Errorf("failed to get bill %s: %w", billID.Hex(), err)
//	}
//
//	// 2. State Machine and Idempotency Check
//	if bill.Status != constants.BillStatusPending.String() {
//		l.logger.Warn("Received payment update for a bill not in pending state.",
//			zap.Stringer("billID", billID),
//			zap.String("current_status", bill.Status),
//		)
//		return bill, nil // Return the bill so caller can still decide to proceed or not based on status.
//	}
//
//	// 3. Handle based on payment success.
//	if isSuccessful {
//		var newStatus string
//		switch bill.Type {
//		case constants.BillTypePayment:
//			newStatus = constants.BillStatusPaid.String()
//			if err := l.orderRepo.UpdateOrder(ctx, bill.Order, repository.WithIncPaid(bill.Amount)); err != nil {
//				return nil, fmt.Errorf("failed to increment order paid amount: %w", err)
//			}
//		case constants.BillTypeRefund:
//			newStatus = constants.BillStatusRefunded.String()
//			updates := repository.Compose(
//				repository.WithIncReturned(bill.Amount),
//				repository.WithIncReturnedItemsCount(1), // Assuming one refund bill corresponds to one item
//			)
//			if err := l.orderRepo.UpdateOrder(ctx, bill.Order, updates); err != nil {
//				return nil, fmt.Errorf("failed to update order for refund: %w", err)
//			}
//		default:
//			return nil, fmt.Errorf("unknown bill type: %s", bill.Type)
//		}
//
//		// Update bill status
//		if err := l.billRepo.UpdateBill(ctx, billID, repository.WithStatus(newStatus)); err != nil {
//			return nil, fmt.Errorf("failed to update bill status: %w", err)
//		}
//	} else {
//		// Update bill status to Failed.
//		if err := l.billRepo.UpdateBill(ctx, billID, repository.WithStatus(constants.BillStatusFailed.String())); err != nil {
//			return nil, fmt.Errorf("failed to update bill status to failed: %w", err)
//		}
//	}
//
//	// Refetch the bill to return the most up-to-date state
//	return l.billRepo.GetBillByID(ctx, billID)
//}

// ProcessPaymentUpdate handles the result of a payment attempt for a specific bill.
// It updates the bill's status and the parent order's paid/returned amount.
// It returns the processed Bill model for the caller to handle subsequent actions.
// This method expects to be called within a transaction.
func (l *BillLogic) ProcessPaymentUpdate(ctx context.Context, billID primitive.ObjectID, paymentID string, isSuccessful bool, paidAt time.Time) (*models.Bill, error) {
	// 1. Get the bill from the database.
	bill, err := l.billRepo.GetBillByID(ctx, billID)
	if err != nil {
		return nil, fmt.Errorf("failed to get bill %s: %w", billID.Hex(), err)
	}

	// 2. State Machine and Idempotency Check
	if bill.Status != constants.BillStatusPending.String() {
		l.logger.Warn("Received payment update for a bill not in pending state, skipping.",
			zap.Stringer("billID", billID),
			zap.String("current_status", bill.Status),
		)
		// Return a specific error to indicate the message should not be requeued,
		// as this is an expected condition for an idempotent consumer.
		return bill, ErrBillAlreadyProcessed
	}

	// 3. Prepare the database update options, starting with the paymentID.
	updateOptions := make([]repository.UpdateOption, 0, 2)
	var paymentObjID primitive.ObjectID
	if paymentID != "" {
		paymentObjID, err = primitive.ObjectIDFromHex(paymentID)
		if err != nil {
			// Log the error but proceed, as the status update is more critical.
			l.logger.Error("Invalid paymentID format, cannot store it", zap.Error(err), zap.String("paymentID", paymentID))
		} else {
			updateOptions = append(updateOptions, repository.WithPaymentID(&paymentObjID))
		}
	}

	now := time.Now()

	// 4. Determine the new status and apply logic based on success or failure.
	if isSuccessful {
		var newStatus string
		switch bill.Type {
		case constants.BillTypePayment:
			newStatus = constants.BillStatusPaid.String()
			if err := l.orderRepo.UpdateOrder(ctx, bill.Order, repository.WithIncPaid(bill.Amount), repository.WithUpdatedBy(models.SystemUser), repository.WithUpdatedAt(now)); err != nil {
				return nil, fmt.Errorf("failed to increment order paid amount: %w", err)
			}
		case constants.BillTypeRefund:
			newStatus = constants.BillStatusRefunded.String()
			if err := l.orderRepo.UpdateOrder(ctx, bill.Order, repository.WithIncReturned(bill.Amount), repository.WithIncReturnedItemsCount(1), repository.WithUpdatedBy(models.SystemUser), repository.WithUpdatedAt(now)); err != nil {
				return nil, fmt.Errorf("failed to update order for refund: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown bill type: %s", bill.Type)
		}
		updateOptions = append(updateOptions, repository.WithStatus(newStatus))
	} else {
		updateOptions = append(updateOptions, repository.WithStatus(constants.BillStatusFailed.String()))
	}

	// 5. Execute the update with all collected options.
	if err := l.billRepo.UpdateBill(ctx, billID, updateOptions...); err != nil {
		return nil, fmt.Errorf("failed to update bill: %w", err)
	}

	// 6. Refetch the bill to return the most up-to-date state to the caller.
	return l.billRepo.GetBillByID(ctx, billID)
}

func (l *BillLogic) GetBillsByOrder(ctx context.Context, orderID primitive.ObjectID) ([]*models.Bill, error) {
	return l.billRepo.GetBillsByOrderID(ctx, orderID)
}

// GetBillsForConsumerByOrder gets bills for a specific order, but only if the consumerID matches the order's owner.
func (l *BillLogic) GetBillsForConsumerByOrder(ctx context.Context, orderID primitive.ObjectID, consumerID primitive.ObjectID) ([]*models.Bill, error) {
	// 1. First, get the order for authorization.
	order, err := l.orderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		// If the order doesn't exist, wrap the error.
		return nil, fmt.Errorf("failed to get order for authorization: %w", err)
	}

	// 2. Core authorization check: ensure the requester owns the order.
	if order.User.UserId != consumerID {
		// Return a clear permission error.
		return nil, fmt.Errorf("permission denied: user %s does not own order %s", consumerID.Hex(), orderID.Hex())
	}

	// 3. If authorization passes, call the underlying repository method to get the bills.
	return l.billRepo.GetBillsByOrderID(ctx, orderID)
}
