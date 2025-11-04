package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/constants"
	"partivo_tickets/internal/db"
	"partivo_tickets/internal/dto"
	"partivo_tickets/internal/logic"
	"partivo_tickets/internal/models"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// BillStatusUpdateHandler handles messages about bill status changes from the payment service.
type BillStatusUpdateHandler struct {
	billLogic   *logic.BillLogic
	orderLogic  logic.OrderLogic
	ticketLogic *logic.TicketLogic
	logger      *zap.Logger
	cfg         *conf.RabbitMQConfig
	tm          db.TransactionManager
}

// NewBillStatusUpdateHandler creates a new handler for bill status updates.
func NewBillStatusUpdateHandler(billLogic *logic.BillLogic, orderLogic logic.OrderLogic, ticketLogic *logic.TicketLogic, logger *zap.Logger, cfg *conf.RabbitMQConfig, tm db.TransactionManager) *BillStatusUpdateHandler {
	return &BillStatusUpdateHandler{
		billLogic:   billLogic,
		orderLogic:  orderLogic,
		ticketLogic: ticketLogic,
		logger:      logger.Named("BillStatusUpdateHandler"),
		cfg:         cfg,
		tm:          tm,
	}
}

// QueueName returns the name of the queue this handler subscribes to.
func (h *BillStatusUpdateHandler) QueueName() string {
	return h.cfg.UpdateBillStatusTopic
}

// Handle processes the incoming message.
func (h *BillStatusUpdateHandler) Handle(ctx context.Context, d amqp.Delivery) error {
	h.logger.Info("Received message for bill status update", zap.ByteString("body", d.Body))

	// 1. Parse the message payload.
	var payload struct {
		BillID       string    `json:"bill_id"`
		PaymentID    string    `json:"payment_id"`
		IsSuccessful bool      `json:"is_successful"`
		PaidAt       time.Time `json:"paid_at"`
	}
	if err := json.Unmarshal(d.Body, &payload); err != nil {
		h.logger.Error("Failed to unmarshal message body", zap.Error(err), zap.ByteString("body", d.Body))
		return nil // Poison pill, ACK and remove.
	}

	// 2. Validate the BillID.
	bid, err := primitive.ObjectIDFromHex(payload.BillID)
	if err != nil {
		h.logger.Error("Invalid BillID format in message", zap.Error(err), zap.String("bill_id", payload.BillID))
		return nil // Invalid format, ACK and remove.
	}

	// 3. The entire process is wrapped in a single transaction controlled by the handler.
	_, err = h.tm.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		// Step 3a: Process the payment update for the bill.
		bill, err := h.billLogic.ProcessPaymentUpdate(sessCtx, bid, payload.PaymentID, payload.IsSuccessful, payload.PaidAt)
		if err != nil {
			if errors.Is(err, logic.ErrBillAlreadyProcessed) {
				h.logger.Warn("Bill status update skipped as it was already processed.",
					zap.String("bill_id", payload.BillID),
				)
				return nil, nil
			}
			return nil, fmt.Errorf("failed during bill payment processing: %w", err)
		}

		// Step 3b: Handle different logic based on the outcome and bill type.
		if payload.IsSuccessful {
			if bill.Type == constants.BillTypeRefund {
				// Successful Refund Flow
				orderItem, err := h.orderLogic.FindRefundableItemForBill(sessCtx, bill)
				if err != nil {
					return nil, fmt.Errorf("failed to find corresponding order item for refund bill %s: %w", bill.ID.Hex(), err)
				}

				updateReq := dto.NewUpdateOrderItemStatusRequest(
					orderItem.ID,
					bill.Order,
					constants.OrderItemStatusReturned.String(),
					"Refund processed successfully",
					models.SystemUser,
				)
				if err != nil {
					return nil, fmt.Errorf("failed to create update order item status request: %w", err)
				}

				// This call will internally trigger recalculateOrderStatus
				if err := h.orderLogic.UpdateOrderItemStatus(sessCtx, updateReq); err != nil {
					return nil, fmt.Errorf("failed to update order item status to returned: %w", err)
				}

				if err := h.ticketLogic.VoidTicketByOrderItem(sessCtx, orderItem.ID); err != nil {
					return nil, fmt.Errorf("failed to void ticket for refunded item: %w", err)
				}
			} else {
				// Successful Payment Flow
				if err := h.orderLogic.HandlePaymentSuccess(sessCtx, bill.Order, h.ticketLogic.GetPublisher()); err != nil {
					return nil, fmt.Errorf("failed to handle successful payment: %w", err)
				}
			}
		} else {
			// On a failed payment, we've already marked the bill as 'Failed' inside ProcessPaymentUpdate.
			// As you correctly pointed out, the order's status itself doesn't need to be recalculated
			// because the paid amount hasn't changed. So, we do nothing further here.
		}

		return nil, nil
	})

	// 4. Check the final result of the transaction.
	if err != nil {
		h.logger.Error("Transaction failed for bill payment update, will retry.", zap.Error(err), zap.Stringer("billID", bid))
		return err // The error is returned, so the message will be requeued.
	}

	h.logger.Info("Successfully processed bill payment update", zap.Stringer("billID", bid))
	return nil // Return nil to acknowledge the message.
}
