package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"partivo_tickets/internal/conf"
	"partivo_tickets/internal/db"
	"partivo_tickets/internal/logic"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// TicketHandler encapsulates the logic for handling ticket-related messages.
type TicketHandler struct {
	ticketLogic *logic.TicketLogic
	orderLogic  logic.OrderLogic
	logger      *zap.Logger
	cfg         *conf.RabbitMQConfig
	tm          db.TransactionManager
}

// NewTicketHandler creates a new TicketHandler.
func NewTicketHandler(ticketLogic *logic.TicketLogic, orderLogic logic.OrderLogic, logger *zap.Logger, cfg *conf.RabbitMQConfig, tm db.TransactionManager) *TicketHandler {
	return &TicketHandler{
		ticketLogic: ticketLogic,
		orderLogic:  orderLogic,
		logger:      logger.Named("TicketHandler"),
		cfg:         cfg,
		tm:          tm,
	}
}

// QueueName returns the name of the queue this handler subscribes to.
func (h *TicketHandler) QueueName() string {
	return h.cfg.GenerateTicketsTopic
}

// Handle is the function for processing messages to generate tickets.
func (h *TicketHandler) Handle(ctx context.Context, d amqp.Delivery) error {
	h.logger.Info("Received message to generate ticket", zap.ByteString("body", d.Body))

	// 1. Parse the message payload.
	var payload struct {
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(d.Body, &payload); err != nil {
		h.logger.Error("Failed to unmarshal message body", zap.Error(err), zap.ByteString("body", d.Body))
		return nil // Poison pill, ACK and remove.
	}

	// 2. Validate the OrderID.
	oid, err := primitive.ObjectIDFromHex(payload.OrderID)
	if err != nil {
		h.logger.Error("Invalid OrderID format in message", zap.Error(err), zap.String("order_id", payload.OrderID))
		return nil // Invalid format, ACK and remove.
	}

	// 3. The entire fulfillment process is wrapped in a single transaction controlled by the handler.
	_, err = h.tm.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		// Step 3a: Generate tickets for the order.
		if err := h.ticketLogic.GenerateTicketsForOrder(sessCtx, oid); err != nil {
			// Check if the error is a permanent one that shouldn't be retried.
			if errors.Is(err, logic.ErrPermanent) {
				// A permanent error occurred. We stop the transaction but will ACK the message
				// by returning a nil error from the handler.
				// Returning (nil, nil) from WithTransaction achieves this.
				return nil, nil
			}
			// For transient errors, wrap and return them to trigger a retry.
			return nil, fmt.Errorf("failed during ticket generation: %w", err)
		}

		// Step 3b: Mark order items as fulfilled and update the order status.
		if err := h.orderLogic.MarkOrderItemsAsFulfilled(sessCtx, oid); err != nil {
			return nil, fmt.Errorf("failed to mark order items as fulfilled: %w", err)
		}

		return nil, nil
	})

	// 4. Check the final result of the transaction.
	if err != nil {
		h.logger.Error("Transaction failed for fulfilling order, will retry.", zap.Error(err), zap.Stringer("orderID", oid))
		// The error is returned, so the message will be requeued for another attempt.
		return err
	}

	h.logger.Info("Successfully fulfilled order in a single transaction", zap.Stringer("orderID", oid))
	return nil // Return nil to acknowledge the message.
}
